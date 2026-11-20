# ADR 0206: Decisions from `internal/game/researchplanner/planner.go`

Status: Accepted

These decisions were written as comments in `internal/game/researchplanner/planner.go` and moved here so the source no longer carries them. Each entry is the original comment.

## MemoryScope namespaces this feature's conversational history inside

Removed from `internal/game/researchplanner/planner.go:12`.

~~~~text
MemoryScope namespaces this feature's conversational history inside
ai_memory, distinct from every other Phase B+ scope.
~~~~


## BuildSnapshot loads the current tech tree state of a player's base.

Removed from `internal/game/researchplanner/planner.go:30`.

~~~~text
BuildSnapshot loads the current tech tree state of a player's base.
goal may be empty ("infer it") or one of ValidGoals().
~~~~


## fetchResearchLevels reads all 7 tech levels for an encampment,

Removed from `internal/game/researchplanner/planner.go:60`.

~~~~text
fetchResearchLevels reads all 7 tech levels for an encampment,
initializing the row if it doesn't exist yet — matching
handlers.ResearchHandler.fetchResearchLevels exactly so the two
never disagree about a brand-new player's starting state.
~~~~


## Recommend produces a fresh AI recommendation for the given player's

Removed from `internal/game/researchplanner/planner.go:92`.

~~~~text
Recommend produces a fresh AI recommendation for the given player's
research order. It stores both turns in ai_memory under MemoryScope.

Advisory-only: nothing in this method spends a single Neuro Core or
otherwise mutates research_states/resources — that stays exclusively
in handlers.ResearchHandler.HandleUpgradeTechCallback, which the
player must tap themselves.
~~~~
