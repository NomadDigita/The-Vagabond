# ADR 0151: Decisions from `internal/engine/tick/render.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/render.go` and moved here so the source no longer carries them. Each entry is the original comment.

## htmlEscapeTick makes a string safe to place inside a Telegram

Removed from `internal/engine/tick/render.go:42`.

~~~~text
htmlEscapeTick makes a string safe to place inside a Telegram
HTML-mode message. Duplicated locally (rather than imported from
bot/handlers or battlereport) to keep this package dependency-free,
per the repo's package-per-feature convention. Any player-set name
(encampment, hero, clan) interpolated into a notification message
MUST go through this first.
~~~~


## htmlBoldTick/htmlItalicTick/htmlCodeTick wrap already-safe text in the

Removed from `internal/engine/tick/render.go:53`.

~~~~text
htmlBoldTick/htmlItalicTick/htmlCodeTick wrap already-safe text in the
matching Telegram HTML tag. Duplicated locally (see htmlEscapeTick doc)
to keep this package dependency-free.
~~~~
