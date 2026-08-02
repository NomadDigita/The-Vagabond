# AI Density & Multi-Scouting Expansion Plan

Status: not started - written 2026-08-01 directly from a real player's
in-game report (Wasteland Radio screenshot: "20 monthly users", nominal
climate on every check, no AI raids for 1-3 days) plus a full reading of
the current code (see `BUGS_AND_INCONSISTENCIES.md`'s "Investigation
session (2026-08-01, later)" entry for the confirmed root causes this
plan is responding to - read that first, it explains WHY each item below
is shaped the way it is). This doc is the design, not the implementation -
whoever builds this should re-verify every referenced file/line against
current `main` first, since other sessions touch this codebase
concurrently (see README's "Three roadmaps, three logs" and the list of
open `agent/*` branches in `git branch -a`).

## Why this is a separate doc from AI_FACTION_DECISION_LOOP_PLAN.md and AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md

Both of those are marked complete and already shipped the *mechanism*
(AI factions can decide to scout/raid/flee; long-range scouting and
region broadcasts exist). This plan is about *density and cadence* -
making those existing mechanisms fire far more often, and adding the one
piece neither doc covers at all: AI factions that are created after boot
and keep being created. Don't re-implement anything those two docs
already cover; this only changes constants, gates, and adds new spawn
logic.

## Item 1: Continuous AI faction spawning (new, not covered elsewhere)

**Status: built and merged to `main` (2026-08-01).** The project owner's
answer, quoted for precision:

> 3/day. Every minutes and secs means, they should always actively
> upgrades their units, they can also list items in market, like, they
> can literally do every single thing in the game - just like real
> human, they must be very active and always building up and competing,
> this will make the game very active and very interactive too.

This resolved the "3/day vs every minutes and secs" contradiction this
section originally flagged: 3/day is the literal spawn rate; "every
minutes and secs" wasn't a second, faster spawn rate at all - it was
describing how *active* each already-existing faction should be
(constant building, market participation, etc.), which is a different
axis entirely from how often *new* factions appear. Split into two
pieces as a result - spawning (this item, done) and general activity
level (moved to the new Item 4 below, mostly not done, see there for
why).

What shipped, in `internal/engine/tick/aispawning.go`:

- `spawnNewAIFactions(ctx, tx)`, registered in the tick phase list as
  `"ai_civilization_spawn"`, running immediately before
  `"ai_civilization_growth"` so a faction spawned this tick is visible to
  growth/decisions within the same transaction (same read-your-writes
  reasoning `decideOneAIFaction`'s overdue-guarantee check relies on).
- Cadence gate: `world_state` (the existing weather singleton row, id=1)
  gained two additive columns, `last_ai_spawn_at` and
  `ai_factions_spawned_count`, read/written under the same `FOR UPDATE`
  lock used for the check - a fixed `24h/3` interval produces exactly
  3/day without any calendar-day-boundary logic.
- Procedural name generator: a 20x20 adjective/noun combiner
  (`aiFactionSpawnPrefixes`/`aiFactionSpawnSuffixes`) in the same
  two-word shape as the 8 hand-authored names - `ai_faction_key`
  (derived from `ai_factions_spawned_count`, not the display name) is
  what guarantees real uniqueness, so name collisions across spawns are
  harmless flavor overlap.
- Spawn coordinates: exact same per-continent quadrant math
  `seedAICivilizations` uses, cycling continents round-robin by spawn
  count so growth stays balanced.
- Starting level: 1 - intentionally weaker than either of the original
  8's starting tiers (3 or 6), so a growing population doesn't front-load
  new threats; `growAICivilizations`'s existing passive leveling ramps it
  up over time with zero spawn-specific logic needed.
- No cap on total AI faction count was specified in the answer above (the
  framing - "always building up and competing," "very active" - reads as
  wanting continuous growth, not a ceiling) - **this is an assumption,
  not confirmed**, so it's flagged here rather than silently baked in.
  If unbounded growth turns out to be wrong, the fix is a simple ceiling
  check in `spawnNewAIFactions` before it acts, not a redesign.
- New tests: `TestSpawnNewAIFactionsCreatesAFullyFunctionalEncampment`,
  `TestSpawnNewAIFactionsRespectsCadence`. Also fixed a real test-isolation
  bug this surfaced: `testDB` (the shared tick-package test helper) never
  reset `world_state`/`world_news` between tests, since neither has an FK
  the existing `TRUNCATE ... CASCADE` would catch - every test in the
  package was silently sharing one `world_state` row's spawn-cadence
  state depending on run order. Fixed in the same commit as this item
  (see `internal/engine/tick/roadencounter_test.go`'s `testDB`), not
  spun off separately, since it would have made this item's own tests
  order-dependently flaky otherwise.


## Item 2: AI raid/scout cadence and gating - tuning, not new mechanism

**Status: built and merged to `main` (2026-08-01), partially - see below
for what was intentionally left out this round.** The project owner
answered the two open questions this section originally posed directly
(quoted, since precision matters for a balance decision):

> 1: yes, let it actually hit twice daily (AI vs AI, but same level of
> very near like never 9 can attack level 7 and vice versa and same)
> also in the human vs AI (very near level or higher). But overall,
> lower level can attack higher level but higher level can't attack
> lower level.
>
> 2: make it to sometimes widen the fairness band generally and
> sometimes a dedicated hasn't-been-raided-in-N-hour becomes eligible.

What shipped in `internal/engine/tick/aidecisions.go`, matching that
direction:

- **Fairness direction inverted.** The old
  `aiMaxLevelsBelowSelfForFairTarget` ("may raid up to 2 levels below
  itself") is gone. `pickFairAIRaidTarget` now requires
  `target.level >= faction.level` - attack up or equal, never down -
  applied identically to human and AI-faction targets (answers the
  "also in the human vs AI" line: there's only one fairness check in
  this codebase and it never distinguished target type in the first
  place, so no separate change was needed there).
- **"Sometimes widen the fairness band generally"** is
  `aiFairnessNormalBandAbove` (3 levels above, the common case) vs.
  `aiFairnessWideBandAbove` (8 levels above, rolled `aiFairnessWideBandChance`
  = 25% of the time) - literally "sometimes wider."
- **"A dedicated hasn't-been-raided-in-N-hour becomes eligible"** is the
  new `pickOverdueRaidTarget` + `aiOverdueRaidThreshold` (10 hours) +
  `aiOverdueMinTargetLevel` (4) - checked first in `decideOneAIFaction`,
  before the probability roll, and bypasses the roll entirely when it
  finds a match. Restricted to real players (not AI-vs-AI, which is
  handled by the probability bump below instead).
- **AI-vs-AI "twice daily"**: `aiVsAIRaidProbabilityWhenEligible` raised
  from 0.12 to 0.35. This is a probability, not a literal frequency
  guarantee (unlike the overdue mechanism above, which is deliberately
  human-only) - it's a first-pass number reasoned from the existing
  20-minute cadence, not measured against real traffic. Flagged
  explicitly in the code comment as needing correction from real play.
- Added `TestPickFairAIRaidTargetOnlyAllowsAttackingAtOrAboveOwnLevel`
  (replacing the old below-self-band test, now checking the *opposite*
  direction is enforced), `TestPickOverdueRaidTargetGuaranteesEligibilityRegardlessOfProbability`,
  and `TestDecideOneAIFactionPrioritizesOverdueGuaranteeOverProbabilityRoll`.
  Updated the AI-vs-AI probability test's stale "12%" wording. Verified
  with `go build ./... && go vet ./... && go test ./...` against real
  Postgres (same temporary-`telebot.v3`-replace-then-revert process as
  Item 3).

**Deliberately not changed this round: `aiDecisionCadence` (still 20
minutes).** The project owner's answers addressed fairness direction and
raid probability directly, but didn't weigh in on cadence, and the
original build-order note above is still correct that cadence,
probability, and fairness compound together - changing cadence too on
top of an already-inverted fairness rule and a bumped AI-vs-AI
probability, all in the same pass, would make it much harder to tell
which change caused which effect if the result needs correcting. If the
"constant scouting" feel still isn't there after this round is observed
in practice, shortening `aiDecisionCadence` is the next lever, but that's
a follow-up decision, not bundled into this one.

## Item 3: Up to 3 concurrent scout missions per encampment, independent destinations/ETAs

**Status: built and merged to `main` (2026-08-01).** The mechanical core
described below shipped essentially as scoped: the tick-side processing
in `internal/engine/tick/scoutmissions.go` was already per-row (it never
assumed "the" encampment's mission - it queries every row in a phase
regardless of which encampment owns it), so no tick-engine changes were
needed at all, only the dispatch-side gate and status rendering in
`internal/bot/handlers/scoutmissions.go`:

- `doDispatchScoutMission`'s gate changed from `EXISTS(...)` (block if
  any mission is active) to `COUNT(*) >= maxConcurrentScoutMissions`
  (block only once 3 are active) - one query shape change, same
  transaction-safety characteristics as before.
- `renderScoutStatus` changed from `ORDER BY started_at DESC LIMIT 1`
  (silently showing only the newest mission and hiding the others) to
  listing every active mission with a `Party N/3` label, phase, and (for
  `returning`) its own ETA - each mission genuinely does track its own
  destination (whatever it found) and its own return time independently,
  since they were always separate rows.
- `HandleScoutPanel` now shows existing mission status *and* still offers
  dispatch buttons as long as at least one slot is free, only fully
  locking out dispatch once all 3 slots are committed - previously any
  single active mission hid the dispatch UI entirely.
- No migration needed - `scout_missions` was already one-row-per-mission
  (see schema.go), never a per-encampment singleton table; the cap was
  purely an application-level gate.
- Added `TestDoDispatchScoutMission_AllowsUpToCapThenRejects`,
  `TestRenderScoutStatus_ListsMultipleConcurrentMissionsIndependently`,
  and `TestScoutMissionSchema_CountBasedCapRecognizesMultipleActiveMissions`;
  updated the two tests that encoded the old one-mission assumption.
  Verified with `go build ./... && go vet ./... && go test ./...`
  against real Postgres (temporary `telebot.v3` replace directive used
  locally to resolve the module against the sandbox's network allowlist,
  then reverted before commit - same process as documented in
  `BUGS_AND_INCONSISTENCIES.md`).

Original design notes below, kept for context on what was intentionally
scoped out (see the last two bullets, still open):

Today `scout_missions` is gated to one active row per `encampment_id`
(`doDispatchScoutMission`'s `EXISTS(... WHERE encampment_id = $1 AND
phase IN ('searching','returning'))` check in
`internal/bot/handlers/scoutmissions.go`). The ask is 3 independent
concurrent missions, each with its own destination and own return time,
plus (already possible today, see finding #3 in the BUGS doc) up to
several Scout Walkers riding together on any one of those three.

- Change the gate from "any row exists" to "COUNT(*) < 3" for that same
  WHERE clause - this is the one-line mechanical core of the feature.
- Everywhere a mission is looked up by `encampment_id` alone today
  (status displays, the tick pass that resolves them, notifications)
  needs to become "for each of this encampment's active missions" instead
  of "the encampment's mission" (singular) - audit every
  `scout_missions` reference in `internal/bot/handlers/scoutmissions.go`
  and the resolving tick phase (`processScoutMissions` in
  `internal/engine/tick/engine.go`) for a singular assumption before
  changing the gate, or a 2nd/3rd mission will silently resolve wrong
  (e.g. overwrite mission #1's status message instead of sending its
  own).
- `AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md` section 3 already extended
  `scout_missions` for long-range, search-until-found scouting with its
  own data model additions (section 3.2) - re-read that section's schema
  before touching this table again, since this plan's change needs to
  layer on top of that shape, not conflict with it.
- **Still open, not built in this pass**: `/scoutstatus`'s slash-command
  text output already benefits for free (it shares `renderScoutStatus`
  with the panel), but if there's a separate persistent-keyboard entry
  point elsewhere that assumed one mission, it wasn't found in this
  pass - worth a second grep sweep before considering this fully done.
- **Still open, not built in this pass**: no migration was needed for
  basic multiplicity, but there's still only one shared
  `last_status_notified_at`-style ping cadence per mission row, not
  per-encampment coordination - if a player has 3 missions all pinging
  independently every ~30 minutes, that could mean up to 3 status
  notifications in quick succession rather than one combined message.
  Worth revisiting if that turns out to feel spammy in practice.

## Item 4: AI factions actively using the rest of the game (market listing/buying, clans, clan wars, federations, arena queueing done; the rest remains a much larger, separate effort)

**Status: partially built - market listing (2026-08-01), then market
buying + clan creation/application added the same day after the project
owner's direct follow-up** ("you only give AI market listing ability
when it's supposed to have all ability to even buy and sell, create clan
and join one and many other"). Everything else in the inventory below is
still intentionally not attempted, for the same scoping reasons as
before.

- **Market listings: done** (unchanged from the original entry below).
- **Market buying: done.** `growAICivilizations` now also (3% chance/tick,
  separate roll from listing) buys an available listing that isn't its
  own, mirroring `HandleBuyListingCallback`'s exact mechanics - dollar
  transfer, resource transfer with storage-cap clamp, `is_sold` flag, a
  notification to the human seller (skipped when the seller is another
  AI faction - negative `telegram_id`, nothing reads its notifications).
  Re-checks `is_sold` under a row lock immediately before committing, the
  same race-safety `HandleBuyListingCallback`'s own `FOR UPDATE`
  provides against a human buying the same listing at the same moment.
  New tests: `TestGrowAICivilizationsCanBuyAvailableListing`,
  `TestGrowAICivilizationsWontBuyItsOwnListing`.
- **Clan creation and application: done.** Also in `growAICivilizations`
  (2% chance/tick, only when not already in a clan), a faction either
  founds its own clan (mirrors `HandleCreateClanCommand`: free, a
  `clans` row plus a `Leader` row in `user_clans`, named "`<faction
  name>` Alliance") or applies to an existing recruiting human-led clan
  (mirrors `HandleApplyToClanCallback`: a pending `clan_applications`
  row plus a notification to the leader - **does not auto-join**; a
  human clan leader still reviews and accepts or rejects it exactly as
  if a human had applied, so this never bypasses a leader's control over
  their own clan roster). Deliberately only applies to clans led by a
  real human (`leader_id > 0`), not another AI faction's clan - avoids
  the odd edge case of AI factions clustering into each other's clans
  unsupervised, which wasn't part of what was asked. New test:
  `TestGrowAICivilizationsCanFoundOrJoinAClan`, which explicitly asserts
  the no-auto-join guarantee holds.
- **"Unit upgrades": still not built**, same reasoning as before - this
  needs a direct answer on what "upgrades" means (the existing
  garrison-building roll, or the human research/building-upgrade tree in
  `internal/game/researchplanner`) before anyone builds it.
- **Clan wars: done.** Also in `growAICivilizations` (2% chance/tick,
  only for a faction with `role = 'Leader'` in `user_clans` - a
  rank-and-file AI member can't declare war, matching the human
  "Leaders only" rule), mirrors `HandleDeclareClanWarCallback` exactly:
  picks a random enemy clan not already at war, 48h duration, notifies
  all members of both clans (real humans only). War *scoring* itself
  needed zero new code - it's computed from the `raids` table by
  whatever already resolves `clan_wars`, and an AI-launched raid via
  `launchAIRaid` was already an ordinary `raids` row, so it already
  counted once this shipped. New tests:
  `TestGrowAICivilizationsCanDeclareClanWar`,
  `TestGrowAICivilizationsWontDeclareWarWhenNotLeader`.
- **Federations: done.** Also in `growAICivilizations` (1% chance/tick,
  Leader-only, only when the clan isn't already federated), 50/50
  between founding (mirrors `HandleFoundFederation`: costs
  5000 Crystal - the constant is duplicated as a literal rather than
  imported from `internal/bot/handlers/federation.go`, since the tick
  engine deliberately doesn't depend on the `bot/handlers` package, the
  same reasoning the exchange lot prices were duplicated for) and
  joining an existing one (mirrors `HandleJoinFederation`: free, just
  points the clan at it). New test:
  `TestGrowAICivilizationsCanFoundOrJoinFederation`.
- **Arena queueing: done.** Also in `growAICivilizations` (2%
  chance/tick), mirrors `HandleJoinQueueCallback`'s exact mechanics
  (entry fee debited, an `arena_queue` row inserted) using the cheapest
  `'solo'` bracket. New test:
  `TestGrowAICivilizationsCanQueueForArena`.
- **The rest of "literally everything"/"many other" abilities: still not
  built, deliberately**, for the same one-subsystem-at-a-time reasoning
  as before. Updated inventory of what's still missing, now that buying,
  clans, clan wars, federations, and arena queueing are done:
  research/building upgrades, hero recruitment and equipping, job
  assignments, diplomacy actions (`internal/bot/handlers/diplomacy.go` -
  alliance/NAP pacts between clans, distinct from clan wars), spy
  missions (`HandleSpyCallback` - blocked on AI factions not currently
  building the "Spy Device" drones a spy mission requires; would need
  `growAICivilizations` to also build drones first, or a different
  resourcing path), and exploration (`exploration_sites` - distinct from
  the `scout_missions` Item 3 covers). Each remains its own subsystem
  needing its own read-the-handler-first pass, exactly like every item
  above got.
- **Suggested next subsystem**: diplomacy (alliance/NAP pacts) is
  probably next-most self-contained, since it follows the same
  clan-Leader-gated pattern as clan wars/federations above and this
  session already read `diplomacy.go`'s function list while scoping this
  round. Spy missions need the drones prerequisite solved first. Confirm
  before starting, same as always.



## Suggested build order

1. ~~Item 3 (multi-scouting) first~~ - **done, merged 2026-08-01.** Smallest,
   most self-contained, no balance risk; the audit work (confirming the
   tick side had no singular-mission assumptions) turned out to already
   be satisfied by the existing per-row design.
2. ~~Item 2 (cadence/probability tuning) next~~ - **fairness direction and
   AI-vs-AI probability done, merged 2026-08-01** per the project owner's
   direct answers (quoted above); `aiDecisionCadence` deliberately left
   unchanged this round, see the note at the end of Item 2.
3. ~~Item 1 (continuous spawning) next~~ - **done, merged 2026-08-01.**
   Built against the fixed-8-plus-2026-08-01-tuning baseline as planned,
   so its effects can be observed cleanly before anything else compounds
   on top.
4. Item 4 (AI factions using the rest of the game) - **market listing,
   market buying, clan creation/application, clan wars, federations, and
   arena queueing done** (listing merged with Item 1; the rest added
   across two follow-up rounds after direct project owner instruction);
   everything else in Item 4 (research/building upgrades, hero
   recruitment, jobs, diplomacy, spy missions, exploration) remains open
   and is realistically its own multi-session project - see Item 4's own
   section for the reasoning and a suggested starting point (diplomacy,
   now that the clan-Leader-gated pattern is established).

## Testing strategy

Same standard as every other tick-phase addition in this codebase per
`AI_FACTION_DECISION_LOOP_PLAN.md`'s and
`AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md`'s testing sections: real
Postgres in tests (no mocking the DB), deterministic seams for anything
probability-based (inject the RNG or assert on distribution over many
runs, don't assert a single roll's outcome), and a regression test for
the exact bug this plan is fixing (e.g. "a level-11 human with no raids
in 24h becomes AI-eligible" as a concrete test case if that guarantee
mechanism from Item 2 gets built).

## Open questions for the project owner (don't guess these, ask)

All of this section's original questions (Item 1's spawn rate and Item
2's fairness/probability direction) were answered directly by the
project owner on 2026-08-01 - see each item's own section above for the
exact quotes and what was built from them. What's still open, from what
surfaced while building those answers:

1. **Item 1**: is there ever a ceiling on total AI faction count, or does
   the world keep accumulating factions indefinitely? Assumed "no cap"
   for now based on the "always building up and competing" framing, but
   that's this session's inference, not a confirmed answer - flagged in
   Item 1's own section too.
2. **Item 4**: does "unit upgrades" mean the existing
   garrison-building system already counts (arguably close enough today),
   or specifically the human research/building-upgrade tree
   (`internal/game/researchplanner` and related) that AI factions don't
   touch at all yet? Which subsystem from Item 4's inventory (research,
   heroes, jobs, clans, ...) matters most to build next, if any?
