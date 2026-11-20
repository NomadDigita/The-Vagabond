# ADR 0135: Decisions from `internal/engine/tick/aiexploration_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/aiexploration_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestGrowAICivilizationsCanDispatchExploration verifies an AI faction

Removed from `internal/engine/tick/aiexploration_test.go:9`.

~~~~text
TestGrowAICivilizationsCanDispatchExploration verifies an AI faction
with ample Rations/Metal eventually dispatches a personal exploration
expedition, mirroring HandleDispatchExpeditionCallback's exact
mechanics (site + dispatch rows inserted, resources debited).
~~~~


## TestGrowAICivilizationsWontDispatchExplorationConcurrently verifies an

Removed from `internal/engine/tick/aiexploration_test.go:65`.

~~~~text
TestGrowAICivilizationsWontDispatchExplorationConcurrently verifies an
AI faction with one expedition already en route never starts a second
one at the same time, mirroring HandleDispatchExpeditionCallback's
"already have an expedition en route" guard.
~~~~


## TestGrowAICivilizationsWontDispatchExplorationWithoutSupplies verifies

Removed from `internal/engine/tick/aiexploration_test.go:115`.

~~~~text
TestGrowAICivilizationsWontDispatchExplorationWithoutSupplies verifies
an AI faction with insufficient Rations/Metal never dispatches an
exploration expedition, mirroring HandleDispatchExpeditionCallback's
"Insufficient supplies" guard.
~~~~


## Zero out the level-1 Scrap/Rations/Metal trickle after every

Removed from `internal/engine/tick/aiexploration_test.go:142`.

~~~~text
Zero out the level-1 Scrap/Rations/Metal trickle after every
tick so this faction never accumulates enough to afford the
30 Rations / 15 Metal dispatch cost across all 50 iterations.
~~~~
