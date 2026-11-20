# ADR 0194: Decisions from `internal/game/herodepth/herodepth.go`

Status: Accepted

These decisions were written as comments in `internal/game/herodepth/herodepth.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package herodepth is NEXT_GENERATION_FEATURES_PLAN.md Phase 6: Hero

Removed from `internal/game/herodepth/herodepth.go:1`.

~~~~text
Package herodepth is NEXT_GENERATION_FEATURES_PLAN.md Phase 6: Hero
System Depth - a skill tree, a hero-specific gear slot distinct from
Phase 3's camp-level gear equip, and a level-gated story arc.
Definitions (skill nodes, story beats) are static Go data, matching
every other definition catalog in this codebase.

Scope note on skill effects, mirroring gear.go's own scope note:
skill nodes grant safe, non-combat bonuses (training cost reduction,
healing cost reduction, resource-generation bonus) rather than raid-
combat-power modifiers, for the same reason gear's bonus stayed out
of resolveRaidCombats - see gear.go's package doc for the full
rationale.
~~~~


## Querier is satisfied by both *sql.DB and *sql.Tx, matching the same

Removed from `internal/game/herodepth/herodepth.go:23`.

~~~~text
Querier is satisfied by both *sql.DB and *sql.Tx, matching the same
convention gear.Querier/progression.Querier already establish.
~~~~


## Branch is one of a faction's three skill-tree branches.

Removed from `internal/game/herodepth/herodepth.go:31`.

~~~~text
Branch is one of a faction's three skill-tree branches.
~~~~


## negative = discount

Removed from `internal/game/herodepth/herodepth.go:50`.

~~~~text
negative = discount
~~~~


## negative = discount (2)

Removed from `internal/game/herodepth/herodepth.go:51`.

~~~~text
negative = discount
~~~~


## positive = bonus

Removed from `internal/game/herodepth/herodepth.go:52`.

~~~~text
positive = bonus
~~~~


## SkillNode is one purchasable skill-tree node - independent (no

Removed from `internal/game/herodepth/herodepth.go:55`.

~~~~text
SkillNode is one purchasable skill-tree node - independent (no
prerequisite chain, kept deliberately simple for this build), costs
1 SkillPoint, and belongs to exactly one Faction + Branch.
~~~~


## "steel_vanguard" or "rust_nomads", matches users.faction

Removed from `internal/game/herodepth/herodepth.go:62`.

~~~~text
"steel_vanguard" or "rust_nomads", matches users.faction
~~~~


## SkillTree is the full launch catalog - 3 branches x 3 nodes per

Removed from `internal/game/herodepth/herodepth.go:68`.

~~~~text
SkillTree is the full launch catalog - 3 branches x 3 nodes per
faction, 18 nodes total. Growing this catalog later needs no new
code, the same property every other definition catalog in this
codebase is built around.
~~~~


## StoryBeat is one narrative moment delivered as a notification the

Removed from `internal/game/herodepth/herodepth.go:122`.

~~~~text
StoryBeat is one narrative moment delivered as a notification the
instant a hero reaches Level. Faction-flavored, matching the two
hero archetypes hero.go's own fallback flavor text already
establishes (Steel Vanguard / Waste Phantom).
~~~~


## StoryArc is the full 5-beat launch arc per faction (levels

Removed from `internal/game/herodepth/herodepth.go:132`.

~~~~text
StoryArc is the full 5-beat launch arc per faction (levels
5/10/15/20/25, matching the plan doc's own design).
~~~~


## DeliverStoryBeats checks heroID's current level/faction/story_arc_stage

Removed from `internal/game/herodepth/herodepth.go:148`.

~~~~text
DeliverStoryBeats checks heroID's current level/faction/story_arc_stage
and delivers (as a direct notification) every StoryBeat whose Level
has now been reached but not yet sent, advancing story_arc_stage so
none is ever repeated. Called from hero.go's HandleHeroCallback right
after a level-up, the same site skill points are awarded.
~~~~


## TrainingCostMultiplier and HealCostMultiplier sum every unlocked

Removed from `internal/game/herodepth/herodepth.go:212`.

~~~~text
TrainingCostMultiplier and HealCostMultiplier sum every unlocked
skill's percentage effect of the matching EffectType into a single
multiplier (e.g. two -10% nodes unlocked = 0.80, with a 20% cost
floor so stacking discounts can't reach zero or negative) - hero.go's
"train"/"heal" actions call these before charging their resource cost.
~~~~


## ResourceBonusPctFor sums every unlocked skill's EffectResourceGenPct

Removed from `internal/game/herodepth/herodepth.go:234`.

~~~~text
ResourceBonusPctFor sums every unlocked skill's EffectResourceGenPct
for campID's hero - resource.go reads this at the exact same call
site gear.ResourceBonusPctFor and Phase 2's Bloom multiplier already
use, stacking with (not replacing) either.
~~~~


## EquipToHero marks a player_gear row as the hero's equipped item -

Removed from `internal/game/herodepth/herodepth.go:264`.

~~~~text
EquipToHero marks a player_gear row as the hero's equipped item -
like gear.Equip, but on the separate equipped_by_hero column, so the
same physical item can be equipped at the camp level (gear.Equip)
and to the hero (this function) independently, or neither, or both.
A hero has exactly one hero-equipped item total (not per-slot -
simpler than the camp's 3-slot model, since a hero is one
combatant, not a squad), so equipping a new item here unequips
whatever the hero had before.
~~~~


## HeroXPBonusPct returns the hero-equipped item's XP-gain bonus for

Removed from `internal/game/herodepth/herodepth.go:284`.

~~~~text
HeroXPBonusPct returns the hero-equipped item's XP-gain bonus for
campID (0 if nothing is hero-equipped) - hero.go's "train" action
applies this to the +20 XP a training action grants. defByKey lets
the caller supply gear.ByKey without this package importing gear
directly (avoids a two-way dependency between the two packages;
hero.go, which already imports both, is the natural place to wire
them together).
~~~~
