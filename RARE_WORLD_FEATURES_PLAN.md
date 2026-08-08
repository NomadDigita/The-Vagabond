# Rare World Features Plan: Relic Convoys, Wasteland Blooms, Legacy
# Epitaphs, the Wandering Merchant, and Constellation Alliances

Status: **in progress**, implemented and committed one feature at a time
per direct project-owner instruction ("work on it one by one... in
phases and milestones... don't minimize"). Each feature below is its own
phase with its own milestones, its own schema, its own tick-engine hook
(where relevant), its own handler(s), its own command/keyboard wiring,
and its own tests. This doc is the coordination point for continuity
across sessions/developers - update the Status line at the top of each
phase as it lands, the same way AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md
and FEEDBACK_CHANGELOG_NLP_PLAN.md track their own build order.

Origin: direct project-owner request for "5 best new features... rare,
sweet, and beautiful" after a market-exchange feature pass, expanded to
"introduce all 5... fully implemented, not minimized... in phases and
milestones" as a follow-up instruction.

---

## Design principles shared by all five

1. **Real tables, real tick-engine phases, real tests against Postgres** -
   same standard as every existing MMO system in this codebase (see
   `internal/db/schema/schema.go`'s idempotent `CREATE TABLE IF NOT
   EXISTS`/`ALTER TABLE ADD COLUMN IF NOT EXISTS` convention). No feature
   here is a UI mockup over a stub.
2. **Rare means rare.** Every spawn/trigger roll in this doc is
   deliberately small (sub-1%-per-tick territory for the biggest
   moments) and gated so at most one instance of a given rare event is
   ever active server-wide at once, unless stated otherwise - a "rare
   and beautiful" moment that happens constantly stops being either.
3. **AI factions, not real players, absorb any "permanent"/destructive
   mechanic.** Legacy Epitaphs (Phase 3) introduces the game's first
   permanent-defeat state; it is deliberately scoped to AI factions only.
   Real players are never wiped, deleted, or locked out by anything in
   this plan - that would be a much larger, riskier design change than
   what was asked for, and isn't introduced here without an explicit
   separate ask.
4. **Notifications follow AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md
   section 5's precedent**: broadcast moments use `QueueToRegion`/
   `QueueToAllPlayers` with the non-mutable `"general"` category (never
   silenced by the routine-ping mute toggle), matching how weather and
   world-boss-tier announcements already behave.
5. **Every new tick phase is its own named entry in `ProcessTick`'s
   `phases` slice** (`internal/engine/tick/engine.go`), so it gets the
   same panic-isolation, per-phase transaction, and admin-visible
   `LastTickStatus` timing every other phase already gets for free.
6. **Follow the file's own local conventions exactly**: `doX` testable
   cores separate from telebot handlers, `sendPanelWithNavHTML`/
   `sendPanelWithNavRich` for panels, `htmlBold`/`htmlEscape`/`htmlCode`
   for all interpolated text, real-Postgres tests only (no mocks),
   `go.mod`/`go.sum` restored before every commit.

---

## Phase 1: Relic Convoys

**Status: ✅ Complete** (see `internal/db/schema/schema.go`'s
`relic_convoys` table, `internal/engine/tick/relicconvoys.go`,
`internal/bot/handlers/relicconvoy.go`, `/relic` command).

A vanishingly rare, server-wide race: a pre-war relic convoy surfaces at
a random point in the wasteland, broadcast to every player at once, and
the first person to tap **Claim** wins a permanent title and a cash
windfall. No travel time, no coordinate proximity requirement, no
military commitment - the entire mechanic is the race itself, kept
deliberately simple so the "rare and exciting" read isn't buried under
mechanical friction.

### 1.1 Schema

`relic_convoys`: `id`, `relic_name` (flavor-generated, see 1.2),
`coordinate_id` (flavor location, references `coordinates`),
`reward_dollars`, `spawned_at`, `expires_at`, `claimed_by` (nullable FK
to `encampments`, `ON DELETE SET NULL` so a later account/encampment
deletion doesn't retroactively rewrite history), `claimed_at`.

### 1.2 Spawn (tick phase `relic_convoy_spawn`)

- Only rolls if there is currently no active (`claimed_by IS NULL AND
  expires_at > now()`) relic convoy - **at most one live at a time,
  server-wide**.
- Roll chance: `0.15%` per tick (`relicConvoySpawnChance`). At the
  existing ~60s tick cadence this averages roughly one spawn every
  ~11 hours of continuous uptime - a real "stop what you're doing"
  moment, not a background hum.
- Location: a uniformly random existing `coordinates` row (reuses
  already-seeded world geography rather than minting a new one).
- `relic_name`: picked from a curated flavor list (`relicNames` in
  `relicconvoys.go`) - "The Last Aurora Convoy", "The Chrome Reliquary",
  etc. - so the broadcast headline reads as a named, memorable event
  rather than a generic "convoy #4821".
- `reward_dollars`: a meaningful windfall, scaled off the game's own
  Crystal-conversion economy (see `crystal_exchange.go`'s `dollars`
  rate of 2000/Crystal) rather than an arbitrary number - `12,000-
  20,000` (a `rand`-rolled range), i.e. roughly 6-10 Crystal's worth of
  cash for doing nothing but tapping first.
- Expiry window: `3 hours` (`relicConvoyWindow`) - long enough that
  players in any timezone/session pattern have a real shot, short
  enough that it stays an event rather than a standing fixture.
- Broadcast: `QueueToAllPlayers`, category `"general"`, directing
  players to `/relic`.

### 1.3 Claim (`HandleClaimRelicCallback`, button `claim_relic`)

- Race-safety: `SELECT ... FOR UPDATE` on the `relic_convoys` row,
  re-checked `claimed_by IS NULL AND expires_at > now()` inside the
  same transaction as every other claim-style mechanic in this codebase
  (mirrors `doBuyListingQty`'s listing lock, `HandleAttackBossCallback`'s
  boss lock).
- On success: `claimed_by`/`claimed_at` set, `reward_dollars` credited
  (storage-cap clamped, same as every other resource-gain path), and a
  **permanent title** is appended to the winning encampment's new
  `relic_title` column (`encampments.relic_title`, nullable `TEXT`) -
  e.g. "🏺 Keeper of the Last Aurora Convoy". This is intentionally
  never overwritten by a later relic claim (first relic keeps the
  title) so the very first server-wide relic feels like it matters more
  than the tenth.
- A second broadcast announces the winner by name server-wide - the
  payoff moment everyone who saw the first broadcast gets to see
  resolved.

### 1.4 Expiry (tick phase `relic_convoy_expire`)

Unclaimed convoys past `expires_at` are cleared with a "the convoy has
moved on" broadcast, matching the weather engine's own "conditions have
cleared" pattern (`world.go`'s `RunWeatherPass`) rather than silently
vanishing.

### 1.5 Panel (`/relic`, `HandleRelicPanel`)

Shows the current live relic (if any) with a Claim button, or a "no
relic detected right now" state, plus a **Hall of Relics** - the last 10
claimed relics server-wide, rendered as a Rich Message `<table>`
(reusing the pattern `exchange.go`/`ranking.go` established) of
relic name / winner / date, so a claimed relic has permanent, visible
legacy rather than disappearing into the database.

### 1.6 Tests

`relicconvoys_test.go` (tick package) and `relicconvoy_test.go` (handlers
package): spawn only when none active, claim race-safety (two concurrent
claim attempts, exactly one wins), reward storage-cap clamping, title
permanence (a second relic claim by the same or a different player
doesn't overwrite an existing title), expiry clears unclaimed rows.

---

## Phase 2: Seasonal Wasteland Blooms

**Status: ✅ Complete** (see `internal/engine/world/weather.go`'s
`bloom` event and `internal/engine/resource/resource.go`'s bloom
bonus).

A rare, *positive* addition to the existing per-continent weather system
(`internal/engine/world/weather.go`) - every other event in `eventPool`
today is a debuff. Bloom is the wasteland's one beautiful outcome:
bioluminescent flora blooms across the continent, and outposts there get
a real, felt resource bonus for the duration.

### 2.1 Why extend weather rather than build a parallel system

Blooms are continent-scoped, timed, and mutually exclusive with a
continent's other active event - exactly `RunWeatherPass`'s existing
persistence/roll/expiry model. Building a second parallel "bloom engine"
would duplicate all of that machinery for no benefit; extending
`eventPool` (with a weighted rarity to keep bloom rarer than the other
six) is the correct, minimal-surface-area way in.

### 2.2 Weighted rarity

`eventPool`'s existing selection was a flat `eventPool[rand.Intn(len(eventPool))]`
- adding bloom as an eighth flat entry would make it as common as Acid
Rain, which undersells "rare." `weatherPool` is now a slice of
`(eventType, weight)` pairs; `pickWeightedEvent` does a weighted draw.
Bloom's weight is `1` against `4` for every other event - roughly 4x
rarer than any single existing event, while every existing event's
*relative* odds against each other are unchanged (still uniform among
themselves).

### 2.3 Effect

While a continent's active event is `bloom`, that continent's resource
tick (`internal/engine/resource/resource.go`) applies a flat `+15%`
multiplier to every resource line that pass actually generates
passively - Scrap, Rations, Electricity, Ether, Metal, and Crystal -
for every real-player and AI-faction encampment in that region, the
mirror image of Supply Crisis's existing negative multiplier elsewhere
in the same pass. (Hydrogen and Dollars aren't part of this passive
generation pass at all - they come from jobs/mining/market instead - so
they're correctly untouched by Bloom, same as by any other weather
event.)

### 2.4 Presentation

- Headline (`eventHeadline`): "🌸 WASTELAND BLOOM: Bioluminescent flora
  has erupted across {continent}'s ruins overnight. Outpost harvesters
  are reporting significantly elevated yields across every resource
  line."
- `eventLabel`: "the Wasteland Bloom" (for the "conditions have cleared"
  message).
- `weatherLine` (`navhelper.go`, used by camp/combat panels' weather
  line): "🌸 Wasteland Bloom - all resource generation +15%."

### 2.6 Tests

`weather_test.go`'s `TestPickWeightedEvent_BloomIsRarerThanEveryOtherEvent`:
over a 20,000-sample statistical draw, bloom lands well under an
unweighted 1-in-8 share and strictly less often than each of its seven
siblings (generous tolerance - a hard exact-ratio assertion would be
flaky, see the test's own comment). `resource_test.go`'s
`TestRunResourcePass_BloomBoostsPassiveGenerationBy15Percent`: an
identical level-1 camp in a blooming region generates exactly 1.15x
the Scrap/Metal/Crystal of a twin camp in a nominal-weather region in
the same pass.

---

## Phase 3: Legacy Epitaphs

**Status: 🔲 Not started.**

The game's first permanent-defeat state - deliberately scoped to **AI
factions only** (see design principle 3 above). When an AI faction is
ground down to nothing (near-zero resources AND an empty garrison) it
is marked defeated exactly once, drops out of every AI spawn/growth/
targeting pool, and leaves a permanent, discoverable ruin with a
generated epitaph crediting whichever real player (or AI faction) most
recently raided it - turning "the AI faction I've been farming finally
died" from an invisible database change into real, rememberable
worldbuilding.

### 3.1 Why this is safe to add now

This deliberately does **not** touch `resolveRaidCombats` (the
existing, large, well-tested combat resolution function) at all - no
risk of destabilizing live combat math. Instead it's a brand new,
independent, read-then-conditionally-write tick phase
(`checkForFactionDefeat`) that runs *after* combat resolution each tick
and asks a simple question per AI faction: is this faction now at
(near-)zero resources and (near-)zero garrison? If yes, and it wasn't
already marked defeated, mark it now. This is the same "separate,
additive, defensive" integration style `AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md`
used for the total-defeat audit (which found the resource floor already
existed and only added a warning, rather than touching the deduction
code itself).

### 3.2 Schema

- `encampments.is_defeated BOOLEAN NOT NULL DEFAULT FALSE`
- `encampments.epitaph TEXT`
- `encampments.defeated_at TIMESTAMP WITH TIME ZONE`

### 3.3 Defeat detection (tick phase `faction_defeat_check`, runs after
`raid_combat`)

For every `is_ai_faction = TRUE AND is_defeated = FALSE` faction:
`total resources (scrap+metal+crystal+dollars) <= factionDefeatResourceThreshold
(50)` **AND** `soldiers+mechs (from workshop_inventory) <= 0`. Both
conditions, not just resources, so a faction that's merely poor but
still garrisoned (or vice versa) isn't prematurely retired - "total
defeat" means genuinely nothing left, matching this plan's own naming.

On trigger:
1. Look up the most recent completed raid (`raids` table, `state =
   'completed'` or equivalent, `defender_id = this faction`, most recent
   `resolve_time`) to credit as the "final blow" - falls back to
   "the wasteland itself" flavor if no raid history exists (e.g. a
   faction that starved from a weather/tax pass instead of combat).
2. Generate an epitaph from a template pool (`epitaphTemplates`) that
   incorporates the faction's own name and, when known, the finishing
   attacker's name/clan - e.g. "*Here fell the {faction}, broken at
   last by {attacker}'s forces. Its banners are dust; its ruins remain.*"
3. Set `is_defeated = TRUE`, `defeated_at = now()`, `epitaph = ...`.
4. Broadcast to the faction's home region (`QueueToRegion`, category
   `"general"`): "🪦 A FACTION HAS FALLEN: {faction} is no more."
5. If a finishing attacker is identified and is a real player, grant a
   small one-time **Conqueror bonus** (cash, scaled like the relic
   windfall) as a "you get to remember doing this" payoff, notified
   directly to that player.

### 3.4 Pool exclusion

`aidecisions.go`, `aijobs.go`, `engine.go`'s `growAICivilizations`, and
`aispawning.go`'s respawn-replacement logic all gain `AND
is_defeated = FALSE` alongside their existing `is_ai_faction = TRUE`
filter - a defeated faction stops scouting, raiding, growing, and
running jobs, but its row (and ruin) persists permanently rather than
being deleted, so 3.5's discovery flavor and the Hall of the Fallen
panel keep working indefinitely.

### 3.5 Discovery flavor

`resolveExplorationDiscovery`/scouting result text (already the place
that names a newly-discovered target) appends the epitaph, in italics,
when the discovered target is a defeated faction - "🪦 *You find only
ruins here.* {epitaph}" - so stumbling onto a fallen faction's old
territory is its own small narrative moment instead of an ordinary
discovery.

### 3.6 Panel (`/legacy`, `HandleLegacyPanel`)

**Hall of the Fallen** - every defeated faction, most recent first,
rendered as a Rich Message `<table>` (name / defeated date / epitaph
excerpt), matching the Phase 1 Hall of Relics and `ranking.go`'s table
pattern.

### 3.7 Tests

`factiondefeat_test.go`: a faction at zero resources/garrison gets
marked defeated exactly once (idempotent on a second tick pass); a
faction that's merely poor OR merely ungarrisoned (not both) is left
alone; the finishing-attacker lookup correctly falls back to flavor text
with no raid history; pool-exclusion queries never return a defeated
faction.

---

## Phase 4: The Wandering Merchant

**Status: 🔲 Not started.**

A rare, time-limited NPC vendor - not tied to any player's own listing,
not competing with the player market exchange (Milestone from the
previous session) - offering a short rotating batch of steeply
discounted resource lots. Gone after being bought out or after its
window closes, encouraging a "drop what you're doing" reaction the same
way Phase 1's relic convoy does, but as a recurring lighter-weight
event rather than a single server-wide race.

### 4.1 Why a separate table instead of a synthetic `market_exchange`
seller

`market_exchange.seller_id` is a `NOT NULL REFERENCES encampments(id)`
- reusing it would require minting a permanent synthetic "Merchant"
encampment row, which would then need explicit exclusion from the
leaderboard, AI-vs-AI conflict pool, AI buy/sell tick, and every other
`is_ai_faction`-style query across the codebase (a wide, error-prone
blast radius for a self-contained feature). A dedicated `merchant_offers`
table with **no seller counterpart at all** avoids all of that - buying
an offer only ever touches the buyer's own resources, exactly like
`crystal_exchange.go`'s NPC-style conversion (no seller row, no
notification to a seller, just a clean debit/credit against the buyer).

### 4.2 Schema

`merchant_offers`: `id`, `item_type`, `quantity`, `price_dollars`,
`spawned_at`, `expires_at`, `purchased_by` (nullable FK to
`encampments`, `ON DELETE SET NULL`), `purchased_at`.

### 4.3 Spawn (tick phase `wandering_merchant_spawn`)

- Only rolls if there are currently no active (`purchased_by IS NULL
  AND expires_at > now()`) offers - a fresh visit is a clean batch, not
  an ever-growing backlog.
- Roll chance: `0.4%` per tick (`merchantSpawnChance`) - rarer than a
  normal weather roll, commoner than a relic convoy, averaging roughly
  one visit every ~4 hours of continuous uptime.
- Generates `2-4` offers (`rand`-rolled count), each a random tradeable
  resource (Metal/Crystal/Scrap) at a random quantity, priced at
  `merchantDiscountFactor (0.6)` × the going NLP-buy market rate concept
  from `doBuyMarketItem`'s own per-unit pricing logic - i.e. roughly
  40% cheaper than a typical player listing, a genuine bargain rather
  than a reskinned market post.
- Window: `90 minutes` (`merchantVisitWindow`) - shorter than a relic
  convoy since it's a recurring, lighter event rather than a
  once-in-many-hours moment.
- Broadcast: `QueueToAllPlayers`, category `"general"`, directing
  players to `/merchant`.

### 4.4 Purchase (`doBuyMerchantOffer`, callback `merchant_buy`)

Transactional, `FOR UPDATE`-locked exactly like `doBuyListingQty`:
checks `purchased_by IS NULL AND expires_at > now()`, checks/deducts
buyer's dollars, credits the resource (storage-cap clamped), marks
`purchased_by`/`purchased_at`. No counterparty payout (there's no
seller) and no seller notification - the entire transaction is the
buyer's own resources.

### 4.5 Expiry (tick phase `wandering_merchant_expire`)

Unpurchased offers past `expires_at` are cleared with a "the Merchant
has moved on" broadcast, same pattern as Phase 1.4/2's weather-clear
precedent.

### 4.6 Panel (`/merchant`, `HandleMerchantPanel`)

Shows current live offers (if any) as a Rich Message `<table>` (item /
quantity / price / implied discount vs. typical market rate) with a Buy
button per offer, or a "the Merchant isn't in range right now" state.

### 4.7 Tests

`wanderingmerchant_test.go`: spawn only when no active offers; purchase
race-safety (two concurrent buyers, exactly one wins); storage-cap
clamping on the resource credit; expiry clears unpurchased offers.

---

## Phase 5: Constellation Alliances

**Status: 🔲 Not started.**

A rare, temporary, cross-clan structure sitting *above* the existing
pairwise `clan_diplomacy` pacts (Phase 7's alliance/NAP system) - 2-3
clans align into a named Constellation with a shared banner, a joint
periodic cash dividend for every member clan, and a hard expiry so it
stays a special, opt-in moment rather than a silent permanent power
bloc.

### 5.1 Why a new structure instead of extending `clan_diplomacy`

`clan_diplomacy` is explicitly pairwise (`clan_a_id`/`clan_b_id`) and
already has a well-defined, tested meaning (blocks raids between
exactly two clans). A Constellation is a different shape entirely -
N-ary (2-3 members), with unanimous accept required from every invited
clan before it activates, and it doesn't touch raid-blocking at all (a
Constellation is about shared prosperity, not mutual defense - clans
wanting raid protection between themselves still use `/ally` as today).
Reusing `clan_diplomacy`'s pairwise shape for an N-ary concept would
mean synthesizing a full mesh of pairwise rows and inferring group
membership from that mesh implicitly - a real new table pair
(`constellations` header + `constellation_members` join, mirroring
`clans/user_clans`'s own header+membership shape) is more explicit and
directly queryable.

### 5.2 Schema

`constellations`: `id`, `name`, `banner_emoji`, `founder_clan_id`
(references `clans`), `status` (`forming`/`active`/`dissolved`),
`created_at`, `activated_at`, `expires_at`.

`constellation_members`: `constellation_id`, `clan_id`, `invited_by`
(user), `status` (`pending`/`accepted`), `responded_at`. Primary key
`(constellation_id, clan_id)`.

### 5.3 Formation flow (mirrors `diplomacy.go`'s propose/accept UX
exactly)

1. A Clan Leader runs `/constellation_form [name] [clan_a] [clan_b]`
   (1 or 2 other clan names, for a 2-3 clan Constellation total). Creates
   the `constellations` row (`status = 'forming'`), a `constellation_members`
   row for the founder's own clan (`status = 'accepted'`, self-invited),
   and one `pending` row per named clan.
2. Each invited clan's Leader sees a pending invitation on `/constellation`
   with Accept/Reject buttons (`keyboards.Styled`, exactly like
   `HandleDiplomacyPanel`'s pact prompts).
3. Once **every** invited member has `accepted`, the Constellation
   becomes `status = 'active'`, `activated_at = now()`,
   `expires_at = now() + 7 days` (`constellationDuration`), and a
   broadcast announces the new Constellation by name and banner to
   every member clan's players.
4. Any single invited clan **rejecting** dissolves the whole forming
   Constellation (`status = 'dissolved'`) rather than leaving it
   permanently stuck at 2-of-3 - matching this plan's "special, not a
   silent permanent state" principle.

### 5.4 Joint dividend (tick phase `constellation_dividend`, folded into
the existing `daily_tax` phase's cadence rather than a new standalone
timer)

While `status = 'active'`, once per real-world day (tracked via a
`last_dividend_at` column rather than assuming the tick cadence lines
up with a calendar day), every member clan's *encampments* each receive
a flat `500` dollar "Alignment Dividend" (storage-cap clamped, notified
individually) - deliberately a flat administrative bonus rather than a
percentage-of-resource-generation change, so this phase never has to
touch the resource-tick formulas Phase 2's Bloom bonus already extended
(keeps the two features' code paths fully independent, avoiding any
risk of the two bonuses compounding in an untested way).

### 5.5 Expiry (tick phase `constellation_expire`)

Active Constellations past `expires_at` become `status = 'dissolved'`
with a "the Constellation has ended" broadcast to every member clan -
same pattern as every other expiry phase in this plan.

### 5.6 Visibility

- `/constellation` panel: current Constellation (if any, at any status),
  member clans, banner, expiry countdown, and the propose/accept flow.
- `ranking.go`'s Top Guilds table (already converted to a Rich Message
  `<table>` in the previous market-exchange session) gets a small
  banner-emoji suffix next to any clan currently in an active
  Constellation - a visible, "arrived" flourish on the leaderboard
  itself rather than a feature only visible from its own panel.

### 5.7 Tests

`constellation_test.go`: formation requires the founder to be a Leader;
activation requires unanimous accept; a single reject dissolves the
whole forming Constellation; dividend pays every member clan's
encampments once per day and no more; expiry dissolves active
Constellations past their window; `ranking.go`'s banner suffix only
shows for `active` Constellations.

---

## Build order and dependencies

Phases are independent of each other (no phase's schema or tick phase
depends on another phase's tables), so they're built and committed
strictly in the order above only because that's the order they were
listed to the project owner - not because of any technical dependency.
A future developer picking up an unfinished phase from this doc can do
so without needing any other phase to be complete first.
