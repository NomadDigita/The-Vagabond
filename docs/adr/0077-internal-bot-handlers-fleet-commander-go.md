# ADR 0077: Decisions from `internal/bot/handlers/fleet_commander.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/fleet_commander.go` and moved here so the source no longer carries them. Each entry is the original comment.

## FleetCommanderHandler exposes Phase C (AI Fleet Commander). New

Removed from `internal/bot/handlers/fleet_commander.go:12`.

~~~~text
FleetCommanderHandler exposes Phase C (AI Fleet Commander). New
command name (/fleet_commander) — no collision with any SpaceHunt
phase 1-6 command, and distinct from combat.go's existing
HandleReconAICallback, which is a static templated report, not an
LLM call.
~~~~


## ── /fleet_commander ─────────────────────────────────────────────────

Removed from `internal/bot/handlers/fleet_commander.go:40`.

~~~~text
── /fleet_commander ─────────────────────────────────────────────────

Analyzes the player's fleet against the rogue-nest PvE target scaled
to their level (see PROJECT_MASTER_PLAN.md — PvP target support is
future work) plus their recent raid history, and returns one of:
attack, retreat, reinforce, scout, wait, split_fleet — with
reasoning. Read-only: never launches anything.
~~~~


## ── callback: fleet_refresh ──────────────────────────────────────────

Removed from `internal/bot/handlers/fleet_commander.go:66`.

~~~~text
── callback: fleet_refresh ──────────────────────────────────────────

Re-runs the same analysis on demand (a real new AI Foundation call,
subject to the usual cost/cache/budget rules) and posts a fresh report.
~~~~
