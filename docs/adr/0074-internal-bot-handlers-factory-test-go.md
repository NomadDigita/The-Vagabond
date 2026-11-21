# ADR 0074: Decisions from `internal/bot/handlers/factory_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/factory_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Every item key any "Recruit"/"Forge"/"Build"/"Craft" button on

Removed from `internal/bot/handlers/factory_test.go:9`.

~~~~text
Every item key any "Recruit"/"Forge"/"Build"/"Craft" button on
HandleRecruitPanel or HandleVehiclesPanel actually sends as its
craft_item callback data - if a button's key isn't resolvable here,
tapping it silently fails with "❌ Unknown item." This test is the
guard against that drifting out of sync as buttons/specs evolve.
~~~~


## TestCraftSpecFor_BulkCeilings is the actual "up to 20x, except for some

Removed from `internal/bot/handlers/factory_test.go:54`.

~~~~text
TestCraftSpecFor_BulkCeilings is the actual "up to 20x, except for some
units" policy check - locks in exactly which units get the full 20x
ceiling and which are deliberately capped lower, so a future edit that
accidentally removes deathstar/nuke's special case (e.g. a careless
find-replace) is caught immediately instead of silently allowing a
20x Nuclear Device order.
~~~~


## extreme power-per-unit, see hardcodedCraftSpecs' comment

Removed from `internal/bot/handlers/factory_test.go:68`.

~~~~text
extreme power-per-unit, see hardcodedCraftSpecs' comment
~~~~


## total-fleet level-cap makes a high per-order ceiling meaningless/misleading

Removed from `internal/bot/handlers/factory_test.go:69`.

~~~~text
total-fleet level-cap makes a high per-order ceiling meaningless/misleading
~~~~


## Zero-amount entries (a resource key present but costing 0) must not

Removed from `internal/bot/handlers/factory_test.go:111`.

~~~~text
Zero-amount entries (a resource key present but costing 0) must not
render as "0 Metal" clutter.
~~~~
