# ADR 0046: Decisions from `internal/bot/handlers/advisors_panel.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/advisors_panel.go` and moved here so the source no longer carries them. Each entry is the original comment.

## HandleAdvisorsPanel plants the AdvisorsNavigation child keyboard and

Removed from `internal/bot/handlers/advisors_panel.go:32`.

~~~~text
HandleAdvisorsPanel plants the AdvisorsNavigation child keyboard and
shows a one-line summary of each advisor so a player can pick the
right one without guessing from the button label alone. Every button
on AdvisorsNavigation fires its advisor's report directly (same "leaf
action, no further sub-panel" shape as JobsNavigation) - each advisor
handler already renders its own full report with its own inline
refresh/detail buttons.
~~~~
