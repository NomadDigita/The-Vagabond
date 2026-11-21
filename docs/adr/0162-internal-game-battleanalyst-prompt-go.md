# ADR 0162: Decisions from `internal/game/battleanalyst/prompt.go`

Status: Accepted

These decisions were written as comments in `internal/game/battleanalyst/prompt.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package battleanalyst implements Phase F of the AI Systems Roadmap:

Removed from `internal/game/battleanalyst/prompt.go:1`.

~~~~text
Package battleanalyst implements Phase F of the AI Systems Roadmap:
the AI Battle Analyst. Like governor, fleetcommander, econadvisor,
and researchplanner, this package is the seam where game state meets
ai.Service; internal/ai itself stays game-agnostic per ADR-001 in
PROJECT_MASTER_PLAN.md.

This file has zero I/O so it can be unit tested directly. All
database access lives in analyst.go.

Unlike Fleet Commander (Phase C), which is forward-looking — "should
I attack this specific target right now?" — Battle Analyst is
backward-looking: it reviews a player's accumulated combat record
across raids (as both attacker and defender) and arena battles, and
surfaces recurring patterns the player might not notice from
scrolling through individual battle reports one at a time.

Scope note: this package deliberately does NOT analyze World Boss
engagements. See ADR-017 in PROJECT_MASTER_PLAN.md — a schema audit
done before writing this file (not assumed from an older session's
notes) confirmed world_boss_attacks rows are deleted once survivors
return home, and world_boss_contributions rows are deleted the
moment a boss is defeated. Neither table retains any history across
completed engagements, so there is nothing durable for this package
to read for World Bosses today.
~~~~


## RaidStats summarizes a player's completed raids from one side of the

Removed from `internal/game/battleanalyst/prompt.go:35`.

~~~~text
RaidStats summarizes a player's completed raids from one side of the
fight (attacker or defender). "Apparent" outcomes are the same
stolen-resources heuristic fleetcommander.BuildCombatHistory already
uses (raids has no explicit outcome column) — see that package's
doc comment for why this is flagged as approximate.
~~~~


## attacker side only; always 0 for defender stats

Removed from `internal/game/battleanalyst/prompt.go:45`.

~~~~text
attacker side only; always 0 for defender stats
~~~~


## ArenaStats summarizes a player's arena_battles rows, matched by

Removed from `internal/game/battleanalyst/prompt.go:48`.

~~~~text
ArenaStats summarizes a player's arena_battles rows, matched by
their current username (see analyst.go for the caveat this implies
about username changes).
~~~~


## Recommendation is the Battle Analyst's structured output. If the

Removed from `internal/game/battleanalyst/prompt.go:76`.

~~~~text
Recommendation is the Battle Analyst's structured output. If the
model's response can't be parsed as JSON, ParseRecommendation falls
back to Summary=<raw text>, matching every other Phase B-J package.
~~~~


## Truncated is true when the fallback happened because the

Removed from `internal/game/battleanalyst/prompt.go:88`.

~~~~text
Truncated is true when the fallback happened because the
model's response was cut off mid-object (almost always meaning
MaxTokens was hit before the model finished). See ADR-016 in
PROJECT_MASTER_PLAN.md.
~~~~


## ParseRecommendation decodes the model's response text, tolerating a

Removed from `internal/game/battleanalyst/prompt.go:150`.

~~~~text
ParseRecommendation decodes the model's response text, tolerating a
markdown code fence the same way every other Phase B-J package does.
~~~~


## See ADR-015: real providers occasionally leave a raw,

Removed from `internal/game/battleanalyst/prompt.go:163`.

~~~~text
See ADR-015: real providers occasionally leave a raw,
unescaped newline/tab inside a string value.
~~~~


## FormatForTelegram renders a Recommendation as a Telegram HTML-mode

Removed from `internal/game/battleanalyst/prompt.go:173`.

~~~~text
FormatForTelegram renders a Recommendation as a Telegram HTML-mode
message. All fields are LLM-generated text and are escaped via
ai.HTMLEscape before being wrapped in a tag - see internal/ai/render.go.
~~~~
