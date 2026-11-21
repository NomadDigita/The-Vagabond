# ADR 0178: Decisions from `internal/game/econadvisor/prompt.go`

Status: Accepted

These decisions were written as comments in `internal/game/econadvisor/prompt.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package econadvisor implements Phase D of the AI Systems Roadmap:

Removed from `internal/game/econadvisor/prompt.go:1`.

~~~~text
Package econadvisor implements Phase D of the AI Systems Roadmap:
the AI Economy Advisor. Like governor and fleetcommander, this
package is the seam where game state meets ai.Service; internal/ai
itself stays game-agnostic per ADR-001 in PROJECT_MASTER_PLAN.md.

This file has zero I/O so it can be unit tested directly. All
database access lives in advisor.go.

Note: this package deliberately does NOT import internal/game/governor
or internal/game/fleetcommander, even though all three read
overlapping data (resources, modules). Each stays independently
buildable/mergeable rather than forming a dependency chain between
sibling AI features — a small amount of duplicated type definition
(ModuleState) is a worthwhile trade for that isolation.
~~~~


## Resources is a generic name→amount map (scrap, rations,

Removed from `internal/game/econadvisor/prompt.go:55`.

~~~~text
Resources is a generic name→amount map (scrap, rations,
electricity, neuro_cores, metal, crystal, hydrogen, dollars,
ether, ...) so a new resource column never requires a change
to this struct's shape.
~~~~


## Action is one recommended step with a required quantitative

Removed from `internal/game/econadvisor/prompt.go:71`.

~~~~text
Action is one recommended step with a required quantitative
estimate, per the roadmap's "Explain expected gains quantitatively."
~~~~


## e.g. "+18% scrap/tick", "~450 dollars/day"

Removed from `internal/game/econadvisor/prompt.go:77`.

~~~~text
e.g. "+18% scrap/tick", "~450 dollars/day"
~~~~


## Truncated is true when the fallback happened because the

Removed from `internal/game/econadvisor/prompt.go:91`.

~~~~text
Truncated is true when the fallback happened because the
model's response was cut off mid-object (almost always meaning
MaxTokens was hit before the model finished). See ADR-016 in
PROJECT_MASTER_PLAN.md.
~~~~


## BuildUserPrompt renders a Snapshot into the data block the model

Removed from `internal/game/econadvisor/prompt.go:109`.

~~~~text
BuildUserPrompt renders a Snapshot into the data block the model
reasons over. Sorted map/slice iteration keeps output deterministic
for internal/ai's cache key hashing.
~~~~


## ParseRecommendation decodes the model's response text, tolerating a

Removed from `internal/game/econadvisor/prompt.go:166`.

~~~~text
ParseRecommendation decodes the model's response text, tolerating a
markdown code fence the same way governor/fleetcommander do.
~~~~


## See ADR-015: real providers occasionally leave a raw,

Removed from `internal/game/econadvisor/prompt.go:179`.

~~~~text
See ADR-015: real providers occasionally leave a raw,
unescaped newline/tab inside a string value.
~~~~


## FormatForTelegram renders a Recommendation as a Telegram HTML-mode

Removed from `internal/game/econadvisor/prompt.go:189`.

~~~~text
FormatForTelegram renders a Recommendation as a Telegram HTML-mode
message. All fields are LLM-generated text and are escaped via
ai.HTMLEscape before being wrapped in a tag - see internal/ai/render.go.
~~~~
