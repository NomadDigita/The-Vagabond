# ADR 0080: Decisions from `internal/bot/handlers/gearpanel.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/gearpanel.go` and moved here so the source no longer carries them. Each entry is the original comment.

## GearHandler is the panel/craft/equip side of

Removed from `internal/bot/handlers/gearpanel.go:14`.

~~~~text
GearHandler is the panel/craft/equip side of
NEXT_GENERATION_FEATURES_PLAN.md Phase 3 - definitions and the core
craft/equip/drop logic live in internal/game/gear; the tick-engine
side (crafting queue resolution) is wired directly into
internal/engine/tick/engine.go's phase list via gear.ResolveCraftingQueue.
~~~~


## HandleGearPanel (/gear) shows the current equipped loadout (one slot

Removed from `internal/bot/handlers/gearpanel.go:27`.

~~~~text
HandleGearPanel (/gear) shows the current equipped loadout (one slot
per row), owned-but-unequipped inventory, and a Craft sub-section for
every Craftable definition with inline affordability - matching the
exchange panel's own established affordability-preview pattern.
~~~~
