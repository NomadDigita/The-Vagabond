# ADR 0154: Decisions from `internal/engine/tick/scoutmissions.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/scoutmissions.go` and moved here so the source no longer carries them. Each entry is the original comment.

## scoutMissionPingInterval gates BOTH the actual discovery-roll cadence

Removed from `internal/engine/tick/scoutmissions.go:18`.

~~~~text
scoutMissionPingInterval gates BOTH the actual discovery-roll cadence
during the searching phase AND the periodic "still searching"/"en route
home" status pings, off a single scout_missions.last_status_notified_at
column - mirroring the AI decision loop's cadence-gate pattern
(ai_last_decision_at), per AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md
section 3.5's own note.

This is a deliberate interpretation of an underspecified detail in the
plan doc, flagged explicitly rather than silently decided: the doc's
literal wording ("each tick... roll ExplorationDiscoveryChance") would,
taken at face value against a 60-second default tick interval, resolve
almost every search within 1-2 minutes - that trivializes "search until
found, no ETA" into "search for basically no time." Gating the roll on
the same ~30-minute cadence as the status ping keeps the probabilistic
model (worldintel.ExplorationDiscoveryChance) meaningful while still
giving the player periodic feedback that something is happening.
~~~~


## scoutBonusDiscoveryChance is a flat, modest chance per gate-open during

Removed from `internal/engine/tick/scoutmissions.go:36`.

~~~~text
scoutBonusDiscoveryChance is a flat, modest chance per gate-open during
the return leg that the party stumbles onto a second target passing by
- mirroring discoverRouteContacts' "reveal a base while marching past
it" mechanic for raids, but rolled rather than guaranteed-on-proximity
since a scout mission's return path (unlike a raid's fixed origin/
destination line) has no interpolatable route until it actually finds
its primary target.
~~~~


## scoutResourceRatePerHourPerScout calibrates the resource trickle from

Removed from `internal/engine/tick/scoutmissions.go:45`.

~~~~text
scoutResourceRatePerHourPerScout calibrates the resource trickle from
section 3.6. A normal /newjobexplore dispatch averages roughly a
32-minute round trip for a flat ~150-400 (avg ~275) single-resource
payout, uncommitted to any manpower. Scouting is a much longer,
open-ended, manpower-committing mission, so it's priced per scout per
hour instead of flat-per-dispatch: at 1 scout for a short ~2-hour
mission that's ~100 total value (less than one normal dispatch, since
nothing was guaranteed and manpower sat idle from other tasks the
whole time); at 5 scouts on a long ~4-hour mission that's ~1000 (a
genuinely meaningful haul for a long, risky-feeling commitment). Split
60/40 scrap/metal, matching the two most common exploration reward
tiers' relative weighting. Tunable starting constant, not a
precisely-right number - same spirit as this session's other new
tunables (ghostProtocolCostFraction, aiFleeGarrisonThreshold, etc.).
~~~~


## scoutMissionFindsTarget picks a target using the same no-omniscience

Removed from `internal/engine/tick/scoutmissions.go:135`.

~~~~text
scoutMissionFindsTarget picks a target using the same no-omniscience
selection query aiScout/resolveExplorationDiscovery already use
(undiscovered-by-this-observer, ordered by danger/established_at) -
but deliberately without a region filter, since long-range scouting is
meant to search the whole planet, not just the home continent (unlike
aiScout, which is intentionally home-continent-scoped for the AI
decision loop). A real player or an AI faction are both valid finds,
consistent with this session's earlier AI-vs-AI parity work.
~~~~


## Nothing left anywhere in the world for this observer to find -

Removed from `internal/engine/tick/scoutmissions.go:160`.

~~~~text
Nothing left anywhere in the world for this observer to find -
not an error, just keep searching.
~~~~


## known_locations is the coordinate SNAPSHOT (see schema.go's comment

Removed from `internal/engine/tick/scoutmissions.go:179`.

~~~~text
known_locations is the coordinate SNAPSHOT (see schema.go's comment
on the table) - locked now, at discovery time, distinct from the
permanent boolean relationship just recorded above.
~~~~


## Same Manhattan-distance march formula launchAIRaid already uses

Removed from `internal/engine/tick/scoutmissions.go:191`.

~~~~text
Same Manhattan-distance march formula launchAIRaid already uses
(steps * 10 minutes, minimum 1 minute) - one consistent notion of
travel time across the codebase rather than a scouting-specific one.
~~~~


## "general" (non-mutable), not "scout_status": the periodic

Removed from `internal/engine/tick/scoutmissions.go:214`.

~~~~text
"general" (non-mutable), not "scout_status": the periodic
"still searching" ping above is routine chatter and fine to
mute, but this is the actual discovery event - the entire
payoff of the mission - and MutableCategories' own contract
(notifications/preferences.go) is explicit that discovery
alerts must never be suppressible. This was previously tagged
"route_status" by mistake (before scouting got its own
mutable category), meaning a player who'd muted the routine
pings would silently lose this one too.
~~~~


## Being found is genuine information for the target too - matches

Removed from `internal/engine/tick/scoutmissions.go:229`.

~~~~text
Being found is genuine information for the target too - matches
discoverRouteContacts' existing "being seen is reciprocal
knowledge" precedent for raids, though a scout sighting doesn't
grant the target reciprocal discovery of the *scout's* base (a
distant, passive survey team isn't the same as a raid crossing
your road) - just a heads-up that they're now on someone's radar.
~~~~


## pingInTransitScoutMissions handles the still-en-route case: a periodic

Removed from `internal/engine/tick/scoutmissions.go:340`.

~~~~text
pingInTransitScoutMissions handles the still-en-route case: a periodic
"ETA" ping, plus a chance (scoutBonusDiscoveryChance, gated on the same
cadence) of a bonus mid-journey discovery mirroring
discoverRouteContacts' "reveal a base while marching past it" behavior
for raids.
~~~~


## scoutMissionBonusDiscovery interpolates the return leg's current

Removed from `internal/engine/tick/scoutmissions.go:401`.

~~~~text
scoutMissionBonusDiscovery interpolates the return leg's current
position (reusing roadcombat's exact progress/position math, the same
way discoverRouteContacts does for raids) and looks for an undiscovered
target within one tile.
~~~~
