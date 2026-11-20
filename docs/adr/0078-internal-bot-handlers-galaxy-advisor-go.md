# ADR 0078: Decisions from `internal/bot/handlers/galaxy_advisor.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/galaxy_advisor.go` and moved here so the source no longer carries them. Each entry is the original comment.

## GalaxyAdvisorHandler exposes Phase H (AI Dynamic Galaxy). New

Removed from `internal/bot/handlers/galaxy_advisor.go:12`.

~~~~text
GalaxyAdvisorHandler exposes Phase H (AI Dynamic Galaxy). New
command (/galaxy_advisor) — no collision with any existing world
command (/world_feed, /sector_map, etc. in internal/bot/handlers/world.go).
~~~~


## buildGalaxyAdvisorKeyboard renders the inline keyboard attached to

Removed from `internal/bot/handlers/galaxy_advisor.go:23`.

~~~~text
buildGalaxyAdvisorKeyboard renders the inline keyboard attached to
every Galaxy Advisor briefing: a single refresh button. Like Battle
Analyst and Guild Assistant, there's no goal selection — this
briefing covers whatever the galaxy's current environmental state
is, not one steerable goal.
~~~~


## renderGalaxyAdvisorReport runs a fresh briefing and returns the

Removed from `internal/bot/handlers/galaxy_advisor.go:35`.

~~~~text
renderGalaxyAdvisorReport runs a fresh briefing and returns the
formatted text plus its attached keyboard, shared by the
/galaxy_advisor command and its refresh callback so the two can
never drift apart.
~~~~


## ── /galaxy_advisor ──────────────────────────────────────────────────

Removed from `internal/bot/handlers/galaxy_advisor.go:47`.

~~~~text
── /galaxy_advisor ──────────────────────────────────────────────────

Briefs the player on their home continent's current world event and
the wider galaxy's environmental state. Read-only: never moves a
fleet, queues construction, or changes any world event itself.
~~~~


## ── callback: galaxy_advisor_refresh ─────────────────────────────────

Removed from `internal/bot/handlers/galaxy_advisor.go:71`.

~~~~text
── callback: galaxy_advisor_refresh ─────────────────────────────────

Re-runs the briefing on demand (a real new AI Foundation call,
subject to the usual cost/cache/budget rules) and posts a fresh
report.
~~~~
