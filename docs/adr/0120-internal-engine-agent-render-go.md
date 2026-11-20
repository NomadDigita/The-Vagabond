# ADR 0120: Decisions from `internal/engine/agent/render.go`

Status: Accepted

These decisions were written as comments in `internal/engine/agent/render.go` and moved here so the source no longer carries them. Each entry is the original comment.

## htmlEscapeAgent makes a string safe to place inside a Telegram

Removed from `internal/engine/agent/render.go:5`.

~~~~text
htmlEscapeAgent makes a string safe to place inside a Telegram
HTML-mode message. Duplicated locally (rather than imported from
bot/handlers or internal/engine/tick) to keep this package
dependency-free, per the repo's package-per-feature convention. Any
player-set name (encampment, hero, clan) interpolated into a
notification message MUST go through this first.
~~~~


## htmlBoldAgent/htmlCodeAgent wrap already-safe text in the matching

Removed from `internal/engine/agent/render.go:16`.

~~~~text
htmlBoldAgent/htmlCodeAgent wrap already-safe text in the matching
Telegram HTML tag. Duplicated locally (see htmlEscapeAgent doc) to
keep this package dependency-free.
~~~~
