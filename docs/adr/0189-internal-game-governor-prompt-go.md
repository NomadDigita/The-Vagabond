# ADR 0189: Decisions from `internal/game/governor/prompt.go`

Status: Accepted

These decisions were written as comments in `internal/game/governor/prompt.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package governor implements Phase B of the AI Systems Roadmap: the

Removed from `internal/game/governor/prompt.go:1`.

~~~~text
Package governor implements Phase B of the AI Systems Roadmap: the
AI Planet Governor. Unlike internal/ai (which must stay game-agnostic
per ADR-001 in PROJECT_MASTER_PLAN.md), this package is allowed to
know about encampments, modules, and resources — it is the seam
where game state meets the AI Foundation.

This file intentionally contains zero I/O (no *sql.DB, no network)
so it can be unit tested directly. All database access lives in
governor.go.
~~~~


## Snapshot is everything about one player's base that the Governor

Removed from `internal/game/governor/prompt.go:27`.

~~~~text
Snapshot is everything about one player's base that the Governor
feeds to the model. It deliberately mirrors existing DB concepts
(see internal/engine/resource.EncampmentState) rather than inventing
a parallel data model.
~~~~


## Recommendation is the Governor's structured output. If the model's

Removed from `internal/game/governor/prompt.go:62`.

~~~~text
Recommendation is the Governor's structured output. If the model's
response can't be parsed as JSON (see ADR-005 in
PROJECT_MASTER_PLAN.md — Anthropic has no enforced JSON mode),
ParseRecommendation falls back to Summary=<raw text>, so the player
always sees *something* useful rather than an error.
~~~~


## Truncated is true when the fallback happened because the

Removed from `internal/game/governor/prompt.go:75`.

~~~~text
Truncated is true when the fallback happened because the
model's response was cut off mid-object (almost always meaning
MaxTokens was hit before the model finished), as opposed to the
response never containing a recognizable JSON object at all.
See ADR-016 in PROJECT_MASTER_PLAN.md.
~~~~


## SystemPrompt is the fixed instruction given to the model for every

Removed from `internal/game/governor/prompt.go:83`.

~~~~text
SystemPrompt is the fixed instruction given to the model for every
Governor call. It stays outside the per-call BuildUserPrompt so it's
easy to find and tune independent of the encampment data.
~~~~


## BuildUserPrompt renders a Snapshot into the human-readable data

Removed from `internal/game/governor/prompt.go:99`.

~~~~text
BuildUserPrompt renders a Snapshot into the human-readable data
block the model reasons over. Kept deterministic (sorted module
list) so identical snapshots produce identical prompts, which
matters for the AI Foundation's cache layer (internal/ai.CacheKey
hashes the full message list).
~~~~


## ParseRecommendation decodes the model's response text. It tolerates

Removed from `internal/game/governor/prompt.go:131`.

~~~~text
ParseRecommendation decodes the model's response text. It tolerates
responses wrapped in markdown code fences (some models add them
despite instructions not to) by stripping a leading/trailing ```
fence before attempting to parse.
~~~~


## Second attempt: real providers occasionally leave a raw,

Removed from `internal/game/governor/prompt.go:146`.

~~~~text
Second attempt: real providers occasionally leave a raw,
unescaped newline/tab inside a string value (most often in a
multi-sentence "summary"), which Go's strict encoding/json
rejects even though the model's intent was clearly valid JSON.
See ADR-015.
~~~~


## FormatForTelegram renders a Recommendation as a Telegram HTML-mode

Removed from `internal/game/governor/prompt.go:159`.

~~~~text
FormatForTelegram renders a Recommendation as a Telegram HTML-mode
message. Every field is LLM-generated text, so it's escaped via
ai.HTMLEscape before being wrapped in a tag - see internal/ai/render.go.
~~~~
