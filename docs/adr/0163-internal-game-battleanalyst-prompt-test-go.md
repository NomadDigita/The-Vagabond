# ADR 0163: Decisions from `internal/game/battleanalyst/prompt_test.go`

Status: Accepted

These decisions were written as comments in `internal/game/battleanalyst/prompt_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Defender-side stats have no meaningful "stolen value" concept

Removed from `internal/game/battleanalyst/prompt_test.go:59`.

~~~~text
Defender-side stats have no meaningful "stolen value" concept
(that's an attacker-only figure) — confirm the prompt doesn't
fabricate one for the defender section.
~~~~


## See ADR-016 in PROJECT_MASTER_PLAN.md: a response cut off mid-object

Removed from `internal/game/battleanalyst/prompt_test.go:129`.

~~~~text
See ADR-016 in PROJECT_MASTER_PLAN.md: a response cut off mid-object
is distinguished from one that never contained JSON at all.
~~~~


## The brace-balance heuristic alone can't see truncation in prose cut

Removed from `internal/game/battleanalyst/prompt_test.go:142`.

~~~~text
The brace-balance heuristic alone can't see truncation in prose cut
off before any '{' was written at all (TestParseRecommendation_
FallsBackOnGarbage above asserts exactly that blind spot). A
provider's real stop/finish reason doesn't have that limitation —
see ai.IsTruncatedStopReason — so passing it through should catch
what the text-only scan misses.
~~~~
