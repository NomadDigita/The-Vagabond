# ADR 0083: Decisions from `internal/bot/handlers/guild_assistant.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/guild_assistant.go` and moved here so the source no longer carries them. Each entry is the original comment.

## GuildAssistantHandler exposes Phase G (AI Guild Assistant). New

Removed from `internal/bot/handlers/guild_assistant.go:12`.

~~~~text
GuildAssistantHandler exposes Phase G (AI Guild Assistant). New
command (/guild_assistant) — no collision with any existing clan
command in internal/bot/handlers/clan.go.

Leader-only, unlike every prior Phase B-F command which any player
can use for their own base/fleet/economy/research/battles: this
feature reasons about clan-wide recruitment and war strategy, which
clan.go already treats as Leader-gated decisions (see
guildassistant/assistant.go's ErrNotLeader doc comment).
~~~~


## buildGuildAssistantKeyboard renders the inline keyboard attached to

Removed from `internal/bot/handlers/guild_assistant.go:29`.

~~~~text
buildGuildAssistantKeyboard renders the inline keyboard attached to
every Guild Assistant report: a single refresh button. Like Battle
Analyst, there's no goal selection here — the analysis covers
whatever the clan's roster/applicants/war state currently is, not
one steerable goal.
~~~~


## renderGuildAssistantReport runs a fresh analysis and returns the

Removed from `internal/bot/handlers/guild_assistant.go:41`.

~~~~text
renderGuildAssistantReport runs a fresh analysis and returns the
formatted text plus its attached keyboard, shared by the
/guild_assistant command and its refresh callback so the two can
never drift apart.
~~~~


## ── /guild_assistant ─────────────────────────────────────────────────

Removed from `internal/bot/handlers/guild_assistant.go:67`.

~~~~text
── /guild_assistant ─────────────────────────────────────────────────

Analyzes the Leader's clan: pending recruitment applicants, roster
health, and war record. Read-only: never accepts/rejects an
applicant, declares a war, or changes membership on the Leader's
behalf.
~~~~


## ── callback: guild_assistant_refresh ────────────────────────────────

Removed from `internal/bot/handlers/guild_assistant.go:89`.

~~~~text
── callback: guild_assistant_refresh ────────────────────────────────

Re-runs the analysis on demand (a real new AI Foundation call,
subject to the usual cost/cache/budget rules) and posts a fresh
report.
~~~~
