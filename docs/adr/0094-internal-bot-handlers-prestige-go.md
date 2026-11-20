# ADR 0094: Decisions from `internal/bot/handlers/prestige.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/prestige.go` and moved here so the source no longer carries them. Each entry is the original comment.

## PrestigeHandler is the panel/confirm side of

Removed from `internal/bot/handlers/prestige.go:14`.

~~~~text
PrestigeHandler is the panel/confirm side of
NEXT_GENERATION_FEATURES_PLAN.md Phase 4 - eligibility checking and
the actual reset live in internal/game/prestige.
~~~~


## HandlePrestigePanel (/prestige) shows the current Prestige level and

Removed from `internal/bot/handlers/prestige.go:25`.

~~~~text
HandlePrestigePanel (/prestige) shows the current Prestige level and
cumulative bonus, an eligibility check with a clear "what's missing"
breakdown if not yet eligible, and a Prestige button (behind a
confirm dialog - see HandlePrestigeConfirmCallback) once eligible.
~~~~


## HandlePrestigeConfirmPromptCallback shows a styled Confirm/Cancel

Removed from `internal/bot/handlers/prestige.go:73`.

~~~~text
HandlePrestigeConfirmPromptCallback shows a styled Confirm/Cancel
dialog making unmistakably clear what resets and what's kept -
Prestige is irreversible by design, so this confirmation has to be
real, matching this project's established Confirm/Cancel-with-cost-
preview convention exactly.
~~~~
