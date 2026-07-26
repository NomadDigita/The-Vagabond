# MMO Living-World Evolution Plan

Status: active continuation plan (2026-07-20).

This document is the implementation contract for evolving The Vagabond from
its Phase-1 expedition logistics foundation into a living, discovery-gated
MMO world. It supplements `MMO_WARFARE_LOGISTICS_PLAN.md`; it does not replace
the source-of-truth unit registry, the persistent raid lifecycle, or the
separate provider-agnostic AI infrastructure workstream documented in
`PROJECT_MASTER_PLAN.md`.

## Operating rules

- Do not modify `assets/`. Visual assets are a separate workstream.
- Preserve the existing database-backed tick model and Telegram callback
  surface. New world state must be recoverable from PostgreSQL and progress
  only through idempotent, transaction-safe tick passes or explicitly owned
  player actions.
- Reuse the current `raids` / `raid_forces` lifecycle where its state machine
  remains valid. Add narrow, named state tables for discoveries, encounters,
  route incidents, camps, and logistics convoys rather than hiding a second
  game inside unstructured JSON.
- A player can attack only a known target. Self-knowledge is implicit; all
  other knowledge must have a discovery provenance and timestamp.
- Every externally meaningful change must enqueue a notification. Tick code
  must notify only at state transitions or threshold crossings, never once per
  tick.
- Balance values in this document are deliberately provisional until backed by
  tests and an in-game balance report. Crystal remains the rarest raid loot;
  it must not be silently promoted to a routine supply currency.

## Confirmed baseline

Phase 1 is already present in the current checkout:

- Raids have carried rations, ammunition, electricity, and logistics pools;
  they can auto-return when critical supply pairs are exhausted.
- Launches require 20 combat units, at least one resource transport owned at
  home, and a travel unit when walking units are mobilized.
- Launch and speed-up economics are materially more expensive, and successful
  raids can return multiple resource types with reduced Crystal/Neuro Core
  loot coefficients.
- The current expedition state machine remains `marching -> engaged ->
  returning -> completed`. Existing exploration is site-reward dispatching;
  it does not discover encampments. Rogue Drone Nests are combat targets, not
  persistent civilian/AI factions.

## Phase 0 - Baseline audit and safety rail

Goal: make changes against verified, rather than assumed, architecture.

Milestones:

1. Review all non-asset Go, SQL, configuration, and Markdown sources; record
   defects and deliberate non-changes in this file and the relevant existing
   logs.
2. Compile, vet, and run the test suite before and after each completed phase.
3. Repair ownership checks, SQL error handling, callback registration gaps,
   and migration/startup drift discovered in the touched surface.
4. Establish focused unit tests for new pure gameplay policy so future balance
   changes do not require a live Telegram bot or database.

Exit criteria: baseline checks have recorded results, no code path added here
relies on an unregistered callback, and every new migration is duplicated in
the startup migration list exactly once.

## Phase 1 - Expedition logistics foundation [complete before this plan]

Goal: require a plausible military column and make supplies/loot meaningful.

Completed milestones: listed under **Confirmed baseline** and in
`MMO_WARFARE_LOGISTICS_PLAN.md`.

Follow-up hardening: replace home-inventory-only transport validation with
transport units actually committed to the force, and derive carried supply
capacity/consumption from force composition rather than fixed 100-point pools.

## Phase 2 - Discovery-gated world intelligence

Status: complete (2026-07-20).

Goal: no base, AI faction, or raid target is globally visible by default.

Implementation milestones:

1. Add a migration and startup statements for persistent discoveries. Each
   record stores observer encampment, discovered encampment, discovery method
   (`exploration`, `route`, `intel`, `ai_seed`), first/last seen timestamps,
   and optional confidence/intelligence metadata. A unique observer-target
   constraint makes discovery idempotent.
2. Add a small `internal/game/worldintel` policy package for discovery gates,
   visibility rules, and deterministic discovery-roll helpers. Bot handlers
   query this package instead of duplicating visibility SQL.
3. Upgrade `/explore`: its existing site expedition remains intact, but an
   arriving exploration dispatch also has a region-bounded chance to discover
   human outposts and seeded AI settlements. No target list is leaked before
   the discovery event succeeds.
4. Filter the Tactical Target Matrix, raid staging, reconnaissance, and AI
   target queries to the caller's known targets. Defend the server action as
   well as the UI: a stale or forged callback cannot launch an undiscovered
   raid.
5. Add a dedicated exploration/scout unit to the canonical unit registry and
   factory, with explicit range, survivability, maintenance, and discovery
   modifiers. Existing Observer remains a home garrison/counter-espionage
   unit and is not silently repurposed.
6. Seed compatibility discoveries only through an explicit, auditable admin
   action or natural re-exploration; never grant global visibility during
   migration.

Notifications: explorer departure, discovery, repeat sighting, discovery by
an enemy route, and target intelligence expiration/downgrade where enabled.

Exit criteria: a new player has no attackable targets until exploration or a
route encounter discovers one; the same rule applies to AI factions.

## Phase 3 - Geographic routes and real expedition state

Status: complete (2026-07-20). Route progress is deliberately derived from
the immutable route snapshot and current ETA rather than stored as a mutable
second clock. `movement_state`, `pause_reason`, and `next_route_event_at`
provide the durable state boundary for later camps and weather incidents.

Goal: turn a timer-only march into a route-aware journey without replacing
the stable raid settlement code.

Implementation milestones:

1. Persist each expedition's origin, destination, region/country route,
   travelled progress, next event check, and visibility/radar thresholds.
   Coordinates remain the authority; route snapshots protect an active march
   from later coordinate changes.
2. Introduce a narrow expedition state model: active phase (`outbound`,
   `engaged`, `returning`), movement state (`moving`, `paused`, `camped`,
   `awaiting_reinforcement`, `encounter_pending`), and reason. Keep legacy
   `raids.state` compatible during the transition.
3. Calculate travel time from origin/destination geography, mobilized travel
   capability, route choice, load, research, and active regional conditions.
   Safe routes trade speed for lower encounter/event risk.
4. Stage all required combat, travel, and resource-transport units in the
   force itself. Capacity, power draw, load, and casualty return are derived
   from the staged units, not from home inventory.
5. Delay an incoming-raid warning until the attacker crosses the target's
   radar warning range. Radar/recon research and defensive units adjust that
   range; a defender never receives the old immediate global warning.

Notifications: departure, proximity-warning crossing, route discovery,
weather/event onset, pause/resume, arrival, and return arrival.

Exit criteria: expedition radar can explain where a force is, why its ETA
changed, which phase it is in, and whether a defender is entitled to know.

## Phase 4 - Road contacts and field battles

Status: complete (2026-07-20).

Goal: armies and nearby bases can meet on both outbound and return journeys.

Implementation milestones:

1. Add `road_encounters` with both expedition/base parties, location snapshot,
   response deadline, initiator decisions, outcome, and idempotency guards.
2. On route ticks, evaluate eligible nearby expeditions and bases with one
   deterministic per-interval roll. New targets encountered en route become
   discovered even if the parties choose not to fight.
3. Notify both players immediately with Attack and Continue actions. Timeout
   resolves to continuing unless an explicitly documented aggressive doctrine
   applies. Validate action ownership, active-state, and response windows on
   every callback.
4. Reuse the battle report and casualty policy for a field battle, but resolve
   against the other expedition's staged force and carried cargo rather than
   a home defense grid. The victor can take capacity-limited cargo including
   rare Crystal; both columns retain clear state after resolution.
5. Ensure encounter resolution cannot leave a destroyed/returned expedition
   locked in `encounter_pending` and cannot duplicate loot or notifications.

Exit criteria: road contact can discover, peacefully pass, or resolve a
field battle correctly on outbound and return legs.

## Phase 5 - Weather, temporary camps, and reinforcement convoys

Goal: environmental disruption and logistics failures create recoverable
operational decisions rather than a hidden combat penalty.

Implementation milestones:

1. Add route incidents for flood, storm, heatwave, sandstorm, EMP, and
   radiation. Incidents capture location, severity, expiry/recheck time,
   movement/combat modifiers, and a resolved marker. Existing regional world
   events remain inputs, not duplicate weather engines.
2. Severe incidents create a temporary camp. A camp pauses movement, changes
   supply consumption, exposes the column to specified risks, and waits for a
   condition-clearing tick that can take 12-36 real-world hours for floods.
3. Add resource convoy staging/dispatch. A convoy requires dedicated transport
   units, has its own route and exposure, transfers only what survives to the
   named expedition, and is never a free instant refill.
4. Model depleted systems separately: loss of rations/ammo pauses or retreats
   a column; loss of electricity/logistics disables high-tech contributions
   before it can force a pause. Recovery happens only after a valid convoy
   delivery or return home.
5. Make weather/camp speed-up exceptional: price scales by remaining delay and
   event severity, consumes Crystal/premium resources, has a documented cap,
   and cannot bypass an unresolved encounter.

Exit criteria: a paused army, its cause, drain rate, recovery option, and
revised ETA are visible; convoys have a full lifecycle and cannot duplicate
supplies.

## Phase 6 - Persistent AI civilizations

Goal: replace isolated training targets with geographically distributed,
rule-bound AI societies.

Implementation milestones:

1. Add persistent AI faction and AI settlement records, each with a region,
   resources, buildings, research, unit inventory, doctrine, knowledge, and
   lifecycle status. Seed a small, balanced set through an idempotent admin
   or migration-safe bootstrap, never on every bot restart.
2. Adapt AI settlement data to the existing encampment/raid interfaces where
   practical. AI and human combat must use the same unit registry, logistics,
   discovery gates, resource caps, battle report, and loot policy.
3. Add bounded AI tick intents: gather, build, research, explore, scout,
   expand, reinforce, and raid. Intent selection must lock rows, respect
   cooldowns/resource availability, and cap per-tick work to avoid a world
   simulation spike.
4. Give AI the same no-omniscience rule: it must discover targets before
   attacking. AI action notifications are player-facing where an action can
   affect or reveal a player.
5. Retire or reframe Rogue Drone Nests only after migration of their player
   experience is complete. Existing raid records and recon flows must remain
   readable; no destructive conversion of live campaigns.

Exit criteria: a newly seeded AI society can be discovered, scouted, raided,
and can independently progress without receiving hidden resources or
knowledge unavailable to a human player.

## Phase 7 - Experience, observability, and balancing

Goal: expose the living-world rules clearly and tune them with evidence.

Implementation milestones:

1. Extend Expedition Radar into a Logistics Planner/Route Status panel:
   staged requirements, cargo capacity, supply burn, route, pause reason,
   radar visibility, encounters, and convoy controls.
2. Add notification preferences/deduplication where high-volume events could
   overwhelm players, without suppressing combat, discovery, or supply-loss
   alerts.
3. Add admin/dev-console metrics for discovery rate, travel delays, stranded
   armies, convoy outcomes, field-battle frequency, AI growth, loot mix,
   Crystal flow, and speed-up expenditure.
4. Codify balance tables for base unit electricity/resource upkeep. Phase 1's
   2.0 electricity-per-tick Agent floor is retained pending data; global
   increases occur only with documented production-rate and new-player impact.
5. Add tests for all policy boundaries, migration compatibility, critical
   idempotency paths, and notification recipient/timing rules.

Exit criteria: players can understand the cost and risk before launch, while
operators can measure whether the loop is fair and economically sustainable.

## Completion protocol for every phase

1. Update this plan's status, completed work, edge cases, and remaining work.
2. Update `MMO_WARFARE_LOGISTICS_PLAN.md`, `README.md`, and any existing
   feature log whose statements changed; add focused Markdown only when it
   becomes the best source of truth.
3. Run formatting, tests, build, and vet; document an unavailable check rather
   than claiming it passed.
4. Review the phase diff for asset changes and unrelated worktree changes.
5. Commit only the completed phase with a descriptive message. Do not commit
   concurrent developers' unrelated changes.

## Current session ledger

| Date | Phase | State | Evidence / notes |
|---|---|---|---|
| 2026-07-20 | 0 | in progress | Repository cloned; architecture, existing Phase-1 plan/diff, active gameplay log, and the continuation's touched non-asset surfaces reviewed. The exhaustive audit remains active alongside implementation. |
| 2026-07-25 | - | handoff note | A prior Codex session's transcript described Phase 3 (road encounters, weather/camps) as implemented, but `git log`/`git ls-remote` on `origin` showed only Phases 0-2 were ever merged (`0219c25`, PR #27) - that session's later work existed only in its own sandbox and was lost when its usage quota ran out before it could push. Claude picked up from the actual merged state (Phase 2 complete) rather than the transcript's claimed state. **Lesson for future sessions/developers: verify the real `git log` on `origin/main` before trusting a prior session's self-report of what's "done."** |
| 2026-07-20 | 1 | complete and hardened | Existing `027_mmo_warfare_logistics_phase1.sql` supplies the foundation. `029_mmo_transport_staging_and_force_recovery.sql` closes the false home-inventory transport check and returns all tracked support units after a campaign. |
| 2026-07-20 | 2 | complete | `028_mmo_world_discovery_and_radar.sql` adds directional discoveries and route snapshots. Exploration can discover rival outposts or the Rogue Nest, Scouts affect discovery odds, targets are filtered and launch-authorized by discovery, route proximity creates reciprocal knowledge, and radar warnings are sent once at capability-dependent proximity. |
| 2026-07-20 | 3 | complete | Route snapshots, movement state, ETA-derived progress, actual staged support units, radar-proximity warnings, and route/status information in Expedition Radar are live. |
| 2026-07-20 | 4 | complete | Active raid columns meeting within one coordinate create a 10-minute response window. `/encounters` offers Attack/Continue; either attack resolves a renderer-backed field battle, applies force casualties, delays the route, and transfers only carried raid cargo. |
| 2026-07-20 | 5-7 | pending | Temporary camps and convoy reinforcement, persistent AI civilizations, and observability/balance tooling remain to be implemented. |
| 2026-07-25 | 3 | complete | `030_mmo_route_legs_and_road_encounters.sql` replaces the resolve_time-derived position estimate with a stable per-leg clock (`leg_started_at`/`leg_total_minutes`, freezable via `paused_at`). Route discovery, radar warnings, and the new road-encounter scan all read position from the same `roadcombat.RouteProgress`/`CurrentPosition` functions, so a paused or delayed column no longer drifts. The expensive speed-up now shortens the leg to match its pulled-forward arrival instead of silently desyncing position from ETA. |
| 2026-07-25 | 4 | complete (expedition-vs-expedition only) | `road_encounters` table + `evaluateRoadEncounters`/`expireRoadEncounters` tick passes detect two converging expeditions, freeze both, and open a 3-minute Attack/Continue window surfaced on the Expedition Radar panel. Attack is unilateral and resolves immediately via a new `roadcombat` field-battle model (separate from, but balance-consistent with, the base-raid resolver); mutual Continue or a timeout resumes both columns from their exact paused position via a leg-shift, never snapping back or skipping ahead. Winner captures a capped share of every `stolen_*` resource the loser carries, including Crystal. **Not yet covered (closed 2026-07-26, see below):** an expedition encountering a passive third-party *base* mid-route still only creates a discovery (existing Phase 2 behavior) rather than a forced battle window - the plan's milestone 2 covers "expeditions and bases," and only the expedition-vs-expedition half is done. AI factions don't yet launch mobile expeditions (Phase 6), so all current road encounters are player-vs-player. |
| 2026-07-25 | 5 | complete | `033_mmo_route_weather_and_reinforcement_convoys.sql` adds local route weather incidents (flood/storm/heatwave/sandstorm/emp/radiation - all six, milestone 1) reusing the existing continent-wide `world_events` table as an input signal, each with a distinct onset effect (see completed-implementation-detail below) rather than an identical pause with a different label; temporary camps pausing a column 12-36 real hours (milestone 2); dedicated resupply convoys gated on real Hauler+Tanker availability, distance-scaled cost, and a real ambush/exposure risk (milestone 3); rations/ammo depletion halting a column outright while electricity/logistics depletion instead disables high-tech contributions with a grace period before escalating to a full halt (milestone 4); and a Crystal-priced pay-to-clear-early option for camps, capped and floored (milestone 5). |
| 2026-07-26 | 5 | follow-up | Asiwaju asked for the remaining Phase 5 gaps (sandstorm/EMP/radiation as real local types, milestone 4's distinct power-failure behavior, milestone 5's priced camp bypass) closed, plus fixed the agent electricity-upkeep rate (base 2.0->5.0/tick, floor 0.2->2.0/tick) and added a new Crystal Exchange feature (convert Crystal into large quantities of any other resource, rate scaled by how commonly-needed the target resource is). All implemented this session - see the updated completed-implementation-detail section below and `internal/bot/handlers/crystal_exchange.go`. While closing these gaps, also found and fixed a real bug: dispatched supply convoys deducted their committed Hauler+Tanker at dispatch but never returned them on any outcome, permanently draining transports on every resupply run regardless of success. |
| 2026-07-25 | - | bugfix | While building Phase 5, found and fixed two correctness bugs in the Phase 3/4 work committed earlier the same day: (1) `resolveRaidCombats` could still fire arrival/return processing on a raid frozen by `paused_at`, because only `movement_state` was checked at the UI/tick-detection layer, not at the resolve_time-driven combat-resolution layer; (2) every unfreeze path shifted `leg_started_at` forward by the pause duration but left `resolve_time` untouched, so `resolve_time` would already be overdue the instant a pause lifted, causing an immediate premature state transition regardless of true remaining leg time. Both are fixed at every pause/unfreeze site (road encounters, weather incidents, and now supply convoys) by shifting `resolve_time` in lockstep with `leg_started_at`, and by gating `resolveRaidCombats`'s query on `movement_state = 'moving'` for marching/returning rows. |
| 2026-07-26 | 4 | complete | `032_mmo_road_base_encounters.sql` + `evaluateRoadBaseEncounters`/`expireRoadBaseEncounters` close the milestone 2 gap noted above: a moving expedition within `roadcombat.EncounterRadius` of a real player's (non-AI) base now has the same `EncounterRollChance` to open a forced Attack/Continue window, not just a passive discovery. Only the expedition's commander gets a choice (the base didn't choose to be in the road's path). Attack resolves immediately against the base's Manual Defense Garrison reserve (`workshop_inventory.garrisoned_soldiers`/`garrisoned_mechs`) as a `roadcombat.FieldForce` - deliberately no resource looting, so a full economic raid still requires the normal `/raid` flow with its existing protections (shields, pacts). Kept as a separate `road_base_encounters` table/`active_base_encounter_id` column rather than widening `road_encounters`, so the expedition-vs-expedition path is untouched. AI/seeded settlements are excluded (their own recon/skirmish flow; Phase 6 untouched).
| 2026-07-25 | 6-7 | pending | Persistent AI civilizations and observability/balance tooling remain to be implemented. |

## Known design assumptions and edge cases

- **Resolved (2026-07-26):** Asiwaju asked whether automation agents'
  electricity upkeep (`internal/engine/agent/agent.go`) was too low at
  "0.2 per tick" and directed a fix. Base upkeep raised 2.0 -> 5.0/tick;
  floor multiplier raised 0.10 -> 0.40, so the floor a fully-teched
  veteran outpost reaches is now 2.0/tick (was 0.2/tick).
- Discovery is directional: A discovering B does not cause B to discover A
  unless the event explicitly says so.
- A discovered target is not necessarily currently scouted. Future stale-intel
  confidence can affect information precision without removing the attack
  gate.
- Existing raids must finish on the legacy lifecycle; migration cannot strand
  campaigns that lack a route snapshot.
- One unit/cargo allocation cannot be staged in more than one raid or convoy.
- Route encounters must use stable locks and a canonical party ordering to
  avoid duplicate encounters when two tick workers observe the same pair.
  Implemented in Phase 4: `road_encounters` enforces `raid_a_id::text <
  raid_b_id::text` via a CHECK constraint, `evaluateRoadEncounters` always
  reorders a detected pair into that order before insert, and a partial
  unique index on `(raid_a_id, raid_b_id) WHERE status = 'pending'` makes a
  duplicate insert from a second tick worker a no-op (`ON CONFLICT DO
  NOTHING`) rather than a second encounter.
- Loot comes from carried cargo in a field battle, never directly from a
  remote home base. Implemented in Phase 4's road battles via
  `captureCargo`, which only moves the loser's own `stolen_*` columns.
- Flood speed-up is an expensive risk-management choice, not a mandatory
  premium gate: waiting remains a valid path.
- AI faction simulation requires bounded scheduling and idempotent seeds so it
  cannot flood a small world or run twice after restart.

## Completed implementation detail: discovery and staged logistics

- `encampment_discoveries` supports either a concrete player outpost or a
  named legacy/system target, with partial unique indexes for idempotency. This
  keeps the current Rogue Drone Nest hidden without a destructive conversion
  before AI-civilization work begins.
- The Tactical Target Matrix joins discoveries instead of `encampments`
  directly. Both the initial draft callback and final launch transaction
  re-check authorization; a copied callback cannot bypass visibility.
- Exploration's ordinary site reward remains unchanged. On completion it has a
  35% base first-contact chance, increased by 5 percentage points per owned
  Scout Walker and capped at 75%. It prefers an undiscovered outpost in the
  explorer's continent, falling back to the hidden Rogue Nest when none exists.
- Active raid route snapshots support a route-sighting pass. Passing within
  one coordinate of an unknown third-party outpost establishes reciprocal
  discovery and queues notifications. It does not yet create an
  attack/continue decision window; that belongs to Phase 4.
- Defender raid alerts no longer fire at departure or on every tick. A
  conditional update marks one alert only when remaining travel crosses the
  defender's radar/recon-derived threshold. Stealth reduces the threshold by
  60% but does not make detection impossible.
- Resource transports must now be drafted, decremented, persisted on
  `raid_forces`, and returned. The completed-return path also restores
  buggies, ships, jets, nukes, and every staged logistics vehicle; previously
  several mobile units could disappear after a normal campaign.

## Completed implementation detail: route state and road encounters

- Raids now retain an explicit `movement_state` and optional reason alongside
  the legacy raid lifecycle. The current implementation uses `moving`,
  `encounter_pending`, and `encounter_battle`; the same fields will represent
  weather pauses, camps, and convoy waits in Phase 5.
- A route collision pauses both expeditions for a ten-minute response window,
  adjusts their ETAs, and queues a notification directing each commander to
  `/encounters`. Pair uniqueness applies only while an encounter is active, so
  the same columns can meet again after a prior encounter has concluded.
- Either commander may choose Attack; mutual Continue or timeout lets both
  forces proceed. An attack resolves once in the tick transaction, with
  per-unit casualties, a SpaceHunt-format field-battle report, and a 15-minute
  route delay. A force with no combat units remaining turns home.
- Field battles never access home-base reserves. They can take half of the
  losing expedition's already-carried raid payload, including rare Crystal,
  then update both moving payloads atomically.
## Completed implementation detail: route legs and road encounters (Phase 3/4)

- New package `internal/game/roadcombat` (zero DB/Telegram dependencies,
  fully unit-tested) owns the pure policy: `RouteProgress` turns a leg's
  start time + planned duration into a stable 0..1 fraction; `CurrentPosition`
  interpolates origin->destination while `marching` and destination->origin
  while `returning`; `Power`/`ResolveBattle`/`CasualtiesFor`/`Survivors`/
  `CargoShare` implement a symmetric mobile-vs-mobile field battle.
- `raids` gained `leg_started_at`, `leg_total_minutes`, `movement_state`,
  `paused_at`, and `active_encounter_id`. Every place that used to write
  `resolve_time` for a marching/returning transition (launch, forced
  supply-depletion retreat, manual abort, and all three raid-combat-outcome
  return transitions) now also resets the leg clock, so a route position
  read is always accurate for the *current* leg regardless of how many
  times `resolve_time` itself has been rewritten by pauses or speed-ups.
- `discoverRouteContacts` (Phase 2) was refactored to read position from the
  same leg-based helpers instead of its old ad-hoc `resolve_time` estimate,
  fixing a latent drift bug: that estimate assumed `base_march_minutes`
  never changed after launch, which stopped being true the moment any pause
  or delay touched `resolve_time` without a matching change to the assumed
  total trip length.
- Road encounters: `evaluateRoadEncounters` (new tick pass, runs right after
  `route_discovery`) scans all `moving` marching/returning raids, computes
  each one's current position, and rolls a 35% chance for any pair within 3
  coordinate units (skipping pairs that resolved an encounter in the last 4
  minutes, so a "Continue" choice isn't immediately re-asked). A hit creates
  a `road_encounters` row, freezes both raids (`movement_state =
  'encounter_pending'`, `paused_at = now()`), and establishes reciprocal
  discovery between the two commanders regardless of what they choose next.
- Because the `notifications` table only ever delivers plain text (see
  `internal/engine/notifications`), the queued alert tells each commander to
  open their Expedition Radar rather than embedding a button - the actual
  Attack/Continue buttons render live in `HandleExpeditionRadar`, which now
  queries `road_encounters` for the caller's pending pairs.
- `HandleRoadEncounterCallback` (new file
  `internal/bot/handlers/combat_road_encounters.go`) implements the
  decision: Attack is unilateral and resolves the field battle immediately
  using each side's actual `raid_forces` composition, supply status (same
  rations/ammo/electricity/logistics depletion rule as the forced-retreat
  check), and `research_states.military_tech_lvl`. Continue only resolves
  once both sides have chosen it, or the tick engine's `expireRoadEncounters`
  times it out after 3 minutes; either path calls the same "shift
  `leg_started_at` forward by the pause duration" logic so a resumed march
  neither snaps back nor silently skips ahead.
- A road-battle loser can be reduced to zero combat units; that raid is
  immediately marked `completed` (wiped in the field, no survivors return)
  rather than being left to limp home or getting stuck in a state the
  existing raid-resolution tick passes don't expect.
- Both `HandleExpeditionActions` (speed/abort) now refuse to act on a raid
  whose `movement_state = 'encounter_pending'`, so a commander can't
  side-step an active road decision by speeding up or retreating out from
  under it.
- Deliberately deferred to a later pass rather than left half-implemented:
  expedition-vs-base road encounters (only expedition-vs-expedition exists
  today - a friendly-fire-adjacent base a column passes is still only
  discovered, per Phase 2, never forced into combat), and any AI-controlled
  mobile expedition (AI factions are still static Rogue Nests until Phase 6,
  so they cannot yet be met on the road).

## Completed implementation detail: weather route incidents and reinforcement convoys (Phase 5, partial)

- `route_incidents` reuses the Phase 3/4 `paused_at`/`leg_started_at` pause
  mechanism rather than inventing a second one: a temporary camp and an
  encounter freeze are both "something external is blocking this column,"
  they just differ in cause (local weather vs. another player) and in what
  ends them (a timer vs. a decision). `evaluateRouteWeatherIncidents` reads
  the *existing* continent-wide `world_events` table
  (`internal/engine/world`) as an input signal - an active `acid_rain`
  event over the continent a column is currently crossing raises the odds
  of a local `flood` there specifically - rather than rolling a second,
  disconnected weather system, per the plan's explicit instruction.
- Severity (1-3, rolled uniformly) maps to a 12h/24h/36h real-world clear
  time via `roadcombat.IncidentDuration`, directly matching "which might
  take a day or even longer."
- Supply depletion (`internal/engine/tick/engine.go`'s per-tick march
  consumption loop) no longer forces an immediate retreat. It now sets
  `movement_state = 'awaiting_reinforcement'` and stops - the column stays
  exactly where it ran dry until the commander either dispatches a convoy
  (`HandleDispatchConvoy`) or manually orders a retreat from the Expedition
  Radar panel (the existing abort action, now explicitly permitted in this
  movement state). A column already paused for any reason (encounter,
  camp, or reinforcement) skips this consumption check entirely rather
  than re-triggering it or continuing to drain supplies while halted.
- `HandleDispatchConvoy` prices a convoy by real distance from home to the
  stranded column's *actual current position* (computed via the same
  `roadcombat.CurrentPosition` used everywhere else, anchored to
  `paused_at` since the column isn't advancing while frozen) and requires
  at least 1 available Hauler and 1 available Tanker, mirroring the
  existing raid-launch rule that transport requirements must be real,
  staged units rather than a resource-only cost. `processSupplyConvoys`
  (new tick pass) delivers a fixed 50-point top-up to all four field-supply
  gauges (capped at the existing 0-100 scale) and resumes the column via
  the same leg/resolve_time shift used everywhere else in Phase 3-5. A
  convoy whose target has already moved on (aborted, resolved, or - not
  yet possible, since paused columns don't self-resume - any other exit)
  before it arrives fails outright: the resources and the committed
  transports' *cargo* are lost, though the Hauler/Tanker themselves return
  to the garrison at resolution either way (their own return trip isn't
  separately modeled yet - see below).
- Known simplifications, intentionally deferred rather than half-built: a
  convoy still travels on a straight point-to-point timer rather than a
  real second column that could be encountered/discovered mid-route like a
  raid can - it does now carry a real 8% flat ambush chance (`
  convoyAmbushChance` in `processSupplyConvoys`) representing that
  exposure, but that's a dice roll at delivery time, not a full
  road-encounter-style system for convoys. That would be the natural next
  increment if convoy-vs-raid interception is wanted.

## Follow-up implementation detail (2026-07-26): closing the Phase 5 gaps, agent upkeep, Crystal Exchange

- **Sandstorm/EMP/radiation are now full local incident types**, not just
  continent-level inputs that bias flood/storm/heatwave. Each has its own
  onset effect, applied once when the temporary camp begins
  (`applyRouteIncidentOnsetEffect`): flood spoils rations, storm damages
  electricity, heatwave burns extra rations, sandstorm fouls logistics
  equipment, EMP knocks out 60% of remaining electricity/logistics in a
  surge, and radiation causes real casualties (~5% of Soldiers/Mechs,
  reusing the road-battle casualty machinery) - this last one is the
  "units may die" weather event from the original request, previously
  only implemented for combat. `IncidentMatchesActiveWeather` now prefers
  the direct-named local counterpart of an active continent event
  (sandstorm -> sandstorm, not the weaker sandstorm -> heatwave
  cross-match) before falling back to the broader mappings.
- **Milestone 4 (distinct power-failure behavior) is implemented.**
  `raids` gained `high_tech_offline`/`power_outage_ticks`. Rations/ammo
  depletion still halts a column outright (nothing to fight or march
  with). Electricity/logistics depletion alone is softer: it sets
  `high_tech_offline = TRUE` immediately (mech multiplier and capital-unit
  tech multiplier disabled - see `roadcombat.Power`'s `HighTechOffline`
  handling, mirrored in `resolveRaidCombats`'s base-raid formula) without
  the blanket -50% offense penalty rations/ammo depletion applies, and
  only escalates to a full `awaiting_reinforcement` halt after 5
  consecutive ticks of continued power failure (a grace period).
- **Milestone 5 (priced early camp clear) is implemented.**
  `handleBreakCampEarly` lets a commander pay Crystal to resolve an active
  `route_incidents` row immediately via the same leg/resolve_time-shift
  resume logic used everywhere else in Phase 3-5, instead of waiting out
  the full 12-36h. Cost scales with severity and hours remaining
  (`breakCampCrystalPerSeverityHour = 0.75`), floored at 3 Crystal and
  capped at 60 (both documented constants, matching the milestone's
  "documented cap" requirement) so it's never free and never unboundedly
  punishing. Abort/retreat is still blocked while camped - only the
  speed-up action is repurposed into this priced bypass.
- **Convoy exposure is real, and a genuine bug was found and fixed while
  building it.** `processSupplyConvoys` previously deducted the committed
  Hauler+Tanker at dispatch (`HandleDispatchConvoy`) but never returned
  them on ANY outcome - every resupply run permanently drained 1 Hauler +
  1 Tanker regardless of success. Fixed: transports now return to the home
  garrison on a successful delivery and on a "missed contact" failure
  (target moved on before the convoy arrived - the crews still make it
  back, only the cargo is wasted). A new 8% flat ambush roll
  (`convoyAmbushChance`) gives convoys real "exposure" as the milestone
  describes: on an ambush, the convoy is lost outright - cargo AND
  transports - and the stranded column stays stranded.
- **Agent electricity upkeep fixed per Asiwaju's direct instruction**
  (`internal/engine/agent/agent.go`): base upkeep 2.0 -> 5.0/tick, floor
  multiplier 0.10 -> 0.40 (so the floor a fully-teched veteran outpost
  reaches is now 2.0/tick, not 0.2/tick). This resolves the open design
  question logged in the "Known design assumptions" section above.
- **New Crystal Exchange feature** (`internal/bot/handlers/
  crystal_exchange.go`, reachable from the Trade Hub panel): converts
  Crystal into a large quantity of any other resource. Rates scale with
  how commonly-needed the target resource is - Scrap/Metal/Rations (burned
  constantly) convert generously (12,000 / 10,000 / 8,000 per Crystal),
  while already-scarce currencies like Neuro-Cores and Ether convert
  conservatively (500 / 100 per Crystal), so Crystal can't be laundered
  into a second rare currency at a generous rate. Presets of 1/5/25
  Crystal avoid needing free-text quantity input; storage-cap clamping
  reuses the existing `storagecap` package so a conversion can't silently
  overflow a warehouse.

## Completed implementation detail: persistent AI civilizations (Phase 6, foundational tier)

- **Design choice: an AI faction is a REAL `encampments` row, not a new
  entity type.** A synthetic `users` row (negative `telegram_id`, e.g.
  `-900001`, `state = 'ai_faction'`) satisfies the existing `NOT NULL
  UNIQUE` FK on `encampments.user_id` without altering that constraint or
  auditing every join that assumes a real player. The payoff: discovery
  (`resolveExplorationDiscovery` already queries "any encampment in this
  continent, undiscovered by me" with no filter that would exclude one),
  the raid target board, `resolveRaidCombats`, loot, and battle reports
  all work on an AI faction identically to a real player's base, with
  **zero code changes** in any of that pipeline. This is the single most
  important integration decision in this pass, and the reason milestones
  1-2 and most of the exit criteria were achievable in one session instead
  of requiring a parallel AI-specific combat/discovery system.
- **Seeded factions:** 8 total, 2 per continent (a stronger level-6 and a
  weaker level-3 per region), each with a distinct name/flavor, a real
  coordinate placed within that continent's existing quadrant convention
  (same ranges `relocateZeroCoordinates` uses), starting resources scaled
  to level, and a starting garrison (`workshop_inventory.soldiers`/
  `mechs`) so it's a real raid target from the moment it's discovered, not
  an empty shell. Seeding is idempotent via `ai_faction_key` and runs once
  at startup (`seedAICivilizations`, called right after
  `relocateZeroCoordinates`) - never re-seeds existing factions.
- **`growAICivilizations`** (new tick pass): each AI faction gains a small
  per-tick trickle of Scrap/Metal/Rations/Electricity (storage-cap
  clamped, same as a player) and a much smaller Crystal trickle, plus a 5%
  per-tick chance to add one Soldier, then one Mech once Soldiers hit
  `level * 25`, then level up (raising both ceilings) once Mechs hit
  `level * 4` - capped at level 15 so an ignored faction becomes a
  meaningfully tougher target over time without becoming unraidable. A
  faction that gets raided regularly stays suppressed; one left alone
  grows - this is the intended emergent difficulty curve, not a bug.
- **Safety audit, not skipped:** seeding synthetic `users` rows risks
  leaking them into "total player count" stats or an admin broadcast.
  Audited and fixed: `admin.go` (two player-count queries + the broadcast
  target list), `profile.go`, and `world.go`'s survivor counts now all
  exclude `state = 'ai_faction'`; `ranking.go`'s two global leaderboards
  now filter `is_ai_faction = FALSE`. `profile.go`'s referral-count query
  was checked and left as-is (a real player's `referred_by` can never
  point to an AI faction, so no filter was needed there).
- **Deliberately NOT done - the larger remaining half of Phase 6:** an AI
  faction today gathers, builds, and expands, but never *decides* to do
  anything - there is no AI tick-intent loop that picks research, launches
  scouting, dispatches a raid, or reinforces. Building real AI-initiated
  raids (an AI faction assessing its own force, picking a target through
  the same discovery gate a player uses, and launching a genuine `raids`
  row with itself as `attacker_id`) is what would let AI factions actually
  appear on the road via the already-built Phase 3/4 road-encounter
  system - that's the natural, concrete next increment, and it's
  substantially larger than this pass (it needs its own decision logic,
  not just more tick-pass math). Rogue Drone Nest was left completely
  untouched throughout, per milestone 5's explicit "no destructive
  conversion" instruction - it remains the fallback for a continent with
  no other outpost.

