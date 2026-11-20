# ADR 0168: Decisions from `internal/game/content/units.go`

Status: Accepted

These decisions were written as comments in `internal/game/content/units.go` and moved here so the source no longer carries them. Each entry is the original comment.

## stable identifier, matches workshop_inventory column

Removed from `internal/game/content/units.go:28`.

~~~~text
stable identifier, matches workshop_inventory column
~~~~


## workshop_inventory column name (usually == Key + "s")

Removed from `internal/game/content/units.go:29`.

~~~~text
workshop_inventory column name (usually == Key + "s")
~~~~


## resource_column -> amount

Removed from `internal/game/content/units.go:34`.

~~~~text
resource_column -> amount
~~~~


## fraction of Cost refunded on deconstruction

Removed from `internal/game/content/units.go:35`.

~~~~text
fraction of Cost refunded on deconstruction
~~~~


## MaxDoomsdayRigs returns how many Doomsday Rigs a player at the given

Removed from `internal/game/content/units.go:39`.

~~~~text
MaxDoomsdayRigs returns how many Doomsday Rigs a player at the given
Outpost level may own at once. Phase 7 rebalance: the old hard cap of
exactly 1 made the unit feel irrelevant at scale (a single 500-rating
unit is negligible next to a garrison of thousands of Soldiers), so
ownership now grows with Outpost level instead - one Rig baseline,
plus one more every 5 levels, capped at 10 so it never becomes a
second army in its own right.
~~~~


## Units is the full canonical fleet roster. Existing units (soldier,

Removed from `internal/game/content/units.go:67`.

~~~~text
Units is the full canonical fleet roster. Existing units (soldier,
drone, mech, nuke, buggy, ship, jet, hauler, tanker, rig, destroyer,
bomber) are already wired into factory.go/deconstruct.go/combat.go
directly and are NOT yet migrated to read from here, to avoid touching
already-shipped, tested code paths without a functional reason. New
units from this point forward are defined here first.
~~~~


## MustFindUnit is like FindUnit but panics if the key doesn't exist. Safe

Removed from `internal/game/content/units.go:207`.

~~~~text
MustFindUnit is like FindUnit but panics if the key doesn't exist. Safe
to use in package-level var initializers (e.g. other handlers building
their own tables from this registry at startup) since a missing key
there is a programming error, not a runtime condition to handle gracefully.
~~~~
