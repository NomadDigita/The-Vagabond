# ADR 0082: Decisions from `internal/bot/handlers/governor.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/governor.go` and moved here so the source no longer carries them. Each entry is the original comment.

## GovernorHandler exposes Phase B (AI Planet Governor) to players.

Removed from `internal/bot/handlers/governor.go:14`.

~~~~text
GovernorHandler exposes Phase B (AI Planet Governor) to players.
Command names (/governor, /governor_autopilot) are new and cannot
collide with any SpaceHunt phase 1-6 command. Callback unique names
(gov_refresh, gov_toggle_autopilot) follow the same convention as
existing inline buttons in internal/bot/handlers/combat.go.
~~~~


## renderGovernorReport runs a fresh recommendation and returns the

Removed from `internal/bot/handlers/governor.go:45`.

~~~~text
renderGovernorReport runs a fresh recommendation and returns the
formatted text plus its attached keyboard, shared by the /governor
command and the refresh callback so the two can never drift apart.
~~~~


## ── /governor ────────────────────────────────────────────────────────

Removed from `internal/bot/handlers/governor.go:56`.

~~~~text
── /governor ────────────────────────────────────────────────────────

Fetches the player's current base state, asks the AI Foundation for
a recommendation, and shows it with a refresh + autopilot-preference
keyboard. Read-only: never builds, upgrades, or queues anything on
the player's behalf.
~~~~


## ── callback: gov_refresh ────────────────────────────────────────────

Removed from `internal/bot/handlers/governor.go:81`.

~~~~text
── callback: gov_refresh ────────────────────────────────────────────

Re-runs the same analysis on demand and posts a fresh report. This
makes a new AI Foundation call each tap (subject to the same
cost/cache/budget rules as any other call — see internal/ai.Service),
so it is a real re-analysis, not a cached replay of the same text.
~~~~


## ── callback: gov_toggle_autopilot ──────────────────────────────────

Removed from `internal/bot/handlers/governor.go:106`.

~~~~text
── callback: gov_toggle_autopilot ──────────────────────────────────

Flips the player's stored autopilot preference and re-sends the
keyboard with an updated label. Per governor.SetAutopilot's doc
comment, this preference is not currently acted upon by any code
path — the callback response says so explicitly so the tap can
never be mistaken for enabling real automation.
~~~~


## ── /governor_autopilot <on|off> ─────────────────────────────────────

Removed from `internal/bot/handlers/governor.go:143`.

~~~~text
── /governor_autopilot <on|off> ─────────────────────────────────────

Text-command equivalent of the toggle button, for players who prefer
typing a command over tapping. Intentionally does NOT cause any
autonomous action yet (see PROJECT_MASTER_PLAN.md §4).
~~~~
