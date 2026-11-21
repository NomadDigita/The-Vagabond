# ADR 0174: Decisions from `internal/game/devconsole/prompt.go`

Status: Accepted

These decisions were written as comments in `internal/game/devconsole/prompt.go` and moved here so the source no longer carries them. Each entry is the original comment.

## may be "" — Telegram username is optional

Removed from `internal/game/devconsole/prompt.go:42`.

~~~~text
may be "" — Telegram username is optional
~~~~


## pre-formatted (e.g. "2026-07-15 14:32 UTC") — see console.go

Removed from `internal/game/devconsole/prompt.go:44`.

~~~~text
pre-formatted (e.g. "2026-07-15 14:32 UTC") — see console.go
~~~~


## "" if the player never finished onboarding (no encampment yet)

Removed from `internal/game/devconsole/prompt.go:45`.

~~~~text
"" if the player never finished onboarding (no encampment yet)
~~~~


## TopPlayer is one entry from the existing Global Ranking (see

Removed from `internal/game/devconsole/prompt.go:48`.

~~~~text
TopPlayer is one entry from the existing Global Ranking (see
internal/game/scoring.ScoreExpr — the same formula
HandleRankingPanel already uses, so this never disagrees with what
players see on that screen).
~~~~


## capped — see console.go's newPlayerListCap

Removed from `internal/game/devconsole/prompt.go:62`.

~~~~text
capped — see console.go's newPlayerListCap
~~~~


## true total, even if NewPlayers was capped for prompt size

Removed from `internal/game/devconsole/prompt.go:63`.

~~~~text
true total, even if NewPlayers was capped for prompt size
~~~~


## users with last_active within the window

Removed from `internal/game/devconsole/prompt.go:68`.

~~~~text
users with last_active within the window
~~~~


## most-recent-first, capped — see console.go

Removed from `internal/game/devconsole/prompt.go:70`.

~~~~text
most-recent-first, capped — see console.go
~~~~


## Snapshot is the real, deterministic window data (see

Removed from `internal/game/devconsole/prompt.go:83`.

~~~~text
Snapshot is the real, deterministic window data (see
BalanceRecommendation.Snapshot for the same rationale) - set by
the caller after parsing, not unmarshaled from the model's JSON.
FormatForTelegram uses it to render an actual Top Players table
from real scores rather than trusting the model to transcribe
them into TopPerformerNarrative's prose correctly.
~~~~


## Truncated is true when the fallback happened because the

Removed from `internal/game/devconsole/prompt.go:94`.

~~~~text
Truncated is true when the fallback happened because the
model's response was cut off mid-object (almost always meaning
MaxTokens was hit before the model finished). See ADR-016 in
PROJECT_MASTER_PLAN.md.
~~~~


## ParseRecommendation decodes the model's response text, tolerating a

Removed from `internal/game/devconsole/prompt.go:166`.

~~~~text
ParseRecommendation decodes the model's response text, tolerating a
markdown code fence the same way every other Phase B-I package does.
~~~~


## See ADR-015: real providers occasionally leave a raw,

Removed from `internal/game/devconsole/prompt.go:179`.

~~~~text
See ADR-015: real providers occasionally leave a raw,
unescaped newline/tab inside a string value.
~~~~


## FormatForTelegram renders a Recommendation as a Telegram HTML-mode

Removed from `internal/game/devconsole/prompt.go:189`.

~~~~text
FormatForTelegram renders a Recommendation as a Telegram HTML-mode
message. All fields are LLM-generated text and are escaped via
ai.HTMLEscape before being wrapped in a tag - see internal/ai/render.go.
~~~~
