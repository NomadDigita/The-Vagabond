# AI Faction Decision Loop Plan

Status: **implemented and merged to `main`** - despite this header having
said "not started" until 2026-08-01, `decideAIFactionActions` (and its
helpers `decideOneAIFaction`, `pickFairAIRaidTarget`, `aiScout`,
`launchAIRaid`, `maybeAIFlee`) exist in full at
`internal/engine/tick/aidecisions.go`, are registered in the tick phase
list as `"ai_civilization_decisions"`, and are covered by
`aidecisions_test.go`. This status line was stale - confirmed by reading
the actual code, not assumed - and left a false "not started" trail for
anyone picking this repo up cold. See the new
`BUGS_AND_INCONSISTENCIES.md` entry for 2026-08-01 ("AI raid frequency
investigation") for why this system, though real, produces raids rarely
enough that a small/low-level player base can go days without ever being
hit - it's a tuning/fairness-gate issue, not a "never runs" issue.

The rest of this document remains useful as the as-built design reference
(it matches the shipped code closely) even though the "not started"
framing above no longer applies - read it for the WHY, just don't trust
the status header anywhere else in this section.

---

**Original status note (now superseded, kept for history):** not started -
this document is the design, written before any of it exists, specifically
so a different session (human or AI) can pick this up mid-build without
re-deriving anything. If you're reading this to resume work, read this
whole file before writing code - it explains WHY each piece is shaped the
way it is, not just what to type.

This is the second half of MMO_WORLD_EVOLUTION_PLAN.md's Phase 6. See that
file's "Completed implementation detail: persistent AI civilizations
(Phase 6, foundational tier)" section (near the bottom) for what already
exists: 8 seeded AI factions, each a real `encampments` row with
`is_ai_faction = TRUE`, fully discoverable/raidable/lootable through the
existing pipeline with zero special-casing, growing passively via
`growAICivilizations` (gather + occasional build + level up). None of that
is touched by this plan - it's additive.

## The actual goal, stated precisely

Today, an AI faction never *decides* anything. It cannot attack a player,
scout, or appear on the road. This plan makes an AI faction periodically:

1. **Scout** - discover a nearby target it hasn't seen yet (mirrors what a
   human does by marching past something, or exploring).
2. **Assess** - given what it's discovered and its own force, decide
   whether to attack, and if so, whom.
3. **Raid** - launch a real `raids` row with itself as `attacker_id`,
   using the exact same march/road-encounter/combat/loot pipeline a human
   player's raid uses. No parallel AI-only combat system.

Exit criteria (copied from MMO_WORLD_EVOLUTION_PLAN.md Phase 6, restated
for this half specifically): a player can be attacked by an AI faction
they've never interacted with, receives the same notifications and has the
same road-encounter/road-base-encounter opportunities they'd have against
a human attacker, and the AI's decision to attack was based only on
information it discovered through the same `encampment_discoveries`
mechanism a human uses - never on hidden omniscient knowledge of where
players are or how strong they are.

## Why this is a separate, larger effort than `growAICivilizations`

`growAICivilizations` is arithmetic: add some resources, roll a die,
increment a counter. This is genuinely different in kind - it requires:

- **A real decision under uncertainty**, using only information the AI
  faction has "legitimately" discovered, computed the same way a human's
  discovery works.
- **Fairness guardrails** a human choosing a target doesn't need, because
  a human raiding a much weaker player is a social/griefing problem the
  game already tolerates (players choose their own targets), but 8 AI
  factions with no restraint mechanically hammering the weakest new
  players on the map would be a structural problem the AI's designer is
  responsible for, not the AI's "choice."
- **Concurrency safety**: multiple tick workers must not have two AI
  factions decide to raid in the same tick and race each other into an
  inconsistent state, or decide twice from a stale read.
- **Full reuse discipline**: every single field the human raid-launch flow
  populates (see below) has to be populated correctly, or the raid will
  silently misbehave somewhere downstream (this is exactly the class of
  bug `evaluateRoadBaseEncounters`' AI-exclusion check had - a plausible-
  looking assumption that turned out to be wrong once it met the rest of
  the system).

## What already exists that this reuses for free

This is the single most important fact for whoever builds this: **because
an AI faction is a real `encampments` row, not a special entity type, the
entire raid/combat/road/loot/notification pipeline built across Phases 3-5
this session works on an AI-launched raid with zero changes**, as long as
the `raids` row this plan creates has exactly the shape a human-launched
one has. That pipeline includes:

- `resolveRaidCombats` (base-to-base combat resolution)
- `evaluateRoadEncounters` / `evaluateRoadBaseEncounters` (road contact,
  now that both expedition-vs-expedition AND expedition-vs-base exist)
- `evaluateRouteWeatherIncidents`, `processSupplyConvoys` (Phase 5)
- `emitRaidRadarWarnings` (defender gets warned exactly like against a
  human)
- Loot capture, battle reports, `notifications.Queue` (the defender is a
  real player with a real `telegram_id`, so they get a real Telegram
  message - no special AI-attacker branch needed anywhere in that code)

None of that pipeline currently branches on `is_ai_faction` for the
*attacker* side at all (it does for the *defender*/target side in a couple
of places - see "Existing AI-awareness inventory" below). That's the
design property to preserve: don't add `if isAI { ... }` branches into the
resolution pipeline. Get the `raids`/`raid_forces` rows shaped correctly at
creation time, and everything downstream already works.

### Existing AI-awareness inventory (don't break these)

- `internal/bot/handlers/combat.go`'s `HandleLaunchRaidCallback` already
  has an `isAI` branch - but that's for a **human** attacking a Rogue
  Drone Nest (`defender_id = NULL`), a completely different, older concept
  than an AI faction. Don't confuse the two. Rogue Drone Nests are
  untouched by Phase 6 per its milestone 5 and stay that way.
- `internal/engine/tick/roadbaseencounter.go`'s `evaluateRoadBaseEncounters`
  filters `e.is_ai_faction = FALSE` when picking a passive base for a
  human column to stumble into - i.e. **an AI faction cannot currently be
  ambushed as a passive base target**. Once AI factions launch mobile
  expeditions, decide whether AI-owned *moving* raids should be visible to
  `evaluateRoadEncounters`'s expedition-vs-expedition matching (almost
  certainly yes - that's the whole point, "AI factions actually appear on
  the road") - that function has no AI filter today, so AI-vs-human road
  encounters should work automatically once an AI raid exists with
  `movement_state = 'moving'`. Verify this with a real test before
  assuming it (see Testing strategy below) - it hasn't been proven, only
  reasoned about.
- `admin.go`, `profile.go`, `world.go`, `ranking.go` all filter AI
  factions out of various "real player" counts/leaderboards - if this
  plan adds any new admin/leaderboard-adjacent query, audit whether it
  needs the same filter.

## Data model additions

New migration (check `migrations/` for the current highest number before
picking one - do not hardcode a number in code without checking, since
other sessions may have added migrations after this doc was written):

```sql
-- AI decision cadence + audit trail. One row per AI faction encampment.
ALTER TABLE encampments ADD COLUMN IF NOT EXISTS ai_last_decision_at TIMESTAMP WITH TIME ZONE;

-- Every AI decision, whether or not it resulted in a raid - useful for
-- both debugging ("why did faction X never attack anyone") and a future
-- devconsole metric (Phase 7 milestone 3 already has an `ai_growth`
-- intent; a natural follow-up is `ai_activity` once this exists).
CREATE TABLE IF NOT EXISTS ai_faction_decisions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    encampment_id UUID NOT NULL REFERENCES encampments(id) ON DELETE CASCADE,
    decided_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    intent VARCHAR(20) NOT NULL,        -- 'scout', 'raid', 'idle'
    target_encampment_id UUID REFERENCES encampments(id) ON DELETE SET NULL,
    resulting_raid_id UUID REFERENCES raids(id) ON DELETE SET NULL,
    reason TEXT                         -- short human-readable "why" for debugging
);
CREATE INDEX IF NOT EXISTS idx_ai_faction_decisions_encampment ON ai_faction_decisions(encampment_id, decided_at DESC);
```

Add this as its own migration file (`03X_ai_faction_decision_loop.sql`,
check the actual next number) plus the matching statements appended to
`internal/db/schema/schema.go`'s `Statements()` (NOT inline in
`cmd/bot/main.go` any more - that was refactored out this session
specifically to make migrations testable; see "Testing strategy" below,
and don't reintroduce the old inline pattern).

## The decision loop itself

New tick pass, e.g. `decideAIFactionActions(ctx, tx)`, registered in the
tick phase list (`internal/engine/tick/engine.go`'s `runTick`-equivalent
registration list, right after `growAICivilizations`). Suggested file:
`internal/engine/tick/aidecisions.go` (new file, matching the existing
convention of one new file per meaningfully-separate tick concern - see
`roadbaseencounter.go` from this session for the pattern to copy: its own
file, its own doc comments, registered in the same list).

### Step 1: cadence gate (avoid an 8-faction fire-drill every tick)

```sql
SELECT id, level, ai_faction_key
FROM encampments
WHERE is_ai_faction = TRUE
  AND (ai_last_decision_at IS NULL OR ai_last_decision_at <= CURRENT_TIMESTAMP - INTERVAL '20 minutes')
```

Pick a real cadence deliberately, don't guess - 15-30 minutes per faction
feels right for 8 factions total (roughly one faction deciding something
every 2-4 minutes world-wide), but there's no existing precedent to copy;
whoever builds this should sanity-check against how often a human plays
in a session. Update `ai_last_decision_at = CURRENT_TIMESTAMP` immediately
after selecting a faction to act on, in the same transaction, before doing
anything else - this is both the cadence gate AND the concurrency lock
(see below).

### Step 2: concurrency safety

Use `SELECT ... FOR UPDATE SKIP LOCKED` on the encampments row when
picking which AI factions get to act this tick, exactly like the pattern
`autoLaunchExpiredStagedRaids` and other existing tick passes already use
for raid rows (`git grep "FOR UPDATE SKIP LOCKED"` to find the existing
examples to copy from - don't invent a new locking idiom). This guarantees
two concurrent tick workers (if the engine is ever scaled horizontally)
never both act on the same AI faction in the same window.

### Step 3: intent selection

For a faction whose cadence gate passed, roll an intent:

- **scout** (see below) if the faction has fewer than N (suggest 3-5)
  currently-undiscovered-by-it real players within its home continent.
- **raid** if it has at least one already-discovered real player target
  that passes the fairness check (below), weighted so raid doesn't happen
  every single cycle - suggest a flat probability (e.g. 40%) even when a
  valid target exists, so AI aggression feels like a threat, not a
  metronome.
- **idle** otherwise (nothing eligible, or the probability roll missed).

Log every decision (even idle) to `ai_faction_decisions` with a `reason`
string - this is what makes this debuggable months from now instead of a
black box.

### Step 4: scout intent

Mirror `resolveExplorationDiscovery`'s actual mechanism (read that
function in `internal/engine/tick/engine.go` before writing this - don't
re-derive discovery logic from scratch, copy the existing shape): find a
real player's `encampments` row in the AI faction's home continent that
has no `encampment_discoveries` row with this AI faction as observer, and
insert one. This is the entire "no omniscience" compliance step - an AI
faction can only ever raid something that has a real
`encampment_discoveries` row naming it as observer, created here (or
incidentally via `discoverRouteContacts`/`evaluateRoadBaseEncounters` once
the AI has a mobile expedition on the road - which becomes possible once
this whole plan is live, a nice emergent bootstrapping loop).

### Step 5: raid intent - target selection and fairness

Query `encampment_discoveries` for real players (not other AI factions -
AI-vs-AI war is explicitly out of scope for this plan, see "Deliberately
out of scope" below) this faction has already discovered, join their
`workshop_inventory` to estimate defensive strength, and apply a fairness
band: **do not select a target whose level is more than roughly 2-3 levels
below the AI faction's own level.** (Exact numbers need play-testing, not
guessing - start conservative, i.e. err toward the AI skipping a raid
rather than curbstomping a new player, and loosen only if it turns out AI
factions are too passive in practice.) This is the guardrail Phase 6's
plan doc flagged the need for ("independently progress without receiving
hidden resources or knowledge") - it's about protecting weak *players*
from unrestrained AI, which is a designer responsibility, not something
the AI "chooses."

### Step 6: launching the raid - copy the human flow exactly

This is the part most likely to be gotten subtly wrong, so it gets its own
section. Read `internal/bot/handlers/combat.go`'s
`HandleLaunchRaidCallback` end-to-end before writing this (search for
`INSERT INTO raids` - there are two near-identical branches there, an
`isAI` one and a real-player one; ignore the `isAI` one, it's the
different Rogue-Drone-Nest concept described above; copy the *shape* of
the real-player branch). At minimum, the new `raids` row needs:

- `attacker_id` = the AI faction's `encampments.id`
- `defender_id` = the target's `encampments.id` (a real player)
- `state = 'marching'`
- `resolve_time` = now + march duration (compute the same way the human
  flow does: base march time modified by distance/route, NOT a fixed
  constant - copy the formula, don't approximate it)
- `attacker_rations`/`attacker_ammo`/`attacker_electricity`/
  `attacker_logistics` = 100.0 each (matches what the human flow sets for
  a fresh launch)
- `origin_x`/`origin_y`/`origin_region` = the AI faction's own coordinate
- `destination_x`/`destination_y`/`destination_region` = the target's
  coordinate
- `leg_started_at = CURRENT_TIMESTAMP`, `leg_total_minutes` = the same
  march duration, so `roadcombat.RouteProgress`/`CurrentPosition` (used by
  `evaluateRoadEncounters` and the Expedition Radar-equivalent, if AI
  raids are ever surfaced there) compute a real moving position, not a
  degenerate one
- `movement_state = 'moving'` (the column default, but set it explicitly
  for clarity)
- A matching `raid_forces` row: how many soldiers/mechs the AI commits.
  Don't send the AI faction's *entire* garrison - it needs to keep some
  defense at home (mirrors the human "Manual Defense Garrison" concept
  from `combat_road_encounters.go`'s `loadBaseGarrisonForce`, though it
  doesn't have to reuse that exact mechanism). A reasonable default:
  commit at most ~60-70% of current soldiers/mechs, leaving the rest
  home.

After the insert, deduct the committed soldiers/mechs from
`workshop_inventory` exactly like the human flow does (`UPDATE
workshop_inventory SET soldiers = soldiers - $1, mechs = mechs - $2 ...`) -
forgetting this is the single easiest way to accidentally let an AI
faction duplicate units out of nothing.

Queue a `world_news` row too (`INSERT INTO world_news (headline) VALUES
(...)`) exactly like the human flow does - a "Rogue Faction has deployed
forces toward Outpost X" headline is exactly the kind of ambient world
event this game already surfaces for everything else.

## Deliberately out of scope for this plan

- **AI-vs-AI raids.** Two AI factions fighting each other adds no value a
  player would ever see and multiplies the testing surface. Filter AI
  factions out of target selection explicitly (`e.is_ai_faction = FALSE`
  on the target side too, mirroring the exact fix
  `evaluateRoadBaseEncounters` needed this session).
- **AI research/tech trees.** The plan doc's milestone 3 lists "research"
  as an intent; this plan defers it. An AI faction's effective strength
  should come from `growAICivilizations`' existing garrison growth, not a
  parallel research system that would need its own balancing pass.
- **AI reinforcement of an ongoing raid.** Mirrors
  `internal/bot/handlers/combat_road_encounters.go`'s reinforcement-convoy
  concept (Phase 5); an AI faction sending a convoy to rescue its own
  stranded expedition is a reasonable future increment, not this one.
- **Retiring Rogue Drone Nests.** Explicitly deferred by Phase 6's
  milestone 5 until "migration of their player experience is complete" -
  nothing in this plan changes that judgment.

## Testing strategy

Follow the pattern this session established in
`internal/engine/tick/roadencounter_test.go` and
`internal/db/schema/schema_test.go`: **real Postgres, not mocks.** If
`SCHEMA_TEST_DATABASE_URL` isn't set in whatever environment picks this
plan up, install Postgres locally first (`apt-get install postgresql`
works in this project's sandboxed dev environment; see this session's
history for the exact commands) - don't substitute a hand-rolled mock of
Postgres-specific SQL semantics, it will hide real bugs the way the
AI-exclusion bug in `evaluateRoadBaseEncounters` almost did.

Minimum test coverage before considering this plan done:

1. An AI faction past its cadence gate that has zero discovered targets
   picks `scout`, not `raid`, and the resulting `encampment_discoveries`
   row makes it eligible to raid on a later decision.
2. An AI faction with a discovered target that fails the fairness check
   (too weak) does not raid it.
3. An AI faction with a discovered target that passes the fairness check
   raids it: assert the resulting `raids` row has every field a human-
   launched raid would have (compare against what
   `HandleLaunchRaidCallback` sets - don't just assert "a row exists"),
   the AI's `workshop_inventory` was actually debited, and a
   `raid_forces` row exists.
4. **The actual exit-criteria test**: seed an AI-launched raid marching
   toward a human player's base, seed a second human expedition marching
   through the same space, run `evaluateRoadEncounters`, and confirm the
   AI's raid is a valid participant in a road encounter exactly like a
   human's would be. This is the single most important test in this
   plan - it's the concrete proof of "AI factions actually appear on the
   road via the already-built Phase 3/4 road-encounter system," which is
   the whole point.
5. Two AI factions never raid each other (assert this holds even after
   many decision-loop iterations, similar to how this session's
   `TestEvaluateRoadBaseEncountersExcludesAIFactionBases` ran 40 iterations
   to make a probabilistic absence-check meaningful, not just lucky once).

## Suggested build order (so this can be picked up incrementally)

1. Migration + `ai_last_decision_at` + `ai_faction_decisions` table.
2. Cadence gate + concurrency-safe faction selection, with intent always
   forced to `idle` for now (i.e. land the skeleton with logging, prove
   the tick pass runs safely, before any real decision logic exists).
3. Scout intent (smaller, no combat/economy risk).
4. Raid intent's target selection + fairness check, with the actual raid
   launch stubbed to just log "would raid X" instead of creating a real
   row - lets the target-selection logic be tested and tuned in isolation.
5. The real raid launch (Step 6 above) - the highest-risk piece, build it
   last, with test #3 and #4 above passing before considering it safe to
   ship.
6. Update `MMO_WORLD_EVOLUTION_PLAN.md`'s Phase 6 ledger row once this
   lands, the same way this session closed out the "expedition-vs-base"
   gap in Phase 4's row - don't just add a new row, edit the stale
   "AI factions do not yet make decisions" sentence so the doc doesn't
   contradict itself (see that file's existing edit history for the
   pattern: the Phase 4 row literally says "**Not yet covered (closed
   2026-07-26, see below)**" as an example of how to do this cleanly).

## A note on process, given this session's history

Every phase in this project so far has shipped via a branch built in
parallel with something else, and collided on merge at least once. If
another agent session is already working on something adjacent (AI
behavior, combat, or the raid-launch flow) when you pick this plan up,
check for open branches/PRs before starting - `git branch -a` and the
GitHub PR list, not just `main`'s latest commit. This plan touches
`combat.go`'s raid-launch logic and `engine.go`'s tick registration, both
of which have been hot spots for exactly this kind of collision all
session. If you find a conflicting branch, resolve it the way this
session resolved PR #31: read both sides' intent before picking one, don't
mechanically take the newer timestamp.
