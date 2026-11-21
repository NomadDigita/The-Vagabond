# ADR 0191: Decisions from `internal/game/guildassistant/assistant.go`

Status: Accepted

These decisions were written as comments in `internal/game/guildassistant/assistant.go` and moved here so the source no longer carries them. Each entry is the original comment.

## MemoryScope namespaces this feature's conversational history inside

Removed from `internal/game/guildassistant/assistant.go:12`.

~~~~text
MemoryScope namespaces this feature's conversational history inside
ai_memory, distinct from every other Phase B+ scope.
~~~~


## memberCap mirrors the hard-coded 15-member cap enforced in

Removed from `internal/game/guildassistant/assistant.go:16`.

~~~~text
memberCap mirrors the hard-coded 15-member cap enforced in
HandleApplicationDecisionCallback (internal/bot/handlers/clan.go).
There's no config row for this today; if that ever becomes
data-driven, this constant needs to move with it.
~~~~


## inactivityLookbackDays flags a member as "inactive" for roster-health

Removed from `internal/game/guildassistant/assistant.go:22`.

~~~~text
inactivityLookbackDays flags a member as "inactive" for roster-health
purposes if users.last_active is older than this many days. Chosen
as a reasonable "hasn't logged in in over a week" signal — not tied
to any existing game rule, since none exists yet for guild activity
requirements.
~~~~


## ErrNotLeader is returned when the calling user is in a clan but

Removed from `internal/game/guildassistant/assistant.go:32`.

~~~~text
ErrNotLeader is returned when the calling user is in a clan but
isn't its Leader. Guild Assistant is Leader-only — see prompt.go's
doc comment for why.
~~~~


## buildWarRecord mirrors the exact active-war query already used by

Removed from `internal/game/guildassistant/assistant.go:123`.

~~~~text
buildWarRecord mirrors the exact active-war query already used by
HandleClanPanel, plus a durable win/loss tally from clan_wars rows
with status='completed' (never deleted — confirmed by grepping for
DELETE FROM clan_wars before writing this, the same audit discipline
Phase F used for its own tables).
~~~~


## Recommend produces a fresh AI analysis for the given Leader's clan.

Removed from `internal/game/guildassistant/assistant.go:175`.

~~~~text
Recommend produces a fresh AI analysis for the given Leader's clan.
It stores both turns in ai_memory under MemoryScope.

Read-only: nothing in this method accepts/rejects any applicant,
declares any war, or changes clan membership — it only reads and
recommends.
~~~~
