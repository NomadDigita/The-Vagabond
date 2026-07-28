# AI Full Parity, Long-Range Scouting, and World-Event Broadcasts Plan

Status: not started - this is the design, written before any of it exists.
Read this whole file before writing code. It covers five features the
project owner asked for together; they're grouped in one document because
several of them share touchpoints (the scouting rework and the discovery-
permanence rule are two halves of one mechanic, and "AI full parity"
touches almost every system this file's other sections also touch), but
each section is buildable and testable independently - see "Suggested
build order" at the end for a safe sequencing.

This supersedes two "deliberately out of scope" notes in
`AI_FACTION_DECISION_LOOP_PLAN.md` (AI-vs-AI raids, and dedicated
scouting/spying) - both are now in scope, per direct instruction. Read
that file first if you haven't; this one assumes you have and doesn't
re-explain the AI decision loop, `roadMover`, `roadcombat`, or the
discovery-gating principle already established there.

---

## 1. AI Full Parity

### 1.1 What "parity" means, precisely

Today, an AI faction is a real `encampments` row and already flows through
the entire raid/combat/road/loot pipeline as an **attacker** (see the AI
decision loop) or as a **raid target** (it always could be, even before
the decision loop existed - a human could always raid a Rogue Drone Nest
or, since Phase 6, an AI faction's base). What's still asymmetric:

| System | Humans | AI factions today | Target |
|---|---|---|---|
| Leaderboard (`ranking.go`) | Ranked | Explicitly filtered out (`WHERE e.is_ai_faction = FALSE`, two places) | Ranked identically |
| Weather route incidents (Phase 5) | Affects a marching raid | Only affects AI-attacker raids incidentally (AI raids are real `raids` rows, so this already works - see 1.3) | No change needed, just confirm with a test |
| Road encounters (Phase 3/4) | Full participant | Full participant as attacker (proven by this session's exit-criteria test); **not yet reachable as the "passive base" side** for `evaluateRoadBaseEncounters`, which explicitly filters `is_ai_faction = FALSE` | Decide per-system, see 1.4 |
| Tax collection (`collectDailyTax`) | Taxed | Unknown - not audited yet | Confirm/fix, see 1.3 |
| Total defeat / total victory economics | Capped loot (40% per raid, see 1.5) | Same cap applies (shared code path) | Explicitly redesigned in 1.5 per the project owner's request |
| Base defense grid, modules, research | Human-managed | AI factions have `workshop_inventory` and (soon) `research_states`? Not audited | Confirm/document what AI factions actually have vs. lack, see 1.3 |

### 1.2 Leaderboard

The fix itself is small (`ranking.go`'s two `WHERE e.is_ai_faction = FALSE`
clauses), but it needs a **display decision**, not just a query change:
should an AI faction's leaderboard row look identical to a human's, or be
marked (e.g. a 🤖 prefix on the name)? Precedent in this codebase strongly
favors marking it - `growAICivilizations`' seeded AI factions all have
flavor names (see `cmd/bot/main.go`'s AI faction seed list) that already
read as non-human, and the exploration/discovery notification code
elsewhere in this codebase always distinguishes "Rogue Drone Nest" from a
real player by name rather than hiding the distinction. **Recommendation:
show AI factions in the same ranked list, unmarked in the score itself
(they earned it exactly the same way), but prefix the name with 🤖 in the
display line only** - competitive parity in the number, honest labeling
in the text. Apply this to `HandleRankingPanel`'s three queries (top
players, top skilled, top guilds - check whether AI factions can belong to
a guild/clan at all; if not, the guild query needs no change).

Also audit `admin.go`, `profile.go`, `world.go` for the same
`is_ai_faction = FALSE` pattern used for *other* real-player-only counts
(new-player lists, active-user counts, etc.) - those should almost
certainly stay excluded (an AI faction was never "new" or "active" in the
sense those metrics mean), so **don't blanket-remove every filter**; the
leaderboard is the one place parity was explicitly asked for.

### 1.3 Auditing what AI factions do and don't have

Before writing new code, run these checks (they're read-only, do this
first):

```sql
-- Do AI factions have a research_states row? (affects march speed,
-- military tech level used in roadcombat.Power, etc.)
SELECT e.name, rs.encampment_id IS NOT NULL AS has_research
FROM encampments e LEFT JOIN research_states rs ON rs.encampment_id = e.id
WHERE e.is_ai_faction = TRUE;

-- Do AI factions get taxed today? (collectDailyTax's query - read that
-- function, don't guess; if it doesn't filter is_ai_faction either way,
-- AI factions are already being taxed exactly like players, which is
-- correct and needs no fix)
```

If AI factions lack `research_states`, every `COALESCE(..., 1)` read
already defaults them to tech level 1 safely (no crash), but that also
means they're permanently at the weakest tier for anything tech-gated -
acceptable for now (matches "foundational tier" framing), but worth a
one-line note in `MMO_WORLD_EVOLUTION_PLAN.md`'s Phase 6 ledger once
confirmed either way, so nobody re-investigates this later as a mystery.

### 1.4 Should AI factions be a valid *passive* road-base-encounter target?

This is a genuine design question, not just an audit item.
`evaluateRoadBaseEncounters` currently excludes `is_ai_faction = TRUE` on
the target side (a human column can't stumble into an AI base and get a
forced Attack/Continue window the way it can with another human's base).

Arguments for enabling it: full parity, "everything should affect them
too."

Arguments against: an AI faction's home garrison
(`workshop_inventory.garrisoned_soldiers`/`garrisoned_mechs`) is a
*player-set* reserve concept (see `loadBaseGarrisonForce`'s doc comment) -
AI factions have never set one (nobody's played their Hero Commander
panel), so it will always read as `0, 0`, meaning every human column that
stumbles into one would get a free, uncontested "win" with zero risk. That
's not parity, it's a trivially exploitable farm.

**Resolution: enable it, but give AI factions a synthetic garrison
reserve instead of reading the player-set column.** Add a
`aiGarrisonReserveFraction` (e.g. 20%) of the AI faction's *current*
`workshop_inventory.soldiers`/`mechs` computed at encounter-resolution
time (not stored, computed on the fly, the same way a human's actual
mobilized-vs-garrisoned split is a live decision) - mirrors the spirit of
`aiFractionOfGarrisonCommitted` from the decision loop (which already
caps at 65% committed, implying 35% stays home; use a similar, separately-
tunable constant here since "how much is left over to defend an ambush"
and "how much a raid commits" are different tuning knobs even if they
start at similar values). This makes an AI faction a real, winnable-but-
not-free target on the road, without needing to fabricate a whole new
manual-garrison UI for factions nobody plays.

### 1.5 Total defeat and total victory economics

This is the biggest, most consequential decision in this section, and the
project owner should sign off on the specific numbers before they ship,
not just the direction - see "Open questions for the project owner" at
the end.

**Current state**: every loot-capture path in the game is capped well
below 100% per event - base-raid loot is capped at `lootPercentage :=
0.40` (`resolveRaidCombats`, `internal/engine/tick/engine.go`), and road-
encounter cargo capture is capped at `roadcombat.CargoCaptureFraction =
0.40`. **No single raid, against anyone, human or AI, can currently
bankrupt the loser.** This is why "they can lose all their resources...
gain all another user['s] resources with heavy raiding" doesn't happen
today - not because of an AI-specific gap, but because the cap applies
uniformly to every raid in the game.

**Recommended design**: don't remove the per-raid cap (a single raid
instantly zeroing a new player's entire economy is a real new-player-
retention risk, not just a balance number) - instead, make **sustained,
repeated raiding** capable of fully depleting someone over multiple hits,
which the 40%-per-raid cap already mathematically allows if raids repeat
(0.6^n of the original resources survive after n full-loot raids - by the
5th consecutive raid, under 8% remains). What's missing isn't a mechanic,
it's **communicating** that this is possible and **removing any hidden
floor** that might exist elsewhere (audit `resolveRaidCombats` end-to-end
for any `if remaining < X { remaining = X }` style floor - if one exists,
it silently contradicts "can lose all their resources" and should be
removed or explicitly justified). Total military defeat (losing every
soldier/mech, not just resources) already has no floor - `CasualtiesFor`/
`Survivors` in `roadcombat.go` and the equivalent base-combat casualty
code can already reduce a force to zero.

For an AI faction specifically: once this session's AI-vs-AI work (below)
and the passive-target parity (1.4) both ship, an AI faction can be
raided repeatedly by both humans and other AI factions with no special
protection - it can genuinely be ground down to nothing, exactly like a
player. `growAICivilizations`' passive growth is the only thing that ever
brings it back, at the same slow trickle rate as before - a "totally
defeated" AI faction should recover the same slow way a "totally raided"
human player recovers by playing normally, not via any special AI-only
respawn/reset logic. **Do not add AI-specific "resurrection" logic** - if
an AI faction's `workshop_inventory` and `resources` hit zero, let
`growAICivilizations` rebuild it from zero exactly like it would for a
brand new faction, no different code path.

---

## 2. AI-vs-AI Conflict

`AI_FACTION_DECISION_LOOP_PLAN.md` explicitly excluded this. Un-excluding
it is almost entirely a matter of **removing** guardrails that were added
specifically to prevent it, not adding new mechanism:

- `countUndiscoveredRealPlayers` and `aiScout` both filter
  `e.is_ai_faction = FALSE` on the *target* side - remove that filter (a
  faction can now scout and discover another AI faction).
- `pickFairAIRaidTarget` filters `e.is_ai_faction = FALSE` on the target
  side too - remove it there as well.
- The fairness band (`aiMaxLevelsBelowSelfForFairTarget`) should still
  apply symmetrically between two AI factions - don't special-case AI-vs-
  AI to skip the fairness check just because both sides are AI.

That's the whole mechanical change - `launchAIRaid` already works for any
valid target regardless of who it is, because (per the AI decision loop's
whole design principle) it just creates a normal `raids` row.

**What does need new thought**: two AI factions can now war with each
other indefinitely with no player involvement at all. Left completely
unchecked, this could produce endless AI-vs-AI combat noise (extra
`world_news` headlines, tick-time cost) that has no player-facing value.
Two mitigations, both worth building together:

1. **`world_news` headline suppression for AI-vs-AI**: still log to
   `ai_faction_decisions` (the audit trail should record everything), but
   skip the public `world_news` "HOSTILE CONTACT" headline when both
   `attacker_id` and `defender_id` resolve to `is_ai_faction = TRUE` -
   nobody reads a newsfeed of two bots fighting each other.
2. **A standing non-aggression baseline, broken only by scarcity**: don't
   let AI factions raid each other with the same 40% probability used for
   AI-vs-player (`aiRaidProbabilityWhenEligible`). Use a separate, lower
   constant (suggest starting around 10-15%) so AI-vs-AI wars are rare
   background texture, not the AI's default behavior - the interesting,
   player-facing case (AI raiding a human) should still dominate.

---

## 3. Long-Range Scouting: search-until-found, not a timed dispatch

This is the "dedicated spy/recon" gap, but built to the project owner's
specific spec, which is meaningfully different from anything in the
codebase today (including the just-rebuilt personal exploration-site
system from PR #33 - **do not touch `exploration.go`/`spawnExplorationSites`
/`resolveExplorationDispatches` for this**, they were just fixed for a
real scarcity bug and serve a different purpose: guaranteed resource
rewards, not player-hunting. Build this as a wholly separate feature with
its own table, its own tick pass, and its own bot commands).

### 3.1 The mechanic, restated precisely from the request

1. A player (or AI faction) dispatches a **Scout Party** with no
   destination and no pre-computed travel time - it "searches the whole
   planet."
2. Each tick, the scout party has a chance to find *something* - a real
   player's base or an AI faction's base it hasn't already permanently
   located (see 3.4's discovery-permanence rule). Until it does, it's just
   "out there," with no ETA to give the player.
3. The instant it finds a target, **that becomes its position** - the
   search phase ends, and a **return leg** begins, with a real travel-time
   calculation based on the distance from that location back to the
   scout's home base (the Ghana-to-Portugal example: the return distance
   is a real function of world-coordinate distance, using the same
   `roadcombat`-style march-time formula every other travel mechanic in
   this game uses - not a flat constant).
4. **On the return leg**, the scout might *also* stumble onto a *second*
   target along the way (mirrors `discoverRouteContacts`'s existing
   "reveal a base while marching past it" mechanic - reuse that pattern,
   don't reinvent it).
5. Periodic "still searching" / "en route home" status notifications are
   sent during the journey, not just at the start and end.
6. When the scout finds a real player, **that player gets a notification
   too** ("your base was spotted by a scouting party") - today's
   `discoverRouteContacts`/`resolveExplorationDiscovery` are silent to the
   discovered party; this new mechanic is explicitly not.
7. On return, the scout party brings back some resources, scaled to how
   long the whole round trip took (a longer, more dangerous-feeling
   mission should feel more rewarding, not just "no worse than not
   scouting at all").

### 3.2 Data model

```sql
CREATE TABLE IF NOT EXISTS scout_missions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
    scouts_committed INT NOT NULL,
    phase VARCHAR(20) NOT NULL DEFAULT 'searching', -- 'searching', 'returning', 'complete'
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    -- Set only once a target is found (phase transitions to 'returning'):
    found_target_encampment_id UUID REFERENCES encampments(id) ON DELETE SET NULL,
    found_at TIMESTAMP WITH TIME ZONE,
    found_x INT,
    found_y INT,
    found_region VARCHAR(50),
    return_eta TIMESTAMP WITH TIME ZONE,
    -- A second, incidental discovery made during the return leg, if any -
    -- mirrors discoverRouteContacts, doesn't extend or shorten the trip.
    bonus_discovery_encampment_id UUID REFERENCES encampments(id) ON DELETE SET NULL,
    last_status_notified_at TIMESTAMP WITH TIME ZONE,
    resources_returned_summary TEXT -- human-readable, for the completion notification/log
);
CREATE INDEX IF NOT EXISTS idx_scout_missions_searching ON scout_missions(encampment_id) WHERE phase = 'searching';
CREATE INDEX IF NOT EXISTS idx_scout_missions_returning ON scout_missions(return_eta) WHERE phase = 'returning';

-- "Permanently found" per the discovery-permanence rule (section 4) -
-- this is a NEW concept distinct from encampment_discoveries (which is a
-- boolean "have I ever seen this" relationship with no coordinate). This
-- table stores the actual coordinate snapshot at time of discovery, which
-- is what "permanent location" means - see section 4 for why this can't
-- just reuse encampment_discoveries as-is.
CREATE TABLE IF NOT EXISTS known_locations (
    observer_encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
    target_encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
    x INT NOT NULL,
    y INT NOT NULL,
    region VARCHAR(50) NOT NULL,
    locked_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (observer_encampment_id, target_encampment_id)
);
```

Reuses the existing `scouts` `workshop_inventory` column as the unit type
committed to a mission (mirrors how raids commit `soldiers_mobilized`/
`mechs_mobilized` - scout missions commit and debit `scouts` the same
way, and get them back - alive or attritted - on mission completion,
matching how a raid's survivors return).

### 3.3 The search phase - why "no ETA" needs a real probabilistic model, not a placeholder

"Search until found, no time calculated at first" cannot literally mean
*infinite* expected time with nothing bounding it - some formula has to
convert "how many scouts committed" and "how much of the world is left
unexplored by this faction" into a per-tick discovery probability, the
same way `worldintel.ExplorationDiscoveryChance(scouts)` already does for
the personal-exploration system. **Reuse that exact function** (or a
close variant) rather than inventing a second probability curve - it
already encodes "more scouts committed = better odds," which is exactly
the right lever here too.

Each tick a mission is in `phase = 'searching'`, roll
`ExplorationDiscoveryChance(scoutsCommitted)`. On a hit, pick a target
using the **same no-omniscience selection query** `aiScout`/
`resolveExplorationDiscovery` already use (undiscovered-by-this-observer,
real player or AI faction, ordered by danger/established_at) - a human's
scout party should be gated by the exact same "nothing hidden" rule an AI
faction's scouting is gated by (this is the connective tissue between
this section and the AI parity section: the *mechanism* now serves both).

Once found: snapshot the target's live coordinates into
`scout_missions.found_x/y/region` AND into `known_locations` (permanent,
see section 4), compute `return_eta` from the distance between the found
location and the scout's home (reuse the `steps * 10.0`-style march
formula every other travel mechanic uses), and flip `phase` to
`'returning'`.

### 3.4 Discovery permanence and the cost of hiding

This is the other half of the mechanic and needs its own careful
treatment, because it changes a standing assumption in the existing
codebase.

**Today**: `encampment_discoveries` is a permanent boolean relationship
("X has discovered Y"), but every system that *acts* on it (raid
targeting, road encounters) always re-reads the target's **live**
coordinate from `encampments.coordinate_id -> coordinates`. This means
the existing `/newjobteleport` (see `jobs.go`) already "hides" nobody
today - an attacker who's discovered you just sees your new coordinates
transparently the moment you teleport, because nothing caches the old
ones.

**What the project owner is asking for**: your location, once found,
should be a **locked intel snapshot** (`known_locations`, above) - an
attacker acts on where they last saw you, not a live GPS feed. The
*only* way to invalidate someone's lock on you is to actually move in a
way expensive and rare enough to matter.

**Do not repurpose the existing `/newjobteleport`** for this. It's a
cheap (1000 Electricity), frequent (24-hour cooldown) utility from an
unrelated Phase 4 "odd jobs" feature; players may already be relying on
its current cost/cadence for reasons unrelated to evading raiders, and
silently making it "very very very costly, once per 3 months" would be a
breaking change to existing content, not an addition. **Add a new,
separate action** instead - suggested name **"Ghost Protocol"** (or
similar - pick something that reads as a big, rare, deliberate act, not a
minor utility):

- Relocates coordinates exactly like `/newjobteleport` does (same random-
  coordinate logic, can share that code).
- Additionally **deletes every `known_locations` row where this
  encampment is the target** - every scout/attacker who'd locked your
  position loses that lock and must rediscover you from scratch (a fresh
  `searching`-phase mission, or a fresh `discoverRouteContacts`/route
  sighting).
- Does **not** touch `encampment_discoveries` (the boolean "have you ever
  heard of this entity" relationship) - that's a different, weaker kind
  of knowledge (matches what a human's Tactical Target Matrix already
  shows: an entity exists, not necessarily where it currently is) and
  should stay permanent; only the *coordinate lock* resets.
- Cost and cooldown: genuinely severe, per the project owner's own words
  ("very very very costly," "once in 3 months"). Suggested starting
  point - all of Scrap/Metal/Crystal/Dollars at a meaningful fraction of
  what a well-established base holds (needs real numbers from actual
  live-economy data once this ships, not a guess baked in permanently;
  treat the exact cost as a tunable constant reviewed after a few weeks
  of real usage, same spirit as this session's `aiMaxLevelsBelowSelfForFairTarget`
  being deliberately conservative-and-adjustable rather than precisely
  right on the first guess) plus a hard 90-day cooldown
  (`last_ghost_protocol_at`, same pattern as `last_teleport_at`).
- AI factions can use Ghost Protocol too (parity) - the AI decision loop
  gets a new possible intent, `'flee'`, chosen only when the faction's own
  garrison/resources have been reduced below some threshold by repeated
  raiding (a reasonable, cheap heuristic: e.g. `workshop_inventory.soldiers
  + mechs < 20% of this faction's peak observed via
  ai_faction_decisions`, or simpler - just resources near zero). This is
  small enough to fold into the existing decision loop rather than
  needing its own tick pass.

### 3.5 Notifications during the mission

- **Periodic "still searching" pings**: not every tick (spammy) - suggest
  every ~30-60 minutes a mission stays in `phase = 'searching'`, gated by
  `scout_missions.last_status_notified_at`, mirroring the AI decision
  loop's cadence-gate pattern (`ai_last_decision_at`). Message like "🔭
  Your scout party continues searching - no contact yet."
- **On finding a target**: notify the scouting player immediately (intel
  found, ETA home now known) - use `notifications.Queue(...,
  "route_status")` since this is routine, not urgent, matching how this
  session tagged peaceful road-pass notifications (see
  `internal/engine/notifications/preferences.go`'s `MutableCategories`).
- **The discovered player gets a notification too** (new behavior,
  explicitly requested) - this should almost certainly be a
  non-mutable category (general, or a new `"scouted"` category kept out
  of `MutableCategories`) since "someone found my base" is closer to a
  discovery/security event than routine chatter, per the existing
  principle that discovery/combat/supply-loss notifications must never be
  silenceable.
- **Periodic "returning, ETA X" pings** during the `'returning'` phase,
  same cadence idea as the searching-phase pings.
- **Bonus mid-return discovery**: if it happens, notify the same as any
  other discovery event (reuse `discoverRouteContacts`'s existing
  notification shape/wording as a template).
- **Completion**: full mission report - what was found (if anything),
  resources brought home, scouts lost (if the mission had any attrition
  risk - decide whether scouting is risk-free or can lose scouts along
  the way; simplest starting design: risk-free, matching how personal
  exploration dispatches work today).

### 3.6 Resources brought home, scaled to journey length

"Should take home some resources too, depending on how long the journey
was" - model this as a small, steady scavenging trickle proportional to
total mission duration (search phase + return phase combined), not tied
to the target found (that would conflate scouting with looting, which
should stay a separate, deliberate raid decision). Suggested formula
shape: `baseRatePerHour * totalMissionHours * scoutsCommitted`, split
across a couple of common resources (scrap/metal, matching what
`explorationTemplates`' common-tier rewards already look like in
`exploration.go`, for value-level consistency with the rest of the game's
economy - don't invent numbers in a vacuum, calibrate against what an
equivalent-duration `/explore` dispatch already pays out).

### 3.7 The tick pass

New file `internal/engine/tick/scoutmissions.go`, new tick registration
(e.g. `{"scout_missions", e.processScoutMissions}`, placed near
`exploration_resolve` in the tick list for topical grouping, not because
of any dependency). One function handling both phases:

- For `'searching'` missions past a decision-style probability roll: find
  a target (see 3.3) or stay searching.
- For `'returning'` missions past `return_eta`: complete the mission -
  pay out resources (3.6), return the scouts to `workshop_inventory`,
  handle any bonus discovery already recorded, send the completion
  notification, set `phase = 'complete'` (or just delete the row - decide
  based on whether a mission history view is wanted; if in doubt, keep
  rows and add a `completed_at` column rather than deleting, cheap
  insurance for a future "mission log" UI).
- Status-ping gating (3.5) runs every tick regardless of phase, checking
  `last_status_notified_at`.

---

## 4. (Merged into section 3.4 above - discovery permanence and Ghost
Protocol are one mechanic, kept together rather than split across two
sections.)

---

## 5. World-Event Broadcasts

### 5.1 The gap

Confirmed by reading the actual code, not assumed:

- **Weather** (`internal/engine/world/weather.go`'s `RunWeatherPass`):
  every event start/clear only writes a `world_news` row - a passive feed
  nobody is pushed to read. No player in the affected continent gets a
  direct notification today.
- **Tax rate changes** (`admin.go`'s `doSetTaxRate`): updates `tax_law`
  with **zero** notification of any kind, not even a `world_news` row.

### 5.2 Design: two broadcast shapes, not one

The project owner's examples split naturally into two different scopes -
build both as explicit, distinct helper functions, don't conflate them:

**Regional** (weather, and anything else tied to a `continent`/`region`):
```go
// internal/engine/notifications/broadcast.go
func QueueToRegion(ctx context.Context, q querier, db regionQuerier, region, message, category string) error {
    rows, err := db.QueryContext(ctx, `
        SELECT DISTINCT u.telegram_id
        FROM users u
        JOIN encampments e ON e.user_id = u.telegram_id
        JOIN coordinates c ON c.id = e.coordinate_id
        WHERE c.region = $1 AND e.is_ai_faction = FALSE`, region)
    // ... Queue() each one, same mute-category logic as any other notification
}
```

**Global** (tax rate, and anything server-wide):
```go
func QueueToAllPlayers(ctx context.Context, q querier, db allPlayersQuerier, message, category string) error {
    rows, err := db.QueryContext(ctx, `SELECT telegram_id FROM users`) // real players only; AI factions have no Telegram session to notify
    // ... Queue() each one
}
```

Both belong in `internal/engine/notifications/` (new file, alongside
this session's `preferences.go`), both go through the existing
`notifications.Queue()` mute-aware path, and both need a **category**
decision: weather affecting your continent and a tax rate change are
significant enough that they probably should **not** be muteable (they're
closer to "discovery" than "routine route status" on the
significance scale this session already established) - default to
`"general"` (non-mutable) unless the project owner wants a new
`"world_events"` mutable category specifically for these. That's a real
product decision, not an engineering one - flag it, don't just pick
silently.

### 5.3 Where to call these

- `RunWeatherPass` (`weather.go`): call `QueueToRegion` at both the event-
  start point and the event-clear point, once the headline text is built
  - reuse the exact headline as the notification text so the `world_news`
  feed and the direct notification never say different things.
- `doSetTaxRate` (`admin.go`): call `QueueToAllPlayers` right after the
  `UPDATE tax_law` succeeds, with a message stating the old and new rate.
- **"And so many others"**: audit every existing `INSERT INTO world_news`
  call site across the codebase (`git grep "INSERT INTO world_news"`) and
  triage each one into "should also be a direct broadcast" vs. "fine as
  ambient flavor text nobody needs pushed to them" (e.g. a single raid's
  "MILITARY DEPLOYMENT" headline is clearly NOT something every player
  needs pushed - that's normal background noise; a server-wide economic
  or environmental shift is). Build the triage list as part of
  implementation, don't guess the full set up front in this doc - the
  `world_news` call sites will have grown further by the time this is
  built, given how many sessions have touched this codebase.

---

## Testing strategy (all sections)

Same discipline as every other plan this session produced: **real
Postgres, not mocks** (see `AI_FACTION_DECISION_LOOP_PLAN.md`'s Testing
strategy section for the exact rationale and setup commands - identical
here, not repeated).

Minimum coverage before considering each section done:

- **AI parity**: a test that an AI faction appears in
  `HandleRankingPanel`'s query results once the filter is removed; a test
  that an AI faction can be a valid `evaluateRoadBaseEncounters` target
  with a non-zero synthetic garrison.
- **AI-vs-AI**: the mirror of this session's
  `TestAIFactionNeverRaidsAnotherAIFaction` - once this ships, that test's
  assertion inverts. Rename/rewrite it to assert AI-vs-AI raids *can*
  happen (probabilistically, over many iterations) rather than deleting
  the coverage.
- **Scouting**: a searching-phase mission finds a target and transitions
  to `returning` with a real ETA computed from real distance (not a
  placeholder); a returning-phase mission completes, credits resources,
  and returns scouts to the garrison; the discovered player receives a
  notification (query `notifications` for their `user_id` after running
  the tick pass); a bonus mid-return discovery is possible (probabilistic
  test, same 40-iteration style as this session's existing probabilistic
  tests).
- **Discovery permanence / Ghost Protocol**: a `known_locations` row
  persists across repeated raid-targeting lookups (proving the lock, not
  a live re-fetch); using Ghost Protocol deletes every `known_locations`
  row targeting that encampment and leaves `encampment_discoveries`
  untouched; the cooldown actually blocks a second attempt within 90
  days.
- **World-event broadcasts**: `QueueToRegion` reaches only users whose
  encampment is in the target region, not the whole player base;
  `QueueToAllPlayers` reaches everyone; both respect
  `notifications.Queue`'s existing mute-category logic if the category
  chosen ends up mutable.

## Suggested build order

1. **AI parity leaderboard fix** (1.2) - smallest, safest, no schema
   changes, immediately visible.
2. **AI-vs-AI** (2) - small mechanical change (remove filters) on top of
   already-working, already-tested code from the AI decision loop.
   Confirm/add the `world_news` suppression and lower raid-probability
   constant alongside it, not as an afterthought.
3. **World-event broadcasts** (5) - self-contained, no dependency on
   sections 3/4, immediately valuable, good place to firm up the
   `QueueToRegion`/`QueueToAllPlayers` helpers that section 3.5 will also
   lean on.
4. **Discovery permanence + Ghost Protocol** (3.4) - build this *before*
   the scouting rework (3.1-3.3, 3.5-3.7), since the scouting system's
   whole "permanent location" promise depends on `known_locations`
   existing first. This is also the piece most likely to need the project
   owner's sign-off on exact costs before shipping (see below) - get that
   conversation started early rather than blocking the rest of the plan
   on it.
5. **Long-range scouting** (3.1-3.3, 3.5-3.7) - the largest, most novel
   piece, deliberately built last once its dependency (discovery
   permanence) and its shared infrastructure (regional/global broadcast
   helpers, if scouting's per-mission pings end up reusing any of that
   plumbing) already exist and are tested.
6. **AI passive-target parity + total-defeat/victory audit** (1.4, 1.5) -
   last, since 1.4 specifically depends on tuning a synthetic-garrison
   constant that's easier to get right once AI-vs-AI (2) has been live
   long enough to observe real AI garrison sizes in practice, and 1.5 is
   explicitly an audit-plus-judgment-call step, not a fixed scope of code.

## Open questions for the project owner (don't guess these, ask)

1. **Ghost Protocol's exact cost/cooldown numbers.** "Very very very
   costly" and "once in 3 months" are directional, not numbers. Proposed
   starting point is in 3.4; needs explicit sign-off, and probably a
   revisit after a few weeks of real usage data (which didn't exist for
   *any* balance number in this game before now - see
   `MMO_WORLD_EVOLUTION_PLAN.md` Phase 7 milestone 4, still blocked on
   exactly this kind of missing data).
2. **World-event broadcast category**: mutable (`"world_events"`, joins
   `route_status` as something a player can silence) or non-mutable
   (`"general"`, always delivered)? Section 5.2 recommends non-mutable but
   this is a real product-feel decision.
3. **Should scouting carry any risk** (losing scouts, not just time), or
   is it risk-free like the current personal-exploration dispatch?
   Section 3.5 defaults to risk-free; flag if that's wrong.
4. **AI faction display on the leaderboard**: 🤖-prefixed name with an
   otherwise-identical score, as recommended in 1.2, or something else
   entirely (a separate leaderboard section instead of an interleaved
   one)?
5. **The total-defeat/total-victory floor audit (1.5)**: confirm there's
   genuinely no hidden resource floor anywhere in `resolveRaidCombats`
   before this is called "just a communication gap" - if a floor *is*
   found, that's a deliberate decision someone made once, and removing it
   is a balance change worth flagging explicitly rather than silently
   deleting.

## A note on process, again

This plan touches `ranking.go`, `engine.go`, `aidecisions.go`, `weather.go`,
`admin.go`, and creates several new files - a wide surface, on a codebase
where every phase so far has collided with parallel work at least once.
Check `git branch -a` and the open PR list before starting, same as every
other plan this session has written. If someone else is mid-build on any
of sections 1-5 when you pick this up, read their branch's intent before
assuming your version of this doc is still the current plan - update this
file itself if the design changes, don't let it go stale the way earlier
docs in this repo briefly did before this session's habit of editing
stale notes instead of leaving them to contradict newer ones.
