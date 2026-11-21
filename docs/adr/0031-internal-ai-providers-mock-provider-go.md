# ADR 0031: Decisions from `internal/ai/providers/mock/provider.go`

Status: Accepted

These decisions were written as comments in `internal/ai/providers/mock/provider.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package mock is a zero-dependency AI Provider used as the default

Removed from `internal/ai/providers/mock/provider.go:1`.

~~~~text
Package mock is a zero-dependency AI Provider used as the default
fallback so The Vagabond always boots and every AI-driven feature
degrades gracefully instead of erroring when no real provider is
configured. It never makes network calls.
~~~~


## Provider implements ai.Provider with canned, deterministic replies.

Removed from `internal/ai/providers/mock/provider.go:15`.

~~~~text
Provider implements ai.Provider with canned, deterministic replies.
It is always Available(), so it belongs last in every fallback chain.
~~~~


## Unknown feature or non-JSON caller: fall back to a plain,

Removed from `internal/ai/providers/mock/provider.go:35`.

~~~~text
Unknown feature or non-JSON caller: fall back to a plain,
clearly-labeled echo rather than guessing at a JSON shape
this provider doesn't know.
~~~~


## placeholderJSON returns a valid, well-formed JSON object matching

Removed from `internal/ai/providers/mock/provider.go:56`.

~~~~text
placeholderJSON returns a valid, well-formed JSON object matching
the structured shape each Phase B-J feature parses, so a request
made with JSONMode still renders through that feature's real
formatted template (FormatForTelegram) instead of falling back to a
raw-text dump. Every field is explicit that it's a placeholder — no
output here is presented as if it were a genuine recommendation.

The field names mirror the `json:"..."` tags in
internal/game/governor, internal/game/fleetcommander, and
internal/game/econadvisor. This package intentionally does NOT
import those packages (mock must stay a pure internal/ai provider
with no game dependency) — the shapes are duplicated here as plain
string literals, which is an acceptable small duplication for that
isolation. If a phase's JSON shape changes, its own tests will catch
a mismatch (ParseRecommendation will fail and fall back to raw
text) — update the corresponding case below to match.
~~~~


## estimateTokens is a rough (chars/4) heuristic used only so the mock

Removed from `internal/ai/providers/mock/provider.go:173`.

~~~~text
estimateTokens is a rough (chars/4) heuristic used only so the mock
provider exercises the cost-accounting path in tests and local dev;
it is never used for real billing decisions.
~~~~
