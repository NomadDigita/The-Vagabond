# ADR 0188: Decisions from `internal/game/governor/governor.go`

Status: Accepted

These decisions were written as comments in `internal/game/governor/governor.go` and moved here so the source no longer carries them. Each entry is the original comment.

## MemoryScope namespaces this feature's conversational history inside

Removed from `internal/game/governor/governor.go:12`.

~~~~text
MemoryScope namespaces this feature's conversational history inside
ai_memory so it never collides with Fleet Commander, Guild
Assistant, etc., which will use their own scope strings.
~~~~


## Governor is the Phase B entry point. One instance is shared across

Removed from `internal/game/governor/governor.go:21`.

~~~~text
Governor is the Phase B entry point. One instance is shared across
all requests (it holds only a DB pool and the shared AI Service,
both already safe for concurrent use).
~~~~


## BuildSnapshot loads the current state of a player's base. Every

Removed from `internal/game/governor/governor.go:33`.

~~~~text
BuildSnapshot loads the current state of a player's base. Every
COALESCE default mirrors internal/engine/resource.RunResourcePass so
the Governor's picture of "level 1 / not built yet" agrees with the
tick engine's own defaults.
~~~~


## Recommend produces a fresh AI recommendation for the given player's

Removed from `internal/game/governor/governor.go:81`.

~~~~text
Recommend produces a fresh AI recommendation for the given player's
base. It stores both the prompt and the response in ai_memory
(scoped to MemoryScope) so a later call to Recommend — or any future
feature that reads the same scope — has continuity without needing
to re-derive it from raw DB state.

This is advisory-only: nothing in this method mutates encampments,
modules, or any other game table. See PROJECT_MASTER_PLAN.md §4 for
why autopilot execution is deliberately not implemented yet.
~~~~


## AutopilotSetting reports whether a player has opted into autopilot

Removed from `internal/game/governor/governor.go:124`.

~~~~text
AutopilotSetting reports whether a player has opted into autopilot
for their base. Reading/writing this preference is implemented now
so the UI and data model exist, but — importantly — nothing in this
package currently *acts* on a true value. No code path calls
upgrade/build/repair automatically. That execution engine is
explicitly deferred (see PROJECT_MASTER_PLAN.md §4) so it can be
built and tested deliberately rather than bolted on here.
~~~~


## SetAutopilot stores the player's preference. See the doc comment on

Removed from `internal/game/governor/governor.go:148`.

~~~~text
SetAutopilot stores the player's preference. See the doc comment on
AutopilotSetting: this does not currently cause any autonomous
action to occur.
~~~~
