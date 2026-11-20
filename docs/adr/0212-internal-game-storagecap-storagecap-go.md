# ADR 0212: Decisions from `internal/game/storagecap/storagecap.go`

Status: Accepted

These decisions were written as comments in `internal/game/storagecap/storagecap.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package storagecap is the single source of truth for outpost storage

Removed from `internal/game/storagecap/storagecap.go:1`.

~~~~text
Package storagecap is the single source of truth for outpost storage
caps. Every resource in the `resources` table (Scrap, Rations,
Electricity, Metal, Crystal, Hydrogen, Dollars, Neuro Cores, Ether) is
subject to the same cap, derived from Tent + Warehouse + Extension
levels. This was previously only enforced for Scrap/Rations/Electricity
in the passive resource tick and the automation agent's `collector`
mode; Phase 7 (full audit, see SPACEHUNT_PHASE7_LOG.md item 5) extends
it to every resource-gain site in the game.
~~~~


## Queryer is satisfied by both *sql.DB and *sql.Tx, so callers already

Removed from `internal/game/storagecap/storagecap.go:17`.

~~~~text
Queryer is satisfied by both *sql.DB and *sql.Tx, so callers already
holding an open transaction can reuse it for the levels lookup instead
of opening a second connection.
~~~~


## Levels fetches the Tent/Warehouse module levels and the Outpost's

Removed from `internal/game/storagecap/storagecap.go:32`.

~~~~text
Levels fetches the Tent/Warehouse module levels and the Outpost's
Extension level for a camp in a single query, for use with Cap.
Missing modules/extension default to their un-upgraded value (Tent 1,
Warehouse 0, Extension 0) rather than erroring.
~~~~


## Clamp applies the "Surplus Preservation System" rule already used for

Removed from `internal/game/storagecap/storagecap.go:55`.

~~~~text
Clamp applies the "Surplus Preservation System" rule already used for
passive Scrap/Rations/Electricity generation: a pre-existing balance
already at or above the cap is never reduced, but no further gain is
added on top of it. Below the cap, gains are allowed up to (but not
past) it. The discarded remainder (if any) is returned alongside the
new balance so callers can surface it in a notification if useful.
~~~~
