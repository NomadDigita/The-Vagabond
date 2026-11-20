# ADR 0138: Decisions from `internal/engine/tick/aijobs.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/aijobs.go` and moved here so the source no longer carries them. Each entry is the original comment.

## runAIJobs mirrors the /jobs panel's resource-only utility actions for

Removed from `internal/engine/tick/aijobs.go:13`.

~~~~text
runAIJobs mirrors the /jobs panel's resource-only utility actions for
AI factions - project owner direction 2026-08-01: "every single thing
a human can do." Split into its own file/phase rather than folded into
growAICivilizations, since that function was already large and these
actions are a genuinely separate category (one-off utility spends, not
passive growth or governance) - matches how aispawning.go and
aidecisions.go are already split out by concern.

Deliberately covers only the resource-only actions from jobs.go:
GatherSunlight, RepairUnits, RepairBuildings, HyperSpeed,
OrbitalManeuver, ExtendPlanet. HandleTeleport is deliberately excluded
- relocating already has a real equivalent for AI factions via
maybeAIFlee/Ghost Protocol in aidecisions.go, and a second, unrelated
relocation mechanic risks working against Item 1's continent-balanced
spawn placement for no real benefit.
~~~~


## Gather Sunlight: mirrors HandleGatherSunlight exactly - a free

Removed from `internal/engine/tick/aijobs.go:43`.

~~~~text
Gather Sunlight: mirrors HandleGatherSunlight exactly - a free
150 Electricity burst, gated by the same 30-minute
last_sunlight_at cooldown a human's own repeated taps would
hit.
~~~~


## Repair Units: mirrors HandleRepairUnits exactly - 200 Scrap

Removed from `internal/engine/tick/aijobs.go:61`.

~~~~text
Repair Units: mirrors HandleRepairUnits exactly - 200 Scrap
for +5 Soldiers.
~~~~


## Repair Buildings (rush construction): mirrors

Removed from `internal/engine/tick/aijobs.go:74`.

~~~~text
Repair Buildings (rush construction): mirrors
HandleRepairBuildings exactly - 150 Scrap halves the
remaining time on whichever module is currently upgrading.
Only relevant now that AI factions actually queue module
upgrades (see growAICivilizations's facility-upgrade block),
so this naturally does nothing until that's true for a given
faction - no ordering dependency needed between the two.
~~~~


## HyperSpeed: mirrors HandleHyperSpeed exactly - 300

Removed from `internal/engine/tick/aijobs.go:98`.

~~~~text
HyperSpeed: mirrors HandleHyperSpeed exactly - 300
Electricity halves the remaining time on the AI faction's
nearest active raid/mission. Only relevant once a faction has
an outbound raid via launchAIRaid, same non-dependency
reasoning as Repair Buildings above.
~~~~


## Orbital Maneuver: mirrors HandleOrbitalManeuver exactly - 400

Removed from `internal/engine/tick/aijobs.go:120`.

~~~~text
Orbital Maneuver: mirrors HandleOrbitalManeuver exactly - 400
Electricity for a +30% defense buff (orbital_buff_until) for
2 hours. A faction under raid pressure benefits from this the
same way a human under pressure would reach for it.
~~~~


## Extend Planet: mirrors HandleExtendPlanet exactly - a

Removed from `internal/engine/tick/aijobs.go:136`.

~~~~text
Extend Planet: mirrors HandleExtendPlanet exactly - a
permanent +1000 storage cap, cost scaling with
extension_lvl+1 (500 Metal / 100 Crystal at level 0,
doubling each level). Deliberately the rarest of these rolls -
it's the human "Jobs" panel's biggest, least-frequent
investment too, not something tapped casually.
~~~~
