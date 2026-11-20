# ADR 0049: Decisions from `internal/bot/handlers/battle_analyst.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/battle_analyst.go` and moved here so the source no longer carries them. Each entry is the original comment.

## BattleAnalystHandler exposes Phase F (AI Battle Analyst). New

Removed from `internal/bot/handlers/battle_analyst.go:12`.

~~~~text
BattleAnalystHandler exposes Phase F (AI Battle Analyst). New
command name (/battle_analyst) — no collision with any SpaceHunt
phase 1-6 command or existing AI-roadmap commands.
~~~~


## buildBattleAnalystKeyboard renders the inline keyboard attached to

Removed from `internal/bot/handlers/battle_analyst.go:23`.

~~~~text
buildBattleAnalystKeyboard renders the inline keyboard attached to
every Battle Analyst report: a single refresh button. Unlike
Research Planner, there's no goal selection here — the analysis
covers the player's whole combat record, not one steerable goal.
~~~~


## renderBattleAnalystReport runs a fresh analysis and returns the

Removed from `internal/bot/handlers/battle_analyst.go:34`.

~~~~text
renderBattleAnalystReport runs a fresh analysis and returns the
formatted text plus its attached keyboard, shared by the
/battle_analyst command and its refresh callback so the two can
never drift apart.
~~~~


## ── /battle_analyst ──────────────────────────────────────────────────

Removed from `internal/bot/handlers/battle_analyst.go:46`.

~~~~text
── /battle_analyst ──────────────────────────────────────────────────

Analyzes the player's accumulated raid (attacker + defender) and
arena battle record, surfacing recurring patterns. Read-only: never
changes any raid, arena battle, or unit on the player's behalf.
~~~~


## ── callback: battle_analyst_refresh ─────────────────────────────────

Removed from `internal/bot/handlers/battle_analyst.go:70`.

~~~~text
── callback: battle_analyst_refresh ─────────────────────────────────

Re-runs the analysis on demand (a real new AI Foundation call,
subject to the usual cost/cache/budget rules) and posts a fresh
report.
~~~~
