# ADR 0103: Decisions from `internal/bot/handlers/research_planner.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/research_planner.go` and moved here so the source no longer carries them. Each entry is the original comment.

## ResearchPlannerHandler exposes Phase E (AI Research Planner). New

Removed from `internal/bot/handlers/research_planner.go:13`.

~~~~text
ResearchPlannerHandler exposes Phase E (AI Research Planner). New
command name (/research_planner) — no collision with any SpaceHunt
phase 1-6 command or the existing /research panel.
~~~~


## buildResearchPlannerKeyboard renders the inline keyboard attached to

Removed from `internal/bot/handlers/research_planner.go:32`.

~~~~text
buildResearchPlannerKeyboard renders the inline keyboard attached to
every Research Planner report: a refresh button (re-runs with
whatever goal was last used/inferred) and one button per goal so the
player can steer future recommendations without typing a command.
~~~~


## renderResearchPlannerReport runs a fresh recommendation and returns

Removed from `internal/bot/handlers/research_planner.go:52`.

~~~~text
renderResearchPlannerReport runs a fresh recommendation and returns
the formatted text plus its attached keyboard, shared by the
/research_planner command and both callbacks so the three can never
drift apart. goal may be empty (infer it).
~~~~


## ── /research_planner [goal] ────────────────────────────────────────

Removed from `internal/bot/handlers/research_planner.go:64`.

~~~~text
── /research_planner [goal] ────────────────────────────────────────

Analyzes the player's tech tree and Neuro Core stockpile, and
recommends the best research order for a stated or inferred
strategic goal (raiding, defense, economy, balanced). Read-only:
never spends Neuro Cores or upgrades any tech node on the player's
behalf — /research remains the only place that actually happens.
~~~~


## ── callback: research_refresh ──────────────────────────────────────

Removed from `internal/bot/handlers/research_planner.go:92`.

~~~~text
── callback: research_refresh ──────────────────────────────────────

Re-runs the analysis on demand with an inferred goal (a real new AI
Foundation call, subject to the usual cost/cache/budget rules) and
posts a fresh report.
~~~~


## ── callback: research_goal <goal> ──────────────────────────────────

Removed from `internal/bot/handlers/research_planner.go:116`.

~~~~text
── callback: research_goal <goal> ──────────────────────────────────

Re-runs the analysis pinned to whichever goal button the player
tapped, overriding inference for that one report.
~~~~
