# ADR 0139: Decisions from `internal/engine/tick/aijobs_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/aijobs_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestRunAIJobsCanGatherSunlight verifies an AI faction eventually gets

Removed from `internal/engine/tick/aijobs_test.go:10`.

~~~~text
TestRunAIJobsCanGatherSunlight verifies an AI faction eventually gets
a free Electricity burst, gated by the same 30-minute cooldown a
human's own repeated taps would hit.
~~~~


## Immediately after gaining, the 30-minute cooldown should block a

Removed from `internal/engine/tick/aijobs_test.go:45`.

~~~~text
Immediately after gaining, the 30-minute cooldown should block a
second gain even across many more ticks.
~~~~


## TestRunAIJobsCanRepairUnits verifies an AI faction with enough Scrap

Removed from `internal/engine/tick/aijobs_test.go:69`.

~~~~text
TestRunAIJobsCanRepairUnits verifies an AI faction with enough Scrap
eventually spends 200 for +5 Soldiers.
~~~~


## TestRunAIJobsCanRushBuildingUpgrade verifies an AI faction with a

Removed from `internal/engine/tick/aijobs_test.go:108`.

~~~~text
TestRunAIJobsCanRushBuildingUpgrade verifies an AI faction with a
module mid-upgrade eventually spends Scrap to halve the remaining time.
~~~~


## TestRunAIJobsCanUseHyperSpeed verifies an AI faction with an active

Removed from `internal/engine/tick/aijobs_test.go:150`.

~~~~text
TestRunAIJobsCanUseHyperSpeed verifies an AI faction with an active
outbound raid eventually spends Electricity to halve its remaining
resolve time.
~~~~


## TestRunAIJobsCanUseOrbitalManeuverAndExtendPlanet verifies the two

Removed from `internal/engine/tick/aijobs_test.go:194`.

~~~~text
TestRunAIJobsCanUseOrbitalManeuverAndExtendPlanet verifies the two
lower-probability actions still fire given enough ticks.
~~~~
