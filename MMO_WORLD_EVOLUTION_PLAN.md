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
| 2026-07-20 | 1 | complete and hardened | Existing `027_mmo_warfare_logistics_phase1.sql` supplies the foundation. `029_mmo_transport_staging_and_force_recovery.sql` closes the false home-inventory transport check and returns all tracked support units after a campaign. |
| 2026-07-20 | 2 | complete | `028_mmo_world_discovery_and_radar.sql` adds directional discoveries and route snapshots. Exploration can discover rival outposts or the Rogue Nest, Scouts affect discovery odds, targets are filtered and launch-authorized by discovery, route proximity creates reciprocal knowledge, and radar warnings are sent once at capability-dependent proximity. |
| 2026-07-20 | 3-7 | pending | Road response windows/field battles, temporary camps and convoy reinforcement, persistent AI civilizations, and observability/balance tooling remain to be implemented. |

## Known design assumptions and edge cases

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
- Loot comes from carried cargo in a field battle, never directly from a
  remote home base.
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
