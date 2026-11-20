# ADR 0200: Decisions from `internal/game/npcintel/prompt.go`

Status: Accepted

These decisions were written as comments in `internal/game/npcintel/prompt.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package npcintel implements Phase I of the AI Systems Roadmap: AI

Removed from `internal/game/npcintel/prompt.go:1`.

~~~~text
Package npcintel implements Phase I of the AI Systems Roadmap: AI
NPC Intelligence. Like every other Phase B-H package, this is the
seam where game state meets ai.Service; internal/ai itself stays
game-agnostic per ADR-001.

This file has zero I/O so it can be unit tested directly. All
database access lives in intel.go.

Scope, decided by reading the actual NPC/combat systems before
writing any Snapshot fields (the same discipline Phases F-H used):
the Rogue Drone Nest (internal/game/content.RogueNestComposition) is
the game's only NPC/hostile-AI entity today — there's no wandering
monster system, faction NPC roster, or NPC dialogue tree. Fleet
Commander (Phase C) already gives a binary attack/no-attack call
against the Nest. What it doesn't do is unpack *why* a specific
fleet composition wins or loses against a specific Nest composition
— the combat engine (internal/engine/tick/engine.go) has real,
mechanically-enforced hard counters (Destroyer/Wraith vs.
drones+jets, Bomber vs. turreted Defense Grids, Guardian vs.
Bomber, and per-turret-type scaling against specific attacker unit
mixes) that the static /recon_ai report shows as raw numbers but
never interprets. Phase I fills that specific, real gap: a
composition-specific tactical read of the Nest the player is about
to face, grounded in those actual counter mechanics — not a second
attack/no-attack call duplicating Fleet Commander.
~~~~


## NestProfile mirrors internal/game/content.RogueNestForce's shape —

Removed from `internal/game/npcintel/prompt.go:36`.

~~~~text
NestProfile mirrors internal/game/content.RogueNestForce's shape —
intentionally duplicated rather than importing that package's
struct directly, keeping this package's Snapshot independent of
content's internal layout the same way every other Phase B-H
package keeps its own independent DB-facing types.
~~~~


## FleetProfile is the player's own current mobile combat fleet

Removed from `internal/game/npcintel/prompt.go:62`.

~~~~text
FleetProfile is the player's own current mobile combat fleet
(garrison-only units like Guardians/Observers never leave the base
on a raid, so they're deliberately not part of this).
~~~~


## "bring_more" | "bring_less" | "fine"

Removed from `internal/game/npcintel/prompt.go:88`.

~~~~text
"bring_more" | "bring_less" | "fine"
~~~~


## Truncated is true when the fallback happened because the

Removed from `internal/game/npcintel/prompt.go:103`.

~~~~text
Truncated is true when the fallback happened because the
model's response was cut off mid-object (almost always meaning
MaxTokens was hit before the model finished). See ADR-016 in
PROJECT_MASTER_PLAN.md.
~~~~


## SystemPrompt is the fixed instruction given to the model for every

Removed from `internal/game/npcintel/prompt.go:110`.

~~~~text
SystemPrompt is the fixed instruction given to the model for every
NPC Intelligence call. The counter mechanics listed here are the
real, mechanically-enforced rules from internal/engine/tick/engine.go's
combat resolution — not flavor text — so the model's advice stays
grounded in what the combat engine will actually do.
~~~~


## ParseRecommendation decodes the model's response text, tolerating a

Removed from `internal/game/npcintel/prompt.go:171`.

~~~~text
ParseRecommendation decodes the model's response text, tolerating a
markdown code fence the same way every other Phase B-H package does.
~~~~


## See ADR-015: real providers occasionally leave a raw,

Removed from `internal/game/npcintel/prompt.go:184`.

~~~~text
See ADR-015: real providers occasionally leave a raw,
unescaped newline/tab inside a string value.
~~~~


## FormatForTelegram renders a Recommendation as a Telegram HTML-mode

Removed from `internal/game/npcintel/prompt.go:194`.

~~~~text
FormatForTelegram renders a Recommendation as a Telegram HTML-mode
message. All fields are LLM-generated text and are escaped via
ai.HTMLEscape before being wrapped in a tag - see internal/ai/render.go.
~~~~
