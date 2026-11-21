# ADR 0067: Decisions from `internal/bot/handlers/economy_advisor.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/economy_advisor.go` and moved here so the source no longer carries them. Each entry is the original comment.

## EconomyAdvisorHandler exposes Phase D (AI Economy Advisor). New

Removed from `internal/bot/handlers/economy_advisor.go:12`.

~~~~text
EconomyAdvisorHandler exposes Phase D (AI Economy Advisor). New
command name (/economy_advisor) — no collision with any SpaceHunt
phase 1-6 command.
~~~~


## ── /economy_advisor ──────────────────────────────────────────────────

Removed from `internal/bot/handlers/economy_advisor.go:38`.

~~~~text
── /economy_advisor ──────────────────────────────────────────────────

Analyzes the player's resources, buildings, bank debt, and market
activity, and recommends the highest-ROI next actions with
quantitative expected gains. Read-only: never buys, sells, or
upgrades anything on the player's behalf.
~~~~


## ── callback: econ_refresh ────────────────────────────────────────────

Removed from `internal/bot/handlers/economy_advisor.go:63`.

~~~~text
── callback: econ_refresh ────────────────────────────────────────────

Re-runs the same analysis on demand (a real new AI Foundation call,
subject to the usual cost/cache/budget rules) and posts a fresh report.
~~~~
