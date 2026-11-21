# ADR 0192: Decisions from `internal/game/guildassistant/prompt.go`

Status: Accepted

These decisions were written as comments in `internal/game/guildassistant/prompt.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package guildassistant implements Phase G of the AI Systems Roadmap:

Removed from `internal/game/guildassistant/prompt.go:1`.

~~~~text
Package guildassistant implements Phase G of the AI Systems Roadmap:
the AI Guild Assistant. Like every other Phase B-F package, this is
the seam where game state meets ai.Service; internal/ai itself stays
game-agnostic per ADR-001.

This file has zero I/O so it can be unit tested directly. All
database access lives in assistant.go.

Scope, decided by reading the real "clans" system (the SpaceHunt
Phase 2 branch's guild feature, see migrations/017) before writing
any Snapshot fields — the same discipline Phase F used for its own
schema audit:
  - Clan roster health: member count vs. the 15-member cap, combined
    level, combined military power (soldiers/mechs — the same
    formula HandleAllianceStatsCallback already uses), and a count
    of members who haven't been active recently.
  - Pending recruitment applications, each with the applicant's
    current outpost level, so the model can reason about fit rather
    than just a name.
  - Clan war record: durable history (clan_wars rows are never
    deleted — confirmed by grepping for DELETE FROM clan_wars,
    exactly as Phase F did for its own tables) plus the current
    active war's live score if one is underway.

This package is deliberately Leader-facing only (see assistant.go):
every other clan-leadership action in internal/bot/handlers/clan.go
(accepting/rejecting applicants, declaring war, kicking members) is
already Leader-gated, and recruitment/war strategy is exactly that
kind of decision — a regular member asking this would be reading
information about applicants and strategy that isn't theirs to act
on.
~~~~


## InActiveWar, OpponentName, OurScore, TheirScore describe the

Removed from `internal/game/guildassistant/prompt.go:55`.

~~~~text
InActiveWar, OpponentName, OurScore, TheirScore describe the
clan's current war if status='active' right now (at most one,
per HandleDeclareClanWarCallback's own "already at war" guard).
~~~~


## no last_active within the lookback window used by assistant.go

Removed from `internal/game/guildassistant/prompt.go:74`.

~~~~text
no last_active within the lookback window used by assistant.go
~~~~


## "accept" | "reject" | "undecided"

Removed from `internal/game/guildassistant/prompt.go:83`.

~~~~text
"accept" | "reject" | "undecided"
~~~~


## Truncated is true when the fallback happened because the

Removed from `internal/game/guildassistant/prompt.go:98`.

~~~~text
Truncated is true when the fallback happened because the
model's response was cut off mid-object (almost always meaning
MaxTokens was hit before the model finished). See ADR-016 in
PROJECT_MASTER_PLAN.md.
~~~~


## ParseRecommendation decodes the model's response text, tolerating a

Removed from `internal/game/guildassistant/prompt.go:155`.

~~~~text
ParseRecommendation decodes the model's response text, tolerating a
markdown code fence the same way every other Phase B-F package does.
~~~~


## See ADR-015: real providers occasionally leave a raw,

Removed from `internal/game/guildassistant/prompt.go:168`.

~~~~text
See ADR-015: real providers occasionally leave a raw,
unescaped newline/tab inside a string value.
~~~~


## FormatForTelegram renders a Recommendation as a Telegram HTML-mode

Removed from `internal/game/guildassistant/prompt.go:178`.

~~~~text
FormatForTelegram renders a Recommendation as a Telegram HTML-mode
message. All fields are LLM-generated text (plus applicant usernames,
which are real player-controlled data) and are escaped via
ai.HTMLEscape before being wrapped in a tag - see internal/ai/render.go.
~~~~
