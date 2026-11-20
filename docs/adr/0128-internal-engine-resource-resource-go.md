# ADR 0128: Decisions from `internal/engine/resource/resource.go`

Status: Accepted

These decisions were written as comments in `internal/engine/resource/resource.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Phase 7 (item 12): world events are per-continent now. Fetch every

Removed from `internal/engine/resource/resource.go:54`.

~~~~text
Phase 7 (item 12): world events are per-continent now. Fetch every
continent's active event once (cheap, small result set) instead of
re-querying per encampment inside the loop below.
~~~~


## RARE_WORLD_FEATURES_PLAN.md Phase 2: Seasonal Wasteland

Removed from `internal/engine/resource/resource.go:117`.

~~~~text
RARE_WORLD_FEATURES_PLAN.md Phase 2: Seasonal Wasteland
Blooms. The wasteland's one purely positive weather event -
a flat +15% applied to every passively-generated resource
line below (Scrap/Rations/Electricity/Ether/Metal/Crystal),
the mirror image of Supply Crisis's negative effect
elsewhere in this codebase. Computed once here so every
generation line below can apply it identically rather than
duplicating the "is this continent blooming" check at each
site.
~~~~


## NEXT_GENERATION_FEATURES_PLAN.md Phase 3: Crafting & Gear.

Removed from `internal/engine/resource/resource.go:133`.

~~~~text
NEXT_GENERATION_FEATURES_PLAN.md Phase 3: Crafting & Gear.
Equipped gear's ResourceBonusPct sums into one more
multiplier here, stacking with (not replacing) Bloom's own -
e.g. a blooming region with a fully-equipped 15%-per-slot
legendary loadout gets 1.15 * 1.45, not one overriding the
other. See gear.go's package doc for why this, not raid
combat power, is what "stat modifier" means for gear.
~~~~


## EMP knocks out unshielded electronics outright, per the news headline.

Removed from `internal/engine/resource/resource.go:153`.

~~~~text
EMP knocks out unshielded electronics outright, per the news headline.
~~~~


## Disease outbreak: commanders divert extra rations to treatment, per the news headline.

Removed from `internal/engine/resource/resource.go:172`.

~~~~text
Disease outbreak: commanders divert extra rations to treatment, per the news headline.
~~~~


## Surplus Preservation System: Only caps new passive allocations. Pre-existing balances are 

Removed from `internal/engine/resource/resource.go:179`.

~~~~text
Surplus Preservation System: Only caps new passive allocations. Pre-existing balances are preserved.
~~~~


## Ether trickles in slowly, scaled by Technology research and

Removed from `internal/engine/resource/resource.go:205`.

~~~~text
Ether trickles in slowly, scaled by Technology research and
boosted further by the Technology Center building. Same cap
rule as everything else: capped if generated passively.
~~~~


## Metal Mine / Crystal Mine: passive building-based generation,

Removed from `internal/engine/resource/resource.go:213`.

~~~~text
Metal Mine / Crystal Mine: passive building-based generation,
distinct from (and stacking with) the active miner-queue system.
Now cap-checked the same as every other passive resource gain.
~~~~
