# ADR 0207: Decisions from `internal/game/researchplanner/prompt.go`

Status: Accepted

These decisions were written as comments in `internal/game/researchplanner/prompt.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package researchplanner implements Phase E of the AI Systems Roadmap:

Removed from `internal/game/researchplanner/prompt.go:1`.

~~~~text
Package researchplanner implements Phase E of the AI Systems Roadmap:
the AI Research Planner. Like governor, fleetcommander, and
econadvisor, this package is the seam where game state meets
ai.Service; internal/ai itself stays game-agnostic per ADR-001 in
PROJECT_MASTER_PLAN.md.

This file has zero I/O so it can be unit tested directly. All
database access lives in planner.go.

Note: this package deliberately does NOT import
internal/bot/handlers (which owns the live /research panel and its
upgrade-spend logic) or any sibling AI package. The 7-node tech
tree's shape (key, db column, cost formula, max level) is a small,
deliberately duplicated copy of internal/bot/handlers/research.go's
researchTree — same trade-off already made for the mock provider's
placeholder JSON (see internal/ai/providers/mock/provider.go): each
AI feature package stays independently buildable/mergeable rather
than forming a dependency chain. If the real tech tree ever changes
(a node added/removed, cost formula changed), update both copies —
this package's own tests (TestResearchNodes_MatchLiveHandler-style
assertions in prompt_test.go) exist to catch drift.
~~~~


## MaxResearchLevel mirrors handlers.MaxResearchLevel. Kept as an

Removed from `internal/game/researchplanner/prompt.go:33`.

~~~~text
MaxResearchLevel mirrors handlers.MaxResearchLevel. Kept as an
independent constant per the package doc comment above.
~~~~


## ValidGoals lists every goal accepted from a callback/command

Removed from `internal/game/researchplanner/prompt.go:48`.

~~~~text
ValidGoals lists every goal accepted from a callback/command
argument. Anything else falls back to inference.
~~~~


## TechNode is a minimal projection of one branch of the real 7-node

Removed from `internal/game/researchplanner/prompt.go:64`.

~~~~text
TechNode is a minimal projection of one branch of the real 7-node
tech tree (see handlers.researchTree). Order here matches the live
panel's display order.
~~~~


## matches handlers.researchNode.key / dbColumn suffix

Removed from `internal/game/researchplanner/prompt.go:68`.

~~~~text
matches handlers.researchNode.key / dbColumn suffix
~~~~


## Neuro Cores to advance from Level to Level+1; 0 if maxed

Removed from `internal/game/researchplanner/prompt.go:72`.

~~~~text
Neuro Cores to advance from Level to Level+1; 0 if maxed
~~~~


## techMeta is the static (key, title, description) triple for each of

Removed from `internal/game/researchplanner/prompt.go:76`.

~~~~text
techMeta is the static (key, title, description) triple for each of
the 7 real tech nodes, in the live panel's display order.
~~~~


## TechNodeCost returns the Neuro Core cost to advance from currentLvl

Removed from `internal/game/researchplanner/prompt.go:90`.

~~~~text
TechNodeCost returns the Neuro Core cost to advance from currentLvl
to currentLvl+1 — the same linear formula (level × 8) as
handlers.researchCost.
~~~~


## BuildTechNodes turns a raw key→level map (as returned by the live

Removed from `internal/game/researchplanner/prompt.go:97`.

~~~~text
BuildTechNodes turns a raw key→level map (as returned by the live
research_states row) into the ordered []TechNode the prompt and
Telegram renderer both use.
~~~~


## RequestedGoal is what the player explicitly picked, if

Removed from `internal/game/researchplanner/prompt.go:125`.

~~~~text
RequestedGoal is what the player explicitly picked, if
anything; empty means "infer it."
~~~~


## InferredGoal returns RequestedGoal if the player set one, otherwise

Removed from `internal/game/researchplanner/prompt.go:130`.

~~~~text
InferredGoal returns RequestedGoal if the player set one, otherwise
infers a reasonable default from the current tech spread: whichever
non-maxed node cluster is furthest behind the others gets
prioritized as "balanced" catch-up, unless every combat-adjacent
node (integrity/defense/military) is already ahead of
econ/production, in which case "economy" is inferred to avoid
recommending more of what's already ahead.
~~~~


## A meaningful (not noise-level) lead one way or the other

Removed from `internal/game/researchplanner/prompt.go:159`.

~~~~text
A meaningful (not noise-level) lead one way or the other
nudges the inferred goal toward evening things out.
~~~~


## Truncated is true when the fallback happened because the

Removed from `internal/game/researchplanner/prompt.go:193`.

~~~~text
Truncated is true when the fallback happened because the
model's response was cut off mid-object (almost always meaning
MaxTokens was hit before the model finished). See ADR-016 in
PROJECT_MASTER_PLAN.md.
~~~~


## BuildUserPrompt renders a Snapshot into the data block the model

Removed from `internal/game/researchplanner/prompt.go:218`.

~~~~text
BuildUserPrompt renders a Snapshot into the data block the model
reasons over. Sorted/stable iteration keeps output deterministic for
internal/ai's cache key hashing.
~~~~


## ParseRecommendation decodes the model's response text, tolerating a

Removed from `internal/game/researchplanner/prompt.go:245`.

~~~~text
ParseRecommendation decodes the model's response text, tolerating a
markdown code fence the same way governor/fleetcommander/econadvisor
do.
~~~~


## See ADR-015: real providers occasionally leave a raw,

Removed from `internal/game/researchplanner/prompt.go:259`.

~~~~text
See ADR-015: real providers occasionally leave a raw,
unescaped newline/tab inside a string value.
~~~~


## FormatForTelegram renders a Recommendation as a Telegram HTML-mode

Removed from `internal/game/researchplanner/prompt.go:269`.

~~~~text
FormatForTelegram renders a Recommendation as a Telegram HTML-mode
message. All fields are LLM-generated text and are escaped via
ai.HTMLEscape before being wrapped in a tag - see internal/ai/render.go.
~~~~
