# ADR 0199: Decisions from `internal/game/npcintel/intel.go`

Status: Accepted

These decisions were written as comments in `internal/game/npcintel/intel.go` and moved here so the source no longer carries them. Each entry is the original comment.

## MemoryScope namespaces this feature's conversational history inside

Removed from `internal/game/npcintel/intel.go:13`.

~~~~text
MemoryScope namespaces this feature's conversational history inside
ai_memory, distinct from every other Phase B+ scope.
~~~~


## ErrNoEncampment is returned when the calling player has no

Removed from `internal/game/npcintel/intel.go:17`.

~~~~text
ErrNoEncampment is returned when the calling player has no
registered base yet (and therefore no level to scale the Nest to,
and no fleet to read).
~~~~


## BuildSnapshot loads the calling player's level (to scale the Nest

Removed from `internal/game/npcintel/intel.go:32`.

~~~~text
BuildSnapshot loads the calling player's level (to scale the Nest
via the same content.RogueNestComposition Fleet Commander and
/recon_ai already use, so this never disagrees with either) and
their current mobile fleet from workshop_inventory.
~~~~


## buildFleet reads only the mobile combat units from

Removed from `internal/game/npcintel/intel.go:75`.

~~~~text
buildFleet reads only the mobile combat units from
workshop_inventory — Guardians/Observers are garrison-only (never
leave the base on a raid, per their own doc comments in
internal/engine/tick/engine.go) so they're deliberately excluded
from the player's own FleetProfile even though they appear on the
Nest side.
~~~~


## Recommend produces a fresh AI tactical read for the given player

Removed from `internal/game/npcintel/intel.go:99`.

~~~~text
Recommend produces a fresh AI tactical read for the given player
against the Rogue Drone Nest scaled to their level. It stores both
turns in ai_memory under MemoryScope.

Read-only: nothing in this method launches a raid or moves any unit
— it only reads and advises.
~~~~
