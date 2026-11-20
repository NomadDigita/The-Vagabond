# ADR 0142: Decisions from `internal/engine/tick/aiupgrades_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/aiupgrades_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestGrowAICivilizationsCanAdvanceResearch verifies an AI faction with

Removed from `internal/engine/tick/aiupgrades_test.go:9`.

~~~~text
TestGrowAICivilizationsCanAdvanceResearch verifies an AI faction with
ample Neuro Cores eventually advances at least one of the seven
research_states tech columns, mirroring HandleUpgradeTechCallback's
exact mechanics (cost = currentLvl*8, applied to whichever column is
rolled).
~~~~


## TestGrowAICivilizationsWontAdvanceResearchPastMaxLevel verifies every

Removed from `internal/engine/tick/aiupgrades_test.go:52`.

~~~~text
TestGrowAICivilizationsWontAdvanceResearchPastMaxLevel verifies every
tech column stays capped at aiResearchMaxLevel (20) even with unlimited
Neuro Cores available, mirroring HandleUpgradeTechCallback's own
MaxResearchLevel guard.
~~~~


## TestGrowAICivilizationsCanUpgradeAModule verifies a high-level AI

Removed from `internal/engine/tick/aiupgrades_test.go:97`.

~~~~text
TestGrowAICivilizationsCanUpgradeAModule verifies a high-level AI
faction with ample Scrap eventually queues a facility upgrade (a
modules row transitioning to is_upgrading = TRUE), mirroring
HandleUpgradeCallback's exact mechanics for non-core modules.
~~~~


## TestGrowAICivilizationsWontQueueSecondModuleUpgradeConcurrently verifies

Removed from `internal/engine/tick/aiupgrades_test.go:132`.

~~~~text
TestGrowAICivilizationsWontQueueSecondModuleUpgradeConcurrently verifies
an AI faction with one module already mid-upgrade never starts a second
one at the same time, mirroring HandleUpgradeCallback's "Construction
Queue Busy" guard.
~~~~


## far in the future, so it never completes mid-test

Removed from `internal/engine/tick/aiupgrades_test.go:146`.

~~~~text
far in the future, so it never completes mid-test
~~~~


## TestGrowAICivilizationsWontUpgradeModuleAboveFactionLevel verifies a

Removed from `internal/engine/tick/aiupgrades_test.go:174`.

~~~~text
TestGrowAICivilizationsWontUpgradeModuleAboveFactionLevel verifies a
low-level AI faction (level 1) never queues a module upgrade even with
unlimited Scrap, since every module starts at level 1 and 1 is not
less than a faction level of 1 - mirrors HandleUpgradeCallback's
"Prerequisite Block: Module levels cannot exceed your Outpost Core
level" rule.
~~~~


## TestGrowAICivilizationsNeuroCoreTrickleRespectsStorageCap verifies the

Removed from `internal/engine/tick/aiupgrades_test.go:214`.

~~~~text
TestGrowAICivilizationsNeuroCoreTrickleRespectsStorageCap verifies the
Neuro Core trickle added alongside the research block clamps to the
faction's storage cap rather than growing unbounded, same as every
other resource trickle in growAICivilizations.
~~~~
