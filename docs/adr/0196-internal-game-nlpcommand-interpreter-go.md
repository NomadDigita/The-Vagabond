# ADR 0196: Decisions from `internal/game/nlpcommand/interpreter.go`

Status: Accepted

These decisions were written as comments in `internal/game/nlpcommand/interpreter.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Interpreter is the Milestone 3 entry point, matching the

Removed from `internal/game/nlpcommand/interpreter.go:10`.

~~~~text
Interpreter is the Milestone 3 entry point, matching the
Planner/Advisor-style struct convention every sibling AI package
(researchplanner.Planner, econadvisor.Advisor, ...) uses. It
deliberately holds no *sql.DB, unlike every sibling package - see
the package doc comment: this interpreter never reads or writes
game state, only classifies intent.
~~~~


## Every other AI feature in this codebase (governor, fleetcommander,

Removed from `internal/game/nlpcommand/interpreter.go:48`.

~~~~text
Every other AI feature in this codebase (governor, fleetcommander,
econadvisor, ...) intentionally shows the mock provider's
placeholder text, because the player explicitly tapped a
dedicated "AI Advisor" button and "no live AI configured yet" is
a meaningful answer to that. This interpreter is different: it's
the last link in an ordinary free-text chain, so a player who
just mistyped something never explicitly asked for an AI
feature. Surfacing mock's raw echo/placeholder text there would
read as a confusing implementation leak rather than a helpful
reply, so treat a mock response as "no match" and let the caller
fall through to its pre-Milestone-3 behavior instead.
~~~~
