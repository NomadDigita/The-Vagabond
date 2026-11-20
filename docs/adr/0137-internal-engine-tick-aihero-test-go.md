# ADR 0137: Decisions from `internal/engine/tick/aihero_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/aihero_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestGrowAICivilizationsCreatesHeroOnFirstTick verifies every AI faction

Removed from `internal/engine/tick/aihero_test.go:9`.

~~~~text
TestGrowAICivilizationsCreatesHeroOnFirstTick verifies every AI faction
gets exactly one Hero row, lazily created on its very first tick,
mirroring HandleHeroPanel's lazy-create behavior.
~~~~


## TestGrowAICivilizationsWontDuplicateHero verifies repeated ticks never

Removed from `internal/engine/tick/aihero_test.go:46`.

~~~~text
TestGrowAICivilizationsWontDuplicateHero verifies repeated ticks never
create a second Hero row for the same faction, relying on the
heroes.encampment_id UNIQUE constraint plus the explicit existence
check.
~~~~


## TestGrowAICivilizationsCanTrainHero verifies an AI faction with ample

Removed from `internal/engine/tick/aihero_test.go:81`.

~~~~text
TestGrowAICivilizationsCanTrainHero verifies an AI faction with ample
Scrap eventually trains its Hero (XP increases, and levels up once XP
crosses 100), mirroring HandleHeroCallback's "train" action exactly.
~~~~


## TestGrowAICivilizationsCanHealHero verifies an injured AI faction Hero

Removed from `internal/engine/tick/aihero_test.go:120`.

~~~~text
TestGrowAICivilizationsCanHealHero verifies an injured AI faction Hero
with ample Rations eventually gets healed, mirroring
HandleHeroCallback's "heal" action exactly.
~~~~


## Force-create the hero on tick 1, then seed a real injury before

Removed from `internal/engine/tick/aihero_test.go:134`.

~~~~text
Force-create the hero on tick 1, then seed a real injury before
letting the heal roll have a chance to fire.
~~~~


## TestGrowAICivilizationsWontHealAlreadyHealthyHero verifies an AI

Removed from `internal/engine/tick/aihero_test.go:174`.

~~~~text
TestGrowAICivilizationsWontHealAlreadyHealthyHero verifies an AI
faction never spends Rations healing a Hero that's already at
'Perfect Health' - the one deliberate AI-side guard on top of the
human handler's exact mechanics (see the comment above the block in
engine.go for the reasoning).
~~~~
