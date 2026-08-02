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

## Item 4: AI factions actively using the rest of the game (market listing done; the rest is a much larger, separate effort)

**Status: partially built (2026-08-01) - market listing only. Everything
else below is intentionally not attempted this round**, and this section
exists to say clearly why, rather than silently leaving it undocumented.

The project owner's answer to Item 1 (quoted there in full) asked for AI
factions to "literally do every single thing in the game" - explicitly
naming unit upgrades and market listings as examples, "just like real
human." What actually shipped:

- **Market listings: done.** `growAICivilizations` now occasionally
  (3% chance/tick, same shape as its existing 5%-chance garrison-building
  roll) lists surplus metal or crystal on `market_exchange`, using the
  *exact* fixed lot size and price a human posting via `/exchange`
  uses (`HandlePostListingCallback` in
  `internal/bot/handlers/exchange.go`: 50 metal/150 dollars, 20
  crystal/300 dollars) - not AI-specific pricing, so a listing on the
  shared exchange looks identical regardless of who posted it, same
  principle `launchAIRaid` already follows for raids. Only lists when a
  real buffer remains afterward. New test:
  `TestGrowAICivilizationsCanListSurplusOnExchange`.
- **"Unit upgrades": not built, and this needs a real answer before
  anyone builds it, not a guess.** This codebase has a genuine
  research/building-upgrade system for human players (see
  `internal/game/researchplanner`, `internal/bot/handlers/advisors_panel.go`
  and related handlers) that this implementing session did not read in
  depth - wiring AI factions into it means understanding its full data
  model, choice logic, and cost/time balance first, which is real
  scoping work in its own right, not a same-day add-on next to spawning
  and market listings. `growAICivilizations`'s existing garrison-building
  roll already produces *more units*; "upgrades" as the project owner
  used the word may mean that existing system is already close enough,
  or may specifically mean the research tree - worth a direct follow-up
  question rather than assuming either way.
- **"Literally everything a human can do" more broadly: not built,
  deliberately.** A non-exhaustive inventory of what a human player can
  do that no AI faction does today: research/building upgrades (above),
  hero recruitment and equipping, job assignments, clan membership and
  clan wars, diplomacy actions, arena queueing, spy missions, buying (not
  just selling) on the exchange, exploration (`exploration_sites` -
  distinct from the `scout_missions` this plan's Item 3 covers), and
  probably more not listed here. Each of those is its own subsystem with
  its own rules a faithful "AI does it too" implementation would need to
  respect - attempting all of them in one uncoordinated pass, without
  reading each subsystem first the way this session read `exchange.go`
  before writing the market-listing code above, risks producing
  something that compiles and passes a shallow test but doesn't actually
  behave like the real system (e.g. an AI "hero" that doesn't interact
  with combat the way a human's hero does). This is realistically its
  own multi-session project, best tackled one subsystem at a time using
  the same pattern established here each time: read the human-facing
  handler for that subsystem first, then mirror its exact mechanics for
  an AI faction, one subsystem, one commit, one set of tests - not a
  single sweeping change.
- **Suggested next subsystem, if this is picked up**: job assignments or
  hero recruitment are probably the next-highest-value, most
  self-contained additions (bounded scope, don't touch combat balance
  the way research upgrades would) - but that's this session's guess,
  not a project-owner decision, so confirm before starting rather than
  assuming.



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
4. Item 4 (AI factions using the rest of the game) - **market listing
   done, merged alongside Item 1** (small enough to build in the same
   pass); everything else in Item 4 (research/building upgrades, hero
   recruitment, jobs, clans, and the rest of the inventory listed there)
   remains open and is realistically its own multi-session project - see
   Item 4's own section for the reasoning and a suggested starting point.

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
