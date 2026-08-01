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

Today `seedAICivilizations` (`cmd/bot/main.go`) inserts exactly 8 AI
factions once, idempotently, at boot, and nothing ever adds a 9th. The
request is for **new AI factions to keep appearing over time** (the
player suggested "at least 3 new AI spawn on the planet daily"), each one
then growing/leveling/building a fleet the same way the original 8 do via
`growAICivilizations`, and eventually acting via `decideAIFactionActions`
- i.e. a new spawn should need zero special-casing in either of those two
functions, it's just another `is_ai_faction = TRUE` encampment row.

Suggested shape:
- New tick phase, e.g. `spawnNewAIFactions(ctx, tx)`, registered in
  `internal/engine/tick/engine.go`'s phase list near
  `"ai_civilization_growth"`/`"ai_civilization_decisions"`.
- Needs a real cadence gate the same shape as
  `aiDecisionCadence`/`ai_last_decision_at` uses for individual factions,
  but world-scoped this time (e.g. a `world_state` or
  `last_ai_spawn_at` singleton row, or reuse the existing
  `world_events`-style table if one already fits - check before adding a
  new table) so this doesn't need to re-derive "how many spawned today"
  from a COUNT query with fuzzy day boundaries every tick.
- Needs a procedural name/key generator (the current 8 are hand-authored
  flavor names like "Ironclad Directive" - either extend that authored
  list well past 8 upfront and spawn from the tail of it, or build a
  simple word-bank combiner; either is fine, but keep `ai_faction_key`
  genuinely unique, since `seedAICivilizations`'s idempotency check
  depends on it).
- Needs a spawn coordinate strategy that doesn't collide with an existing
  encampment and roughly balances the 4 continents (mirror whatever
  `seedAICivilizations` or the human onboarding flow already does for
  fresh coordinates - don't invent a third way to pick a starting tile).
- Starting level: keep new spawns intentionally weak (the original 8
  start at level 3 or 6) so a growing AI population doesn't front-load
  the world with instantly-dangerous new threats - let
  `growAICivilizations`'s existing passive leveling do the ramp-up over
  time, exactly like it already does for the original 8.
- **Open product question, don't guess**: is there any cap on total AI
  faction count, or does this run forever? "New one keeps spawning...
  every minutes and secs too" plus "3 new AI daily" read as somewhat
  contradictory (sub-minute spawns would be *far* more than 3/day) - get
  a real target rate from the project owner before picking a cadence
  constant, the same way `aiDecisionCadence`'s "20 minutes" was a
  deliberate, stated guess in the original plan, not an accident.

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

## Suggested build order

1. ~~Item 3 (multi-scouting) first~~ - **done, merged 2026-08-01.** Smallest,
   most self-contained, no balance risk; the audit work (confirming the
   tick side had no singular-mission assumptions) turned out to already
   be satisfied by the existing per-row design.
2. ~~Item 2 (cadence/probability tuning) next~~ - **fairness direction and
   AI-vs-AI probability done, merged 2026-08-01** per the project owner's
   direct answers (quoted above); `aiDecisionCadence` deliberately left
   unchanged this round, see the note at the end of Item 2.
3. Item 1 (continuous spawning) next - the largest new subsystem, and the
   one most likely to interact badly with Item 2's tuning if built before
   Item 2's changes have been observed in practice (more factions on top
   of an already-loosened fairness band and raised AI-vs-AI probability
   compounds fast - build this against the fixed 8 first, watch how the
   2026-08-01 tuning actually plays out, then reconsider).

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

**Items 3 and 4 (both about Item 2) were answered 2026-08-01 - see
Item 2's section above for the exact quote and what was built.** Item 1's
two questions are still open and block starting that item:

1. Item 1's actual target spawn rate - "3/day" and "every minutes and
   secs" aren't the same number; which one (or something else entirely)?
2. Is there a ceiling on total AI faction count ever, or does the world
   just keep accumulating factions indefinitely?
