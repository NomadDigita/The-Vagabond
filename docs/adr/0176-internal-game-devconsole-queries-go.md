# ADR 0176: Decisions from `internal/game/devconsole/queries.go`

Status: Accepted

These decisions were written as comments in `internal/game/devconsole/queries.go` and moved here so the source no longer carries them. Each entry is the original comment.

## ── Safety model for natural-language admin queries ──────────────────

Removed from `internal/game/devconsole/queries.go:12`.

~~~~text
── Safety model for natural-language admin queries ──────────────────

The model NEVER writes or sees raw SQL, and nothing it outputs is
ever concatenated into a query. Instead: the model picks one name
from queryIntents (a fixed whitelist) plus a small number of
bounded, validated integer parameters (see clampDays/clampLimit in
nlquery.go); this file maps that choice to one specific, already-
written parameterized query and executes it. An admin asking "drop
all users" or "run this SQL: ..." has no path to actually doing
that — the worst a malicious or confused free-text question can do
is select an intent from this list, which are all read-only by
construction (every query below is a SELECT). This mirrors the
discipline used throughout the AI Systems Roadmap of never letting
model output become a code/query path — see ADR-019 and this
package's own doc comment for why Phase J stayed deliberately
narrow, and this file's addition is the one place that narrowness
gets safely widened: more READ-ONLY intents, never open-ended
execution.
~~~~


## Phase 7 milestone 3: "Add admin/dev-console metrics for discovery

Removed from `internal/game/devconsole/queries.go:41`.

~~~~text
Phase 7 milestone 3: "Add admin/dev-console metrics for discovery
rate, travel delays, stranded armies, convoy outcomes, field-battle
frequency, AI growth, loot mix, Crystal flow, and speed-up
expenditure." All nine, added together since they're one milestone.
~~~~


## IsKnownIntent reports whether name is in the whitelist. Any name the

Removed from `internal/game/devconsole/queries.go:68`.

~~~~text
IsKnownIntent reports whether name is in the whitelist. Any name the
model returns that isn't in this list is rejected outright by
nlquery.go, never executed as a fallback guess.
~~~~


## RunIntent executes exactly one whitelisted, already-parameterized

Removed from `internal/game/devconsole/queries.go:76`.

~~~~text
RunIntent executes exactly one whitelisted, already-parameterized
query and renders the result as a plain-text data block for the
model's second (answer) call. days and limit are pre-validated by
the caller (see clampDays/clampLimit in nlquery.go) before reaching
here.
~~~~


## Unreachable in practice — nlquery.go rejects unknown

Removed from `internal/game/devconsole/queries.go:366`.

~~~~text
Unreachable in practice — nlquery.go rejects unknown
intents before calling RunIntent — but fail closed rather
than execute anything if it ever is.
~~~~


## buildTopPlayersN and buildRecentNewsN are limit-parameterized

Removed from `internal/game/devconsole/queries.go:373`.

~~~~text
buildTopPlayersN and buildRecentNewsN are limit-parameterized
variants of BuildSnapshot's fixed-count helpers, added for
RunIntent's use without changing the weekly-report path's existing
behavior.
~~~~
