# ADR 0184: Decisions from `internal/game/galaxyadvisor/prompt.go`

Status: Accepted

These decisions were written as comments in `internal/game/galaxyadvisor/prompt.go` and moved here so the source no longer carries them. Each entry is the original comment.

## "nominal" if clear — matches world.ActiveEventFor's own convention

Removed from `internal/game/galaxyadvisor/prompt.go:40`.

~~~~text
"nominal" if clear — matches world.ActiveEventFor's own convention
~~~~


## ordered to match world.Continents

Removed from `internal/game/galaxyadvisor/prompt.go:47`.

~~~~text
ordered to match world.Continents
~~~~


## most-recent-first, from world_news

Removed from `internal/game/galaxyadvisor/prompt.go:48`.

~~~~text
most-recent-first, from world_news
~~~~


## Truncated is true when the fallback happened because the

Removed from `internal/game/galaxyadvisor/prompt.go:68`.

~~~~text
Truncated is true when the fallback happened because the
model's response was cut off mid-object (almost always meaning
MaxTokens was hit before the model finished). See ADR-016 in
PROJECT_MASTER_PLAN.md.
~~~~


## eventEffect gives the mechanical-effect description for a

Removed from `internal/game/galaxyadvisor/prompt.go:75`.

~~~~text
eventEffect gives the mechanical-effect description for a
world_events event_type, matching internal/engine/world/weather.go's
own eventHeadline text. Deliberately duplicated rather than shared —
weather.go's eventHeadline is unexported, and this codebase's
existing pattern (see that function's own doc comment pointing at
internal/bot/handlers/world.go's separate in-panel text) is to keep
each consumer's copy of "what this event means" independently
readable rather than adding a cross-package dependency for what's
fundamentally presentation text. If a new event type is added to
weather.go's eventPool, add it here too.
~~~~


## ParseRecommendation decodes the model's response text, tolerating a

Removed from `internal/game/galaxyadvisor/prompt.go:152`.

~~~~text
ParseRecommendation decodes the model's response text, tolerating a
markdown code fence the same way every other Phase B-G package does.
~~~~


## See ADR-015: real providers occasionally leave a raw,

Removed from `internal/game/galaxyadvisor/prompt.go:165`.

~~~~text
See ADR-015: real providers occasionally leave a raw,
unescaped newline/tab inside a string value.
~~~~


## FormatForTelegram renders a Recommendation as a Telegram HTML-mode

Removed from `internal/game/galaxyadvisor/prompt.go:175`.

~~~~text
FormatForTelegram renders a Recommendation as a Telegram HTML-mode
message. All fields are LLM-generated text and are escaped via
ai.HTMLEscape before being wrapped in a tag - see internal/ai/render.go.
~~~~
