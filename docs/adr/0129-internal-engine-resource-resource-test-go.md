# ADR 0129: Decisions from `internal/engine/resource/resource_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/resource/resource_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## seedResourceCamp creates a camp with a Scrap Heap, Generator, Metal

Removed from `internal/engine/resource/resource_test.go:36`.

~~~~text
seedResourceCamp creates a camp with a Scrap Heap, Generator, Metal
Mine, and Crystal Mine all at level 1 (so every passive generation
line RunResourcePass touches produces a clean, non-zero, easily
hand-verified amount) in the given region.
~~~~


## TestRunResourcePass_BloomBoostsPassiveGenerationBy15Percent is the

Removed from `internal/engine/resource/resource_test.go:68`.

~~~~text
TestRunResourcePass_BloomBoostsPassiveGenerationBy15Percent is the
direct test for RARE_WORLD_FEATURES_PLAN.md Phase 2.3: a continent
with an active "bloom" event should see every passively-generated
resource line (Scrap/Metal/Crystal here - the ones a level-1 camp
with no loan/troops actually produces a clean, comparable amount of)
come out to exactly 1.15x what an identical camp in a nominal-weather
continent produces in the same pass.
~~~~


## TestRunResourcePass_EquippedGearBoostsPassiveGeneration is the

Removed from `internal/engine/resource/resource_test.go:125`.

~~~~text
TestRunResourcePass_EquippedGearBoostsPassiveGeneration is the
direct test for NEXT_GENERATION_FEATURES_PLAN.md Phase 3's resource-
generation integration: a camp with equipped gear generates more
than an identical camp with none, by exactly the gear's combined
ResourceBonusPct.
~~~~


## scrap_blade (+2%) + patched_vest (+2%) = +4% total, equipped on

Removed from `internal/engine/resource/resource_test.go:138`.

~~~~text
scrap_blade (+2%) + patched_vest (+2%) = +4% total, equipped on
gearedCamp only.
~~~~


## TestRunResourcePass_PrestigeBonusBoostsPassiveGeneration is the

Removed from `internal/engine/resource/resource_test.go:176`.

~~~~text
TestRunResourcePass_PrestigeBonusBoostsPassiveGeneration is the
direct test for NEXT_GENERATION_FEATURES_PLAN.md Phase 4's resource-
tick integration: a camp with a nonzero prestige_bonus_pct generates
more than an identical camp at 0%, by exactly that bonus.
~~~~
