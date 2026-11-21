# ADR 0064: Decisions from `internal/bot/handlers/dev_console.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/dev_console.go` and moved here so the source no longer carries them. Each entry is the original comment.

## DevConsoleHandler exposes Phase J (AI Developer Console). New

Removed from `internal/bot/handlers/dev_console.go:18`.

~~~~text
DevConsoleHandler exposes Phase J (AI Developer Console). New
command (/weekly_report). Admin-only, using the exact same
AdminIDs/IsAdmin gate every other admin-only action in
internal/bot/handlers/admin.go already uses — this reports on the
whole game's player activity, not any one player's own data.
~~~~


## ── /weekly_report [days] ────────────────────────────────────────────

Removed from `internal/bot/handlers/dev_console.go:66`.

~~~~text
── /weekly_report [days] ────────────────────────────────────────────

Admin-only. Summarizes new player signups, top players, active
users, and recent world news over the given window (default 7
days). Read-only: never changes any player, setting, or game data.
~~~~


## ── callback: dev_console_refresh ────────────────────────────────────

Removed from `internal/bot/handlers/dev_console.go:97`.

~~~~text
── callback: dev_console_refresh ────────────────────────────────────

Re-runs the report for the same window on demand (a real new AI
Foundation call, subject to the usual cost/cache/budget rules).
~~~~


## ── /admin_ask <question> ────────────────────────────────────────────

Removed from `internal/bot/handlers/dev_console.go:127`.

~~~~text
── /admin_ask <question> ────────────────────────────────────────────

Admin-only. Natural-language question about game state, answered
via the classify → execute → answer flow in devconsole.Ask. See
devconsole/queries.go's doc comment for the safety model: the model
never writes or executes SQL, only picks from a fixed whitelist of
read-only queries.
~~~~


## ── /balance_report [days] ───────────────────────────────────────────

Removed from `internal/bot/handlers/dev_console.go:159`.

~~~~text
── /balance_report [days] ───────────────────────────────────────────

Admin-only. Real unit usage/outcome correlations from completed
raids, with AI commentary. See devconsole/balance.go's doc comment
for the correlational-not-causal framing this is built around.
~~~~
