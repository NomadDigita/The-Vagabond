# ADR 0146: Decisions from `internal/engine/tick/engine.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/engine.go` and moved here so the source no longer carries them. Each entry is the original comment.

## defaultTickInterval mirrors cmd/bot/main.go's own fallback so the two

Removed from `internal/engine/tick/engine.go:47`.

~~~~text
defaultTickInterval mirrors cmd/bot/main.go's own fallback so the two
stay in sync; duplicated here (rather than imported) to keep this
package free of a cmd/bot dependency.
~~~~


## Guards against a startup crash: time.NewTicker panics on a

Removed from `internal/engine/tick/engine.go:54`.

~~~~text
Guards against a startup crash: time.NewTicker panics on a
non-positive duration, which previously meant a typo'd or
accidentally-zeroed GAME_TICK_SECONDS (e.g. "0") would take
down the entire bot process at boot instead of degrading
gracefully. Confirmed bug (2026-08-02): nothing validated
this value between os.Getenv/strconv.Atoi in cmd/bot/main.go
and the NewTicker call in Start() below.
~~~~


## Third layer of defense-in-depth (runPhase and ProcessTick each

Removed from `internal/engine/tick/engine.go:82`.

~~~~text
Third layer of defense-in-depth (runPhase and ProcessTick each
have their own recover() too): if a panic ever still reaches
here, log it and keep the loop alive for the *next* tick
instead of letting the goroutine die and silently stopping the
game clock forever. Wrapped per-iteration (not once for the
whole loop) so recovering doesn't exit the for/select loop.
~~~~


## LastTickStatus reports when the most recent tick pass started and

Removed from `internal/engine/tick/engine.go:112`.

~~~~text
LastTickStatus reports when the most recent tick pass started and
finished, so admin-facing panels (see internal/bot/handlers/admin.go's
"server_metrics" action) can show at a glance whether the background
tick engine - and therefore the weather/world-event system, which
runs as its first phase - is actually alive in the current
deployment, instead of players having to infer it indirectly from
whether events have ever fired.
~~~~


## tickPhase describes one isolated unit of tick work. Each phase gets its

Removed from `internal/engine/tick/engine.go:125`.

~~~~text
tickPhase describes one isolated unit of tick work. Each phase gets its
own transaction so that a failure in one subsystem (e.g. arena matchmaking)
can never block or roll back unrelated subsystems (e.g. espionage,
mining, combat resolution) in the same tick.
~~~~


## runPhase executes a single phase in its own transaction. It always

Removed from `internal/engine/tick/engine.go:134`.

~~~~text
runPhase executes a single phase in its own transaction. It always
returns (never aborts the caller's loop) so subsequent phases still run
even if this one fails. Errors are logged with the phase name attached
for easier diagnosis.
~~~~


## Panic recovery here is critical: previously an unrecovered panic in

Removed from `internal/engine/tick/engine.go:139`.

~~~~text
Panic recovery here is critical: previously an unrecovered panic in
any single phase (e.g. a nil pointer deref while resolving one bad
raid row) would propagate all the way up through ProcessTick and
out of the tick engine's background goroutine in Start(), silently
killing that goroutine forever. Since nothing restarts it, every
subsequent tick simply never ran again - which is why players saw
raids permanently stuck showing "(0s remaining)": the countdown
target time had passed, but the tick pass that resolves/advances
raids had stopped running entirely, so nothing ever cleared the
stale state. Confirmed root cause 2026-08-03. Recovering here keeps
one bad phase from taking down the whole engine, matching the
isolation runPhase already provides via per-phase transactions.
~~~~


## collectDailyTax implements SpaceHunt's Daily Tax Law: once every 24

Removed from `internal/engine/tick/engine.go:174`.

~~~~text
collectDailyTax implements SpaceHunt's Daily Tax Law: once every 24
hours, every player pays tax_rate_percent of their banked Dollars into a
shared pool, which is then paid out evenly to the top 3 ranked
survivors (by the canonical scoring.ScoreExpr).
~~~~


## resolveExplorationDiscovery turns a completed site expedition into a

Removed from `internal/engine/tick/engine.go:326`.

~~~~text
resolveExplorationDiscovery turns a completed site expedition into a
possible first-contact event. It deliberately selects only a target the
explorer does not already know, so retrying a tick cannot manufacture
duplicate notifications or reset intelligence timestamps.
~~~~


## RARE_WORLD_FEATURES_PLAN.md Phase 3.5: stumbling onto a

Removed from `internal/engine/tick/engine.go:363`.

~~~~text
RARE_WORLD_FEATURES_PLAN.md Phase 3.5: stumbling onto a
permanently-defeated AI faction's old territory is its
own small narrative moment - the epitaph generated when
it fell (see factiondefeat.go's checkForFactionDefeat)
gets appended here instead of the ordinary "now
available in the Tactical Target Matrix" framing, since
there's no target left to raid.
~~~~


## A new world can have no other player outpost in its sector. In that

Removed from `internal/engine/tick/engine.go:385`.

~~~~text
A new world can have no other player outpost in its sector. In that
case an expedition may instead triangulate the existing Rogue Nest;
it is still hidden until this explicit discovery occurs.
~~~~


## autoScanSweep refreshes intelligence on one already-discovered rival.

Removed from `internal/engine/tick/engine.go:400`.

~~~~text
autoScanSweep refreshes intelligence on one already-discovered rival.
Automatic scanning must never reveal an undiscovered player: exploration
and route sightings remain the only ways to create first contact.
~~~~


## resolveWorldBossAttacks handles both stages of a real World Boss

Removed from `internal/engine/tick/engine.go:457`.

~~~~text
resolveWorldBossAttacks handles both stages of a real World Boss
engagement: marching parties arriving deal damage AND take real
retaliation casualties (the boss fights back), then survivors begin an
actual return march instead of the old instant-resolve model. On a
killing blow, loot is split by cumulative damage share exactly as
before, just deferred until the engagement actually happens.
~~~~


## Someone else already killed it between launch and arrival -

Removed from `internal/engine/tick/engine.go:498`.

~~~~text
Someone else already killed it between launch and arrival -
no fight happens, survivors just turn back immediately.
~~~~


## Real retaliation: the boss hits back. Danger scales per boss

Removed from `internal/engine/tick/engine.go:519`.

~~~~text
Real retaliation: the boss hits back. Danger scales per boss
(retaliation_rating), and mechs are tankier than soldiers in the
losses split (mirroring the standard 70/30 raid pattern).

NEXT_GENERATION_FEATURES_PLAN.md Phase 8: a Gauntlet boss
that's already below its enrage_threshold_pct BEFORE this
engagement's damage lands (checked against m.bossCurHP, the
HP it had on arrival - not newHP, so an attack that itself
crosses the threshold doesn't retroactively enrage against
the attacker who caused it) hits back at double retaliation -
the real risk that makes Gauntlet's guaranteed Epic/Legendary
drop (see payoutWorldBossLoot) feel earned rather than free.
~~~~


## Survivors begin the real return march - same duration as the

Removed from `internal/engine/tick/engine.go:588`.

~~~~text
Survivors begin the real return march - same duration as the
trip out, exactly like a real raid, not an instant reset.
~~~~


## payoutWorldBossLoot splits a defeated boss's loot pool proportional to

Removed from `internal/engine/tick/engine.go:638`.

~~~~text
payoutWorldBossLoot splits a defeated boss's loot pool proportional to
each contributor's total damage dealt, then resets the boss to full HP
with a 10% larger pool for long-term grinding incentive.
~~~~


## NEXT_GENERATION_FEATURES_PLAN.md Phase 3: World Bosses are

Removed from `internal/engine/tick/engine.go:674`.

~~~~text
NEXT_GENERATION_FEATURES_PLAN.md Phase 3: World Bosses are
one of Epic/Legendary gear's only two sources (the other
is Relic Convoy claims - relicconvoy.go). Every
contributor gets an independent roll, not just the top
damage dealer, since the loot pool itself already
rewards top damage proportionally - gear drops are a
separate, flatter "you showed up and fought" bonus. Epic
is meaningfully more common than Legendary (8% vs 2%),
matching this project's established "rare means rare"
principle for its best rewards.

NEXT_GENERATION_FEATURES_PLAN.md Phase 8: a Gauntlet
kill instead GUARANTEES a drop for every contributor
(70% Epic / 30% Legendary) rather than the normal
mostly-nothing roll - the real, felt payoff for having
survived that boss's doubled enrage retaliation above.
~~~~


## applyClanWarScore checks whether the attacker and defender encampments

Removed from `internal/engine/tick/engine.go:718`.

~~~~text
applyClanWarScore checks whether the attacker and defender encampments
belong to two clans currently at war with each other, and if so, adds
the raid's loot value to the attacker's clan war score. This is what
makes Clan Wars a real, live-scored mechanic instead of a one-time
notification with no lasting effect.
~~~~


## Dollars, split evenly among the winning clan's members

Removed from `internal/engine/tick/engine.go:776`.

~~~~text
Dollars, split evenly among the winning clan's members
~~~~


## Outer safety net in addition to runPhase's own per-phase recover().

Removed from `internal/engine/tick/engine.go:864`.

~~~~text
Outer safety net in addition to runPhase's own per-phase recover().
Anything in ProcessTick that runs outside a phase (e.g. the idle/
auto-scan cadence checks below, or a future addition) is still
inside the same background goroutine started in Start(); an
unrecovered panic there would permanently stop all future ticks
the same way an unrecovered per-phase panic used to.
~~~~


## 20-Minute Idle Miner Sweeper Check. Kept isolated from the main phase

Removed from `internal/engine/tick/engine.go:936`.

~~~~text
20-Minute Idle Miner Sweeper Check. Kept isolated from the main phase
list since it's on its own cadence rather than every tick.
~~~~


## Force-complete any partial rallying transits into lead base

Removed from `internal/engine/tick/engine.go:1002`.

~~~~text
Force-complete any partial rallying transits into lead base
~~~~


## Populate the same origin/destination/region/leg-clock columns

Removed from `internal/engine/tick/engine.go:1025`.

~~~~text
Populate the same origin/destination/region/leg-clock columns
HandleConfirmHangarLaunchCallback sets on solo launches - without
these, evaluateRoadEncounters/evaluateRoadBaseEncounters (which
both filter on origin_x IS NOT NULL) can never see a co-op raid
launched through this staged-lobby path, so it silently skips
Phase 3/4/5's road-encounter/weather system entirely. See
BUGS_AND_INCONSISTENCIES.md for the full writeup.
~~~~


## Simulated return flight time

Removed from `internal/engine/tick/engine.go:1208`.

~~~~text
Simulated return flight time
~~~~


## Frozen by a road encounter, a weather camp, or already

Removed from `internal/engine/tick/engine.go:1334`.

~~~~text
Frozen by a road encounter, a weather camp, or already
awaiting reinforcement - the column has halted, so it isn't
burning marching supplies (or re-triggering this same
depletion check) until it resumes.
~~~~


## AI factions can never manage either of the two consumption

Removed from `internal/engine/tick/engine.go:1341`.

~~~~text
AI factions can never manage either of the two consumption
pools below - HandleDispatchConvoy and the "abort" branch of
HandleExpeditionActions both require a real Telegram sender
whose encampment matches raids.attacker_id (see combat.go /
combat_road_encounters.go), and nothing in aidecisions.go ever
tops off an AI faction's home Metal or orders it a retreat.
Previously this meant EVERY AI-attacker column that outran
either pool was doomed one of two ways: halted forever at
movement_state = 'awaiting_reinforcement' (now mitigated by
resolveStalledColumns, but still a slow multi-cycle crawl), or
- worse, and untouched by that watchdog since movement_state
never changes - its resolve_time getting pushed 3 minutes
further into the future on every ~10s tick once home Metal hit
zero, a runaway loop where "arrival" recedes faster than real
time passes and the raid provably never completes. Real
players get a genuine logistics minigame (that's the point);
AI factions were just never eligible to play it, so exempt
them entirely instead of leaving a fight they can't win.
~~~~


## Bugfix: this used to check newMetalFuel <= 0 (current state),

Removed from `internal/engine/tick/engine.go:1379`.

~~~~text
Bugfix: this used to check newMetalFuel <= 0 (current state),
which re-fired every single tick for as long as home Metal
stayed at zero - stacking another 3-minute delay on top of the
last one every ~10s. That's a runaway loop: resolve_time gets
pushed into the future faster than real time elapses, so a
column whose home base ran out of Metal and stayed there
(trivially easy - Metal has plenty of other draws) could never
arrive, full stop, with nothing on the radar or Expedition
Control panel explaining why. Checking the old->new transition
instead means the one-time "fuel shortage" penalty applies
once per shortage, not once per tick of an ongoing shortage.
~~~~


## Raid-scoped supply pools (attacker_rations/attacker_ammo on the

Removed from `internal/engine/tick/engine.go:1395`.

~~~~text
Raid-scoped supply pools (attacker_rations/attacker_ammo on the
raid itself, distinct from the home base's stockpile above) -
this is what actually reflects "the army in the field is running
out of what it packed for the march", and previously ammo was a
dead stub that never moved off its 100.0 default.

Bugfix: this used to be a flat -4/-4/-3/-2 per tick regardless
of how long the march actually takes. At the ~10s real-world
tick interval this game actually runs at, rations/ammo hit
zero in ~4 minutes no matter what - so any march longer than
that (the overwhelming majority: raids routinely quote 15m to
multi-day arrivals) was *guaranteed* to halt at
awaiting_reinforcement long before arriving, not as an
occasional consequence of a long campaign but as the default
outcome of marching at all. Scaling the drain to the column's
own base_march_minutes means a normal, uninterrupted march
keeps a real safety margin (supplies theoretically stretch to
~140-220% of the march's expected duration) instead of
draining on a clock that has nothing to do with the trip's
actual length - while a march that gets meaningfully slowed
down by road encounters or weather can still genuinely run dry,
which is the intended risk, not a guaranteed one.
~~~~


## Threshold-crossing notifications only (not every tick), so the

Removed from `internal/engine/tick/engine.go:1438`.

~~~~text
Threshold-crossing notifications only (not every tick), so the
player gets one heads-up at 25% and one at empty per resource,
instead of spam every 3 seconds.
~~~~


## Phase 5 milestone 4: the two failure modes are NOT the same.

Removed from `internal/engine/tick/engine.go:1486`.

~~~~text
Phase 5 milestone 4: the two failure modes are NOT the same.
Rations/ammo depletion means the column has nothing to fight or
march with - it halts outright, same as before. Electricity/
logistics depletion is softer: "high tech" (mech bonuses,
capital-unit tech multipliers - see roadcombat.Power's
HighTechOffline handling and the matching check in
resolveRaidCombats below) goes offline immediately, but the
column keeps moving on muscle and diesel alone. Only if power
stays out for a sustained period (grace window) does it
escalate to a full halt, matching "disables high-tech
contributions before it can force a pause."
~~~~


## Power recovered (e.g. after a convoy delivery ticked

Removed from `internal/engine/tick/engine.go:1524`.

~~~~text
Power recovered (e.g. after a convoy delivery ticked
electricity/logistics back above zero) - clear the grace
counter and restore high-tech capability.
~~~~


## BUGFIX: powerRating was declared but never computed or

Removed from `internal/engine/tick/engine.go:1564`.

~~~~text
BUGFIX: powerRating was declared but never computed or
used anywhere in this function - every arena match
was actually being decided by pure queue order (first
half of the FIFO-matched group always "won", the rest
always "lost"), completely ignoring the soldiers/mechs
this query goes to the trouble of fetching. Weighting
mirrors the wider game's general soldier:mech power
ratio (mechs are the far more expensive, stronger unit).
~~~~


## BUGFIX: this used to be `winners := matched[:requiredCount/2]`

Removed from `internal/engine/tick/engine.go:1582`.

~~~~text
BUGFIX: this used to be `winners := matched[:requiredCount/2]`
/ `losers := matched[requiredCount/2:]` - i.e. whoever queued
first always won, every single match, regardless of army
strength. Now: shuffle into two random (not queue-order)
teams of equal size, total each team's powerRating, and
roll a power-weighted coin flip - stronger teams are more
likely to win, but it's not a guaranteed stomp, matching
the RNG-with-a-thumb-on-the-scale pattern used by the main
raid combat resolution elsewhere in this file.
~~~~


## Both teams get a small combat-power floor so a 0-army team

Removed from `internal/engine/tick/engine.go:1606`.

~~~~text
Both teams get a small combat-power floor so a 0-army team
(e.g. a fresh outpost with no troops yet) doesn't create a
division-by-zero and always has *some* chance, however slim.
~~~~


## routeProgressAt uses a frozen progress snapshot plus elapsed active travel

Removed from `internal/engine/tick/engine.go:1777`.

~~~~text
routeProgressAt uses a frozen progress snapshot plus elapsed active travel
time.  Pauses never alter the snapshot, so adding time for an encounter,
weather, or recovery cannot move a column backwards on the map.
~~~~


## Pre-migration campaigns have no snapshot. Preserve their previous ETA

Removed from `internal/engine/tick/engine.go:1793`.

~~~~text
Pre-migration campaigns have no snapshot. Preserve their previous ETA
interpretation until their next route transition writes one.
~~~~


## resumeRoadRaids recalculates ETA from the route snapshot on a mutually

Removed from `internal/engine/tick/engine.go:1823`.

~~~~text
resumeRoadRaids recalculates ETA from the route snapshot on a mutually
continued or expired encounter. It is deliberately conditional so a stale
resolver cannot resume a force claimed by another active encounter.
~~~~


## Being seen is reciprocal knowledge. The contact has not been

Removed from `internal/engine/tick/engine.go:1945`.

~~~~text
Being seen is reciprocal knowledge. The contact has not been
attacked yet, but knows a foreign force crossed its road.
~~~~


## roadMover is a marching/returning expedition eligible for a road

Removed from `internal/engine/tick/engine.go:1961`.

~~~~text
roadMover is a marching/returning expedition eligible for a road
encounter roll: it has a route snapshot, is not already frozen by a
pending encounter, and its current position is derived from the stable
leg clock rather than a resolve_time estimate.
~~~~


## current region (origin before the leg's midpoint, destination after)

Removed from `internal/engine/tick/engine.go:1972`.

~~~~text
current region (origin before the leg's midpoint, destination after)
~~~~


## evaluateRoadEncounters implements MMO_WORLD_EVOLUTION_PLAN.md Phase 4:

Removed from `internal/engine/tick/engine.go:1975`.

~~~~text
evaluateRoadEncounters implements MMO_WORLD_EVOLUTION_PLAN.md Phase 4:
two moving expeditions that converge in world space have a chance to
notice each other. Detection always establishes reciprocal discovery
(matches the request: "while crossing countries Player A encounters
Player C... notifications should immediately be sent to both"); an actual
encounter roll then decides whether it becomes a live Attack/Continue
decision window. Only one encounter per raid may be active at a time.
~~~~


## same commander can't ambush their own second column

Removed from `internal/engine/tick/engine.go:2023`.

~~~~text
same commander can't ambush their own second column
~~~~


## Cooldown: skip a pair that just resolved an encounter with

Removed from `internal/engine/tick/engine.go:2035`.

~~~~text
Cooldown: skip a pair that just resolved an encounter with
each other recently, so a "Continue" choice isn't
immediately re-asked next tick.
~~~~


## another tick worker already created this pending pair

Removed from `internal/engine/tick/engine.go:2064`.

~~~~text
another tick worker already created this pending pair
~~~~


## Freeze both columns: no arrival/engage/return processing and

Removed from `internal/engine/tick/engine.go:2069`.

~~~~text
Freeze both columns: no arrival/engage/return processing and
no route-progress advance until the encounter resolves.
paused_at anchors the effective "now" used by every route
position calculation so a frozen army does not silently
keep advancing while its commander is deciding.
~~~~


## The encounter itself is a discovery event even if nobody

Removed from `internal/engine/tick/engine.go:2078`.

~~~~text
The encounter itself is a discovery event even if nobody
chooses to fight - matches the request's Nigeria/Ghana/
America example directly.
~~~~


## expireRoadEncounters resolves any pending encounter whose response window

Removed from `internal/engine/tick/engine.go:2105`.

~~~~text
expireRoadEncounters resolves any pending encounter whose response window
has lapsed as a peaceful pass (no attack decision was made in time), and
unfreezes both columns from exactly the position they paused at.
~~~~


## resolveEncounterAsContinue is the shared "both sides pass" resolution

Removed from `internal/engine/tick/engine.go:2132`.

~~~~text
resolveEncounterAsContinue is the shared "both sides pass" resolution
used by both an explicit mutual Continue and a response-window timeout.
It unfreezes each raid by shifting its leg_started_at forward by exactly
the pause duration, so route progress resumes from where it paused
instead of jumping ahead (the pause never counted as travel time) or
snapping back (the position itself never moved while paused).
~~~~


## evaluateRouteWeatherIncidents implements MMO_WORLD_EVOLUTION_PLAN.md

Removed from `internal/engine/tick/engine.go:2172`.

~~~~text
evaluateRouteWeatherIncidents implements MMO_WORLD_EVOLUTION_PLAN.md
Phase 5's local weather layer: a marching/returning column can be forced
into a temporary camp by a flood, storm, or heatwave. This deliberately
reads the EXISTING continent-wide world_events table (internal/engine/
world.WeatherEngine) as an input signal rather than rolling a second,
disconnected weather system - an active acid_rain/radiation_storm/emp/
sandstorm event over the continent a column is currently crossing raises
the odds of a matching local incident there.
~~~~


## All six local incident types the request asked for. flood/storm/

Removed from `internal/engine/tick/engine.go:2218`.

~~~~text
All six local incident types the request asked for. flood/storm/
heatwave existed from the first Phase 5 pass; sandstorm/emp/
radiation are the direct-mapped counterparts of the matching
continent-wide world_events, each with its own onset effect below
(routeIncidentOnsetEffect) rather than just a different label on the
same generic pause.
~~~~


## directLocalName covers the continent-wide events whose

Removed from `internal/engine/tick/engine.go:2243`.

~~~~text
directLocalName covers the continent-wide events whose
local incident counterpart isn't just the same string
(radiation_storm -> "radiation"); everything else that has
a direct local incident of the same name (sandstorm, emp)
is handled by the identity check below.
~~~~


## already has an active incident (race with another tick worker)

Removed from `internal/engine/tick/engine.go:2287`.

~~~~text
already has an active incident (race with another tick worker)
~~~~


## applyRouteIncidentOnsetEffect gives each of the six local incident types

Removed from `internal/engine/tick/engine.go:2313`.

~~~~text
applyRouteIncidentOnsetEffect gives each of the six local incident types
real mechanical teeth instead of an identical pause with a different
label, and directly implements the original request's "units may die,
starvation may occur" for weather (as opposed to combat). Effects fire
once, at the moment the incident begins camping the column.
~~~~


## Standing water spoils dry stores.

Removed from `internal/engine/tick/engine.go:2329`.

~~~~text
Standing water spoils dry stores.
~~~~


## Lightning/downed lines damage the column's field generators.

Removed from `internal/engine/tick/engine.go:2336`.

~~~~text
Lightning/downed lines damage the column's field generators.
~~~~


## Heat and dehydration burn through rations faster than planned.

Removed from `internal/engine/tick/engine.go:2343`.

~~~~text
Heat and dehydration burn through rations faster than planned.
~~~~


## Fine particulates foul engines and logistics equipment.

Removed from `internal/engine/tick/engine.go:2350`.

~~~~text
Fine particulates foul engines and logistics equipment.
~~~~


## A surge knocks out a large share of remaining electrical/

Removed from `internal/engine/tick/engine.go:2357`.

~~~~text
A surge knocks out a large share of remaining electrical/
logistics capacity outright - the signature EMP effect.
~~~~


## Exposure causes real casualties - "units may die" from weather,

Removed from `internal/engine/tick/engine.go:2365`.

~~~~text
Exposure causes real casualties - "units may die" from weather,
not just combat. Reuses the same roadcombat casualty machinery
as a road battle, at a much lower fraction.
~~~~


## loadRoadFieldForceForIncident is the tick-engine-side twin of

Removed from `internal/engine/tick/engine.go:2389`.

~~~~text
loadRoadFieldForceForIncident is the tick-engine-side twin of
internal/bot/handlers/combat.go's loadRoadFieldForce - duplicated at
this small size because the two live in different packages with
different transaction/handler contexts, exactly like
resolveEncounterAsContinue/resolveRoadEncounterContinue above.
~~~~


## routeIncidentPresentation gives each local incident type a matching

Removed from `internal/engine/tick/engine.go:2405`.

~~~~text
routeIncidentPresentation gives each local incident type a matching
emoji/headline, distinct from the continent-wide event headlines in
internal/engine/world/weather.go (these are personal to one column, not
a regional news broadcast).
~~~~


## clearRouteWeatherIncidents resolves any route incident whose cleared_at

Removed from `internal/engine/tick/engine.go:2428`.

~~~~text
clearRouteWeatherIncidents resolves any route incident whose cleared_at
has passed, resuming the column from exactly where it paused - the same
leg_started_at/resolve_time shift used to resume from a road encounter,
since both are the same underlying "something external paused this
column" mechanism.
~~~~


## The convoy itself was ambushed en route - its own

Removed from `internal/engine/tick/engine.go:2531`.

~~~~text
The convoy itself was ambushed en route - its own
unmodeled "exposure." Unlike a missed-contact failure,
this is a real loss: cargo AND the committed Hauler/Tanker
crews are gone, and the stranded column is still stranded.
~~~~


## stalledColumnGraceAIFaction / stalledColumnGraceRealPlayer bound how long

Removed from `internal/engine/tick/engine.go:2579`.

~~~~text
stalledColumnGraceAIFaction / stalledColumnGraceRealPlayer bound how long
a column may sit halted at movement_state = 'awaiting_reinforcement'
before resolveStalledColumns steps in. AI factions can NEVER dispatch a
convoy or order a retreat themselves (HandleDispatchConvoy and
HandleExpeditionActions both require a real Telegram sender whose
encampment matches raids.attacker_id - see combat.go / combat_road_
encounters.go), so without this watchdog any AI-attacker column that
outruns its own supplies halts permanently: resolveRaidCombats only
picks up rows with movement_state = 'moving', so the raid's resolve_time
- and therefore its "INBOUND INVASION WARNING ... (0s remaining)" radar
entry on the defender's side - never advances or expires again. A real
player's own column can ALSO be stranded indefinitely if they lack the
Hauler+Tanker or Scrap/Metal a convoy costs and don't notice the alert,
so the same watchdog applies there too, just with a much longer grace
window that respects the player's intended choice between "pay for a
convoy" and "eat the 20% scrap penalty and retreat" before acting for
them. This was a genuinely separate bug from the tick-engine panic fixed
in ADR "Fix raids stuck at (0s remaining)" - that fix stopped a dead
background goroutine from freezing ALL ticks; this fixes columns that
are correctly excluded, tick after tick, by design, with nothing ever
changing their excluded state. See BUGS_AND_INCONSISTENCIES.md.
~~~~


## stalledColumnMinResupplyPct is the (partial, "just enough to move")

Removed from `internal/engine/tick/engine.go:2603`.

~~~~text
stalledColumnMinResupplyPct is the (partial, "just enough to move")
emergency resupply granted when the watchdog fires - deliberately far
below the 100% a real dispatched convoy delivers, so this reads as a
last-resort bridge rather than a free substitute for the real mechanic.
~~~~


## resolveStalledColumns is the watchdog described above: it finds every

Removed from `internal/engine/tick/engine.go:2609`.

~~~~text
resolveStalledColumns is the watchdog described above: it finds every
column that has been sitting at movement_state = 'awaiting_reinforcement'
longer than its grace window, tops its field supplies up to a bare
minimum (mirroring the "delivered" branch of processSupplyConvoys - same
clock-shift approach, so leg progress and resolve_time stay coherent),
and lets it resume marching or returning on its own. A column with an
active resupply convoy already en route is left alone; that convoy will
resolve it first (delivered, ambushed, or missed contact) well within
either grace window under normal travel times.
~~~~


## aiCivilizationMaxSoldiersPerLevel / aiCivilizationMaxMechsPerLevel cap how

Removed from `internal/engine/tick/engine.go:2699`.

~~~~text
aiCivilizationMaxSoldiersPerLevel / aiCivilizationMaxMechsPerLevel cap how
large an AI faction's garrison can grow relative to its level, so an
undisturbed AI base becomes a genuinely tougher (and more rewarding)
target over time without growing without bound.
~~~~


## aiResearchTechColumns duplicates the seven research_states column names

Removed from `internal/engine/tick/engine.go:2706`.

~~~~text
aiResearchTechColumns duplicates the seven research_states column names
from researchTree in internal/bot/handlers/research.go (key/emoji/title
text isn't needed here, just the columns an AI faction can advance) -
the tick engine deliberately doesn't import bot/handlers, same reasoning
as every other duplicated literal in growAICivilizations.
~~~~


## aiResearchMaxLevel and aiResearchCostPerLevel duplicate MaxResearchLevel

Removed from `internal/engine/tick/engine.go:2716`.

~~~~text
aiResearchMaxLevel and aiResearchCostPerLevel duplicate MaxResearchLevel
and researchCost from internal/bot/handlers/research.go.
~~~~


## aiUpgradeableModules duplicates the non-core module type strings from

Removed from `internal/engine/tick/engine.go:2721`.

~~~~text
aiUpgradeableModules duplicates the non-core module type strings from
camp.go's structural/defense-grid/infrastructure panels (tent/heap/
generator plus defenseGridModules and infrastructureModules) -
deliberately excludes "camp_core", since that already grows via the
garrison section below once a faction maxes its soldier/mech caps.
~~~~


## aiModuleUpgradeCostPerLevel duplicates the Scrap cost-per-level used by

Removed from `internal/engine/tick/engine.go:2733`.

~~~~text
aiModuleUpgradeCostPerLevel duplicates the Scrap cost-per-level used by
HandleStructuralUpgrades/HandleDefenseGridPanel/HandleInfrastructureGridPanel
(currentLvl*150) in internal/bot/handlers/camp.go.
~~~~


## aiExplorationDispatchRationsCost/aiExplorationDispatchMetalCost and

Removed from `internal/engine/tick/engine.go:2738`.

~~~~text
aiExplorationDispatchRationsCost/aiExplorationDispatchMetalCost and
aiExplorationMinTravelMinutes/aiExplorationMaxTravelMinutes duplicate the
same-named constants in internal/bot/handlers/exploration.go.
~~~~


## aiExplorationSiteTemplate and aiExplorationTemplates duplicate

Removed from `internal/engine/tick/engine.go:2746`.

~~~~text
aiExplorationSiteTemplate and aiExplorationTemplates duplicate
explorationSiteTemplate/explorationTemplates from
internal/bot/handlers/exploration.go - same weighted reward pool a
human's personal expedition rolls from, so an AI faction's exploration
haul feels like the same game, not a simplified AI-only version.
~~~~


## aiRollExplorationTemplate duplicates rollExplorationTemplate's exact

Removed from `internal/engine/tick/engine.go:2772`.

~~~~text
aiRollExplorationTemplate duplicates rollExplorationTemplate's exact
weighted-pick logic from internal/bot/handlers/exploration.go.
~~~~


## economicCollapseWarningThreshold gates the "ECONOMIC COLLAPSE" warning

Removed from `internal/engine/tick/engine.go:2789`.

~~~~text
economicCollapseWarningThreshold gates the "ECONOMIC COLLAPSE" warning
added per AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 1.5 - a
combined post-raid resource total (Scrap+Metal+Crystal+Rations+
Electricity+Dollars) below this is treated as "critically depleted."
Deliberately a small absolute number rather than a fraction of some
baseline: a genuinely new player's starting stockpile is itself only in
the low hundreds, so this needs to stay well below that to mean
anything. Tunable starting constant, not a precisely-right number - the
project owner should revisit alongside the raid loot percentages
themselves if new-player balance ever gets tuned.
~~~~


## aiClanLeaveChance is the per-tick probability (per AI faction

Removed from `internal/engine/tick/engine.go:2801`.

~~~~text
aiClanLeaveChance is the per-tick probability (per AI faction
currently in a clan) of leaving it - a Leader disbands the whole
clan, a rank-and-file member just leaves. See growAICivilizations's
own comment at the roll site for the full rationale. A var, not a
const, so aigovernance_test.go's pre-existing clan-war and
federation tests (which need their tracked clan to survive their
full tick loop) can zero it out for their duration rather than
risking the clan being randomly disbanded out from under them
mid-test.
~~~~


## economicCollapseWarning returns the appended report text when a

Removed from `internal/engine/tick/engine.go:2812`.

~~~~text
economicCollapseWarning returns the appended report text when a
defeated defender's post-raid resource total is critically low, or ""
otherwise. Pulled out as its own pure function specifically so this
section 1.5 addition has a unit-testable seam, independent of standing
up a full resolveRaidCombats integration test.
~~~~


## growAICivilizations implements MMO_WORLD_EVOLUTION_PLAN.md Phase 6's

Removed from `internal/engine/tick/engine.go:2844`.

~~~~text
growAICivilizations implements MMO_WORLD_EVOLUTION_PLAN.md Phase 6's
"gather resources, build units, expand" requirement for persistent AI
factions (seeded by seedAICivilizations in cmd/bot/main.go). Growth is
deliberately modest and capped per tick - an AI faction left alone for a
long time becomes a meaningfully richer, better-defended target, but a
player who raids it periodically can keep it suppressed, which is the
intended emergent dynamic rather than an ever-escalating unkillable base.
~~~~


## A trickle of Crystal too, at a much lower rate - it's the

Removed from `internal/engine/tick/engine.go:2891`.

~~~~text
A trickle of Crystal too, at a much lower rate - it's the
rarest resource in the world, an AI faction shouldn't
accumulate it quickly either.
~~~~


## Occasionally buy an available listing that isn't its own,

Removed from `internal/engine/tick/engine.go:2913`.

~~~~text
Occasionally buy an available listing that isn't its own,
exactly like a human would via /exchange's Buy button -
project owner follow-up direction 2026-08-01: selling alone
wasn't the full ask, "buy and sell" both. Mirrors
HandleBuyListingCallback's exact mechanics (dollar transfer,
resource transfer with storage-cap clamp, is_sold flag, a
seller notification for real human sellers) rather than a
simplified AI-only path, so a purchase behaves identically
regardless of who made it - same principle the listing code
below and launchAIRaid already follow. Re-checks is_sold
under a row lock immediately before committing the purchase,
same race-safety HandleBuyListingCallback's own FOR UPDATE
provides against a human buying the same listing at the same
moment.
~~~~


## Every column the exchange can ever list -

Removed from `internal/engine/tick/engine.go:2957`.

~~~~text
Every column the exchange can ever list -
see validMarketResources in exchange.go -
falls back to "metal" only for legacy rows
that predate an item_type this map doesn't
recognize, which shouldn't happen going
forward.
~~~~


## only notify real human sellers - other AI factions have negative telegram_id and no one re

Removed from `internal/engine/tick/engine.go:2978`.

~~~~text
only notify real human sellers - other AI factions have negative telegram_id and no one reads their notifications
~~~~


## Occasionally create or apply to join a clan, exactly like a

Removed from `internal/engine/tick/engine.go:2988`.

~~~~text
Occasionally create or apply to join a clan, exactly like a
human would via /clan_create or the "Apply" button on
/clans - project owner follow-up direction 2026-08-01:
"create a clan and join one" was named explicitly alongside
buying/selling. Mirrors HandleCreateClanCommand (free, just a
clans row + a Leader row in user_clans) and
HandleApplyToClanCallback (a pending clan_applications row
plus a notification - does NOT auto-join; a human clan
leader still has to accept it via the normal applications
inbox, exactly as if a human had applied) rather than
special-casing either path for AI. An AI faction's own
user_id (from seedAICivilizations/spawnNewAIFactions, always
negative) is what user_clans/clans key off, so no schema
change was needed - it's just another user_id as far as the
clan system is concerned.
~~~~


## Found a new clan, named after the faction itself

Removed from `internal/engine/tick/engine.go:3008`.

~~~~text
Found a new clan, named after the faction itself
so it reads naturally on /clans - ON CONFLICT
DO NOTHING via the name UNIQUE constraint simply
skips this cycle if that name's already taken,
same as a human hitting "Name Taken" and trying
again another time.
~~~~


## Occasionally leave (or, as Leader, disband) a clan, exactly

Removed from `internal/engine/tick/engine.go:3042`.

~~~~text
Occasionally leave (or, as Leader, disband) a clan, exactly
like a human via HandleLeaveClanCallback's "🚪 Leave
Alliance" button - the missing half of the create/join
behavior above. Without this, an AI faction that ever
created or joined a clan (via the block just above) stayed
there permanently for the rest of the world's life: it could
never re-roll into a different clan, never dissolve a clan
it founded that stopped making sense, and the "AI never
leaves/disbands/rejoins" gap the project owner flagged
2026-08-09 was exactly this - the join-side existed, the
leave-side never did. Mirrors HandleLeaveClanCallback's own
role-based branch precisely: a Leader leaving disbands the
whole clan (DELETE FROM clans CASCADEs to user_clans,
clan_applications, clan_diplomacy, clan_wars, and
constellation_members - the same cascade a human-triggered
disband already relies on), a rank-and-file member leaving
only removes their own membership row. Rolled independently
and considerably rarer than the 2% join roll (0.5%) so a
faction that just joined doesn't churn back out too quickly -
clan membership should still feel like a real, if not
permanent, commitment.
~~~~


## Notify real human members before the cascade wipes

Removed from `internal/engine/tick/engine.go:3067`.

~~~~text
Notify real human members before the cascade wipes
their user_clans row out from under them - an AI
Leader can preside over human members too (the
human /clans "Apply" flow doesn't filter out
AI-led clans the way this faction's own join-roll
above filters out AI-led ones for itself), so
losing their clan without warning would otherwise
be a silent, confusing loss for a real player.
~~~~


## only real humans have positive telegram_id

Removed from `internal/engine/tick/engine.go:3082`.

~~~~text
only real humans have positive telegram_id
~~~~


## Occasionally review pending applications to a clan this

Removed from `internal/engine/tick/engine.go:3100`.

~~~~text
Occasionally review pending applications to a clan this
faction leads, exactly like a human Leader via the
Applications Inbox (HandleApplicationDecisionCallback).
Related gap found alongside the leave/disband fix above: a
real player CAN apply to an AI-led clan (the human /clans
"Apply" flow doesn't filter by leader_id the way this
faction's own join-roll filters out AI-led clans for
itself), but with nothing ever reviewing clan_applications
for an AI Leader, that application just sat pending forever
- the applicant was never accepted, never rejected, and
never told. Accepts every pending applicant that still fits
within the 15-member cap (a simple, welcoming default - an
AI Leader has no reason to be selective the way a human
might be), exactly mirroring HandleApplicationDecisionCallback's
own capacity check and re-check for an applicant who joined
elsewhere in the meantime.
~~~~


## only notify real humans - another AI applicant has nothing reading its notifications

Removed from `internal/engine/tick/engine.go:3144`.

~~~~text
only notify real humans - another AI applicant has nothing reading its notifications
~~~~


## NEXT_GENERATION_FEATURES_PLAN.md Phase 3 AI-parity: a much

Removed from `internal/engine/tick/engine.go:3152`.

~~~~text
NEXT_GENERATION_FEATURES_PLAN.md Phase 3 AI-parity: a much
simpler crafting heuristic than the full player /gear UX -
an AI faction doesn't fight World Bosses or claim Relic
Convoys today (both real-player-only content), so its gear
ceiling is intentionally Common/Rare only. A small roll
crafts whatever Common/Rare item the faction can currently
afford in its best-value (highest ResourceBonusPct) affordable
slot and equips it immediately - AI factions don't have a
panel to visit later, so there's no separate "collect the
finished craft" step the way a real player's crafting queue
has; this rolls, crafts, and equips synchronously in one
tick rather than waiting out CraftMinutes, since an AI
faction gaining a small resource-generation edge a few
minutes later than a real player would is not a meaningful
balance concern the way skipping the wait for a *player*
would be.
~~~~


## Occasionally declare a clan war, exactly like a human Leader

Removed from `internal/engine/tick/engine.go:3195`.

~~~~text
Occasionally declare a clan war, exactly like a human Leader
would via the clan panel - project owner direction
2026-08-01: "every single thing a human can do." Mirrors
HandleDeclareClanWarCallback's exact mechanics (random enemy
clan not already at war, 48h duration, notifies all members
of both clans - real humans only, an AI member's negative
telegram_id has nothing reading its notifications). Only
fires if this faction actually leads a clan (role = 'Leader'
in user_clans - a rank-and-file AI member declaring a war
isn't a thing a human member could do either) and that clan
isn't already at war. War *scoring* itself needed no new
code at all - it's computed from the raids table by whoever
already resolves clan_wars, and an AI-launched raid via
launchAIRaid is already an ordinary raids row, so it already
counts once a member's clan is at war.
~~~~


## Occasionally found or join a federation, exactly like a

Removed from `internal/engine/tick/engine.go:3251`.

~~~~text
Occasionally found or join a federation, exactly like a
Clan Leader would via /fed_found or /fed_join. Mirrors
HandleFoundFederation (aiFederationFoundCost Crystal, same
figure as federationFoundCost in
internal/bot/handlers/federation.go - duplicated as a literal
rather than imported, since the tick engine deliberately
doesn't depend on the bot/handlers package, same reasoning
the exchange lot prices above were duplicated for) and
HandleJoinFederation (free, just points the clan at an
existing federation). Only fires for a Clan Leader whose clan
isn't already in a federation.
~~~~


## Occasionally engage in clan diplomacy, exactly like a human

Removed from `internal/engine/tick/engine.go:3291`.

~~~~text
Occasionally engage in clan diplomacy, exactly like a human
Clan Leader would via /ally, /nap, or the Accept/Reject
buttons on /diplomacy - project owner direction 2026-08-01:
"every single thing a human can do." Only fires for a
Leader-role AI faction, mirroring proposePact's and
HandleDiplomacyRespondCallback's "Leaders only" gate. Checks
for a pending proposal addressed to this faction's clan
first (an active Leader wouldn't leave one hanging forever),
then falls back to proposing a new pact to an unrelated clan.
aidecisions.go's pickFairAIRaidTarget/pickOverdueRaidTarget
already respect whatever pact results from this - the same
rule HasActivePact enforces for human-launched raids - so an
AI faction can't form a truce here and then ignore it there.
~~~~


## Accept most of the time - an AI Leader isn't

Removed from `internal/engine/tick/engine.go:3313`.

~~~~text
Accept most of the time - an AI Leader isn't
adversarial toward diplomacy by default.
~~~~


## Occasionally queue for the arena, exactly like a human would

Removed from `internal/engine/tick/engine.go:3353`.

~~~~text
Occasionally queue for the arena, exactly like a human would
via /arena. Mirrors HandleJoinQueueCallback's exact mechanics
(entry fee debited, an arena_queue row inserted) using the
cheapest ('solo') bracket - whatever tick pass matches queued
players needs no AI-specific handling, since this is just
another user_id sitting in the same queue.
~~~~


## Occasionally advance a research tech node, exactly like a

Removed from `internal/engine/tick/engine.go:3373`.

~~~~text
Occasionally advance a research tech node, exactly like a
human would via /research's "Upgrade" buttons - part of the
Item 4 inventory's "research/building upgrades" entry, which
was left as an open question ("does 'unit upgrades' mean the
existing garrison-building roll, or the human
research/building-upgrade tree?") in this plan doc. Answer:
both are real, distinct systems, so both get built - this is
the tech-tree half. Mirrors HandleUpgradeTechCallback's exact
mechanics (FOR UPDATE row lock, Neuro Core cost of
currentLvl*8, MaxResearchLevel of 20) with the seven tech
columns duplicated as literals rather than importing
researchTree from internal/bot/handlers, since the tick
engine deliberately doesn't depend on the bot/handlers package
(same reasoning the exchange lot prices and federation cost
above were duplicated for).
~~~~


## Occasionally queue a facility upgrade, exactly like a human

Removed from `internal/engine/tick/engine.go:3408`.

~~~~text
Occasionally queue a facility upgrade, exactly like a human
would via /camp's Structural/Defense Grid/Infrastructure
panels - the building-upgrade half of the same Item 4 entry
research handles above. Mirrors HandleUpgradeCallback's exact
mechanics for non-core modules: one upgrade in flight at a
time per Outpost (is_upgrading gate), a module's level can't
exceed the Outpost's own level (same "Prerequisite Block" rule
humans hit), cost of currentLvl*150 Scrap, and a 20-second
build timer. Deliberately reuses resolveCompletedUpgrades
unchanged - it already resolves any modules row with
is_upgrading = TRUE regardless of who owns it, so an
AI-queued upgrade completes and even notifies (to the
faction's own negative user_id, read by no one, same as every
other AI notification) with zero changes needed there.
Doesn't touch camp_core - that already grows via the garrison
section below once a faction maxes out its soldier/mech caps,
so gating this list on faction level (not campLvl+1) avoids
double-counting the same growth two different ways.
~~~~


## Occasionally dispatch a personal exploration expedition, exactly

Removed from `internal/engine/tick/engine.go:3454`.

~~~~text
Occasionally dispatch a personal exploration expedition, exactly
like a human would via /explore's "Launch Expedition" button -
the last self-contained, single-faction item from Item 4's
inventory (`exploration_sites`/`exploration_dispatches`,
distinct from the `scout_missions` Item 3 already covers -
scouting finds raid targets, exploration finds resource hauls
and first contacts). Mirrors HandleDispatchExpeditionCallback's
exact mechanics: one dispatch in flight at a time per Outpost,
cost of 30 Rations + 15 Metal, a weighted reward-site roll
(duplicated as aiExplorationTemplates literals rather than
imported from internal/bot/handlers/exploration.go, same
reasoning as every other duplicated literal in this function),
and a 20-45 minute travel window. resolveExplorationDispatches
further down engine.go needed zero changes to pick this up and
resolve it later - it already credits whichever encampment_id
dispatched, AI or human, and even rolls the same first-contact
discovery chance off that Outpost's Scout Walker count.
~~~~


## Heal only when actually injured - unlike Train (which a human

Removed from `internal/engine/tick/engine.go:3559`.

~~~~text
Heal only when actually injured - unlike Train (which a human
might click repeatedly for XP with no real "waste" concept),
clicking Heal on an already-'Perfect Health' Hero produces the
exact same state for a real Rations cost, which is the kind
of pointless spend the AI-vs-human parity goal never asked
for; every prior resource-spending block in this function
checks for an actual need first, same reasoning here.
~~~~


## Occasionally list surplus resources on the exchange, exactly

Removed from `internal/engine/tick/engine.go:3576`.

~~~~text
Occasionally list surplus resources on the exchange, exactly
like a human would via /exchange - project owner direction
2026-08-01: AI factions should "literally do every single
thing in the game," with market listings given as the
explicit example. Reuses the exact fixed lot sizes/prices
HandlePostListingCallback uses (50 metal for 150 dollars, 20
crystal for 300 dollars) rather than inventing AI-specific
pricing, so a listing on the shared exchange looks and
behaves identically regardless of who posted it - a buyer
can't tell the difference, the same principle launchAIRaid
already follows for raids. Only lists once there's a real
buffer left over afterward (not down to the last unit), and
crystal is preferred when available since it's the rarer,
more valuable resource.
~~~~


## Garrison is maxed for this level - expand instead,

Removed from `internal/engine/tick/engine.go:3614`.

~~~~text
Garrison is maxed for this level - expand instead,
raising the ceiling for future growth. Capped at 15 so
a truly ignored AI faction doesn't become an
unraidable superpower.
~~~~


## emitRaidRadarWarnings issues one proximity warning, not a warning per game

Removed from `internal/engine/tick/engine.go:3625`.

~~~~text
emitRaidRadarWarnings issues one proximity warning, not a warning per game
tick. The threshold is evaluated against the defender's actual radar and
garrisoned recon capability, so attackers are not announced at launch.
~~~~


## The conditional update is the idempotency guard: even if multiple

Removed from `internal/engine/tick/engine.go:3679`.

~~~~text
The conditional update is the idempotency guard: even if multiple
bot processes overlap, only the process that marks the row sends
the defender notification.
~~~~


## BUGFIX: co-op helper survivors were never credited home on

Removed from `internal/engine/tick/engine.go:3809`.

~~~~text
BUGFIX: co-op helper survivors were never credited home on
a NATURAL raid completion - only the manual "abort" path
in combat.go's HandleExpeditionActions did this. Every
helper who joined a raid that simply ran its course (won
or lost normally) had their surviving soldiers/mechs
permanently vanish into an orphaned raid_coop_members row.
raid_coop_members.soldiers_contributed/mechs_contributed
already reflects post-casualty survivor counts by this
point (updated during battle resolution above), so this
is a straight credit-back, same shape as the primary
attacker's just above.
~~~~


## Destroyers, Bombers, Liberators and Wraiths aren't yet draftable

Removed from `internal/engine/tick/engine.go:3929`.

~~~~text
Destroyers, Bombers, Liberators and Wraiths aren't yet draftable
into co-op lobbies (only the raid creator's primary force can
bring them), so no helper contribution to add here - just the
primary force.
~~~~


## Phase 7 AI scaling: the Nest now feeds the exact same

Removed from `internal/engine/tick/engine.go:3983`.

~~~~text
Phase 7 AI scaling: the Nest now feeds the exact same
per-subsystem fields a real defender does (Defense Grid,
Guardians/Observers, research, shields, hero superpower)
instead of a single flat fallback bonus, so it resolves
through the identical combat code path below as any
player base.
~~~~


## Phase 6 "engage-weapon" mechanics: pull each turret type's

Removed from `internal/engine/tick/engine.go:4020`.

~~~~text
Phase 6 "engage-weapon" mechanics: pull each turret type's
level individually so each one can apply its own situational
bonus below, instead of contributing to just one flat sum.
~~~~


## Guardian/Observer: garrison-only units that never leave the

Removed from `internal/engine/tick/engine.go:4029`.

~~~~text
Guardian/Observer: garrison-only units that never leave the
base, so they're read straight from workshop_inventory rather
than mobilized like Liberators/Wraiths.
~~~~


## Phase 7 (item 12): world events are now per-continent. Scope

Removed from `internal/engine/tick/engine.go:4040`.

~~~~text
Phase 7 (item 12): world events are now per-continent. Scope
this to the attacker's own region - always present (unlike
defenderID, which is null for AI/rogue-nest raids) and it's
the attacking fleet's own systems (targeting, mech corrosion)
executing the operation.
~~~~


## Orbital Maneuver: +30% defense rating while active

Removed from `internal/engine/tick/engine.go:4051`.

~~~~text
Orbital Maneuver: +30% defense rating while active
~~~~


## cheap, flat anti-infantry baseline

Removed from `internal/engine/tick/engine.go:4066`.

~~~~text
cheap, flat anti-infantry baseline
~~~~


## top-tier, no counter needed

Removed from `internal/engine/tick/engine.go:4070`.

~~~~text
top-tier, no counter needed
~~~~


## Wraith: stealth strike fighter. Its cloaking field partially

Removed from `internal/engine/tick/engine.go:4073`.

~~~~text
Wraith: stealth strike fighter. Its cloaking field partially
blinds the target's Defense Grid before the engagement,
shaving a chunk off the combined turret bonus.
~~~~


## Guardian: garrison-only heavy defense walker, adds directly to

Removed from `internal/engine/tick/engine.go:4080`.

~~~~text
Guardian: garrison-only heavy defense walker, adds directly to
defense rating. Observer: garrison-only recon satellite, a
smaller early-warning-flavored defense bonus (mirrors Scouts).
~~~~


## mechOffenseCount is the mech count used ONLY for the attack

Removed from `internal/engine/tick/engine.go:4097`.

~~~~text
mechOffenseCount is the mech count used ONLY for the attack
rating below; it defaults to the true totMechs and is reduced
only by acid_rain, so totMechs itself always stays accurate
for casualties, reports, and split ratios (see the bug-fix
comment on the acid_rain case below).
~~~~


## Solar Flare introduces accuracy/electronic target variance (scrambles telemetry)

Removed from `internal/engine/tick/engine.go:4108`.

~~~~text
Solar Flare introduces accuracy/electronic target variance (scrambles telemetry)
~~~~


## BUG FIX (2026-07-26): this used to mutate the shared

Removed from `internal/engine/tick/engine.go:4115`.

~~~~text
BUG FIX (2026-07-26): this used to mutate the shared
totMechs variable directly ("totMechs = totMechs / 2"),
which silently corrupted every downstream use of totMechs
for the rest of this raid's resolution - casualty-pool
distribution (soldierMechShare), the reported attacker
casualty count, the Mechs count shown in the battle
report, and the co-op raid split ratio all read totMechs
AFTER this line ran, so an acid-rain battle would show
half as many mechs as were actually committed and would
misallocate casualties between the soldier/mech pool and
the specialist pool. The corrosion penalty is now applied
only to a dedicated mechOffenseCount used solely in the
attack-rating formula below; totMechs itself (and every
report/ratio that depends on it) stays accurate.
~~~~


## EMP hits electronics-dependent systems hard - a flat, non-random offense penalty.

Removed from `internal/engine/tick/engine.go:4132`.

~~~~text
EMP hits electronics-dependent systems hard - a flat, non-random offense penalty.
~~~~


## Sandstorm degrades visibility/targeting - similar shape to Solar Flare's variance, but a s

Removed from `internal/engine/tick/engine.go:4136`.

~~~~text
Sandstorm degrades visibility/targeting - similar shape to Solar Flare's variance, but a straight reduction rather than a swing.
~~~~


## Phase 5 milestone 4: sustained electricity/logistics failure

Removed from `internal/engine/tick/engine.go:4147`.

~~~~text
Phase 5 milestone 4: sustained electricity/logistics failure
disables high-tech contributions (mech multiplier, in this
formula) without applying the full rations/ammo-depletion
offense penalty above - matches roadcombat.Power's identical
treatment for road battles.
~~~~


## Destroyer: anti-drone/anti-air specialist. Rating scales up the

Removed from `internal/engine/tick/engine.go:4160`.

~~~~text
Destroyer: anti-drone/anti-air specialist. Rating scales up the
more drones/jets the defender is fielding - a hard counter unit.
~~~~


## Bomber: siege specialist. Rating scales up the more fortified the

Removed from `internal/engine/tick/engine.go:4168`.

~~~~text
Bomber: siege specialist. Rating scales up the more fortified the
defender's Defense Grid is - a hard counter to turtled bases.
Guardian is Bomber's own hard counter: the more Guardians a
defender fields, the less a Bomber-heavy raid gains from this.
~~~~


## Liberator: mid-tier capital gunship, flat attack rating with no

Removed from `internal/engine/tick/engine.go:4178`.

~~~~text
Liberator: mid-tier capital gunship, flat attack rating with no
situational bonus - the accessible stepping stone between
Bombers and the Battlecruiser.
~~~~


## Wraith: stealth anti-air fighter. Rating scales up the more

Removed from `internal/engine/tick/engine.go:4186`.

~~~~text
Wraith: stealth anti-air fighter. Rating scales up the more
drones/jets the defender is fielding, same hard-counter shape
as Destroyer, on top of the Defense-Grid-blinding effect it
already applied to turretDefenseBonus above.
~~~~


## Battlecruiser: top-tier capital ship, flat massive attack rating

Removed from `internal/engine/tick/engine.go:4197`.

~~~~text
Battlecruiser: top-tier capital ship, flat massive attack rating
with no situational counter-bonus - the expensive, always-strong
"flex" unit. Attack value sourced from the canonical content
registry rather than hardcoded here.
~~~~


## Deathstar: the ultimate superweapon. Flat, enormous attack rating -

Removed from `internal/engine/tick/engine.go:4206`.

~~~~text
Deathstar: the ultimate superweapon. Flat, enormous attack rating -
a single Doomsday Rig can single-handedly decide a battle.
~~~~


## SpaceHunt-style "Exploding Units": each destroyed unit has a chance

Removed from `internal/engine/tick/engine.go:4247`.

~~~~text
SpaceHunt-style "Exploding Units": each destroyed unit has a chance
to blow up and take a unit from the opposing side down with it.
This keeps lopsided wins from being entirely free for the winner.
~~~~


## Casualty split: preserve the original 70/30 soldier/mech ratio for

Removed from `internal/engine/tick/engine.go:4266`.

~~~~text
Casualty split: preserve the original 70/30 soldier/mech ratio for
that pool, and split whatever share of casualties falls on the
Destroyer/Bomber pool proportional to their relative counts. When
no destroyers/bombers are drafted this reduces to exactly the old
behavior (soldierMechShare = 1.0).
~~~~


## bumped alongside the Phase 7 attack rating rebalance (was 150.0)

Removed from `internal/engine/tick/engine.go:4300`.

~~~~text
bumped alongside the Phase 7 attack rating rebalance (was 150.0)
~~~~


## Build the SpaceHunt-style battle report. Composition reflects

Removed from `internal/engine/tick/engine.go:4494`.

~~~~text
Build the SpaceHunt-style battle report. Composition reflects
forces standing AT THE START of this round (before losses); loss
tallies are what fell THIS round specifically.
Surface what recon already promised - Defense Grid, Guardians/
Observers, Nuclear Shields, and a Hero/Warlord's superpower -
in the actual battle outcome, not just the pre-attack preview.
These already silently factor into the combat math above
(turretDefenseBonus/guardianBonus/observerBonus/defenderShields);
this just makes that visible to the player reading the report.
~~~~


## AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 1.5: the game

Removed from `internal/engine/tick/engine.go:4602`.

~~~~text
AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 1.5: the game
already allows "lose everything" through sustained, repeated
raiding (0.6^n of resources survive after n full-loot raids -
under 8% by the 5th), but nothing ever told the defender that's
what's happening to them. This is purely a communication
addition - no new mechanic, no floor added or removed, just
naming what the existing math already does once someone's down
to near-nothing.
~~~~


## Attacker and defender see the identical report text, but

Removed from `internal/engine/tick/engine.go:4617`.

~~~~text
Attacker and defender see the identical report text, but
should feel opposite things about it - 2026-08-06: give each
recipient their own message-effect animation based on which
side of report.Outcome they're on, rather than either no
effect or (worse) the same one for the winner and the loser.
Multi-round battles still ongoing (OutcomeOngoing/OutcomeDraw)
deliberately get no effect - the fight isn't over, so neither
"win" nor "lose" framing is warranted yet.
~~~~


## Total wipeout of combat units, but Buggies aren't part of

Removed from `internal/engine/tick/engine.go:4642`.

~~~~text
Total wipeout of combat units, but Buggies aren't part of
the combat casualty pool - if any were committed, they
survive the rout and can still scavenge a little before
retreating. Nowhere near a real win's take, but not
nothing either - matches how transport crews would
realistically grab whatever's within reach and flee.
~~~~


## Cargo Ship tiers stack on top of a Hauler's base reduction,

Removed from `internal/engine/tick/engine.go:4715`.

~~~~text
Cargo Ship tiers stack on top of a Hauler's base reduction,
each tier cutting further into the return-march loot penalty.
~~~~


## The return trip is anchored to the SAME distance as the

Removed from `internal/engine/tick/engine.go:4727`.

~~~~text
The return trip is anchored to the SAME distance as the
outbound march (r.baseMarchMinutes), not some disconnected
flat number - carrying looted resources back only ever adds
time on top of that, it never makes the trip home shorter
than the trip out.
~~~~


## Clan War score: if the attacker and defender belong to

Removed from `internal/engine/tick/engine.go:4753`.

~~~~text
Clan War score: if the attacker and defender belong to
two clans with an active war between them, this victory
counts toward the attacker's clan war score - real
combat outcomes drive the war, not a one-time stub.
~~~~
