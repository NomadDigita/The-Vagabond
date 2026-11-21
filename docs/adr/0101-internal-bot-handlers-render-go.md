# ADR 0101: Decisions from `internal/bot/handlers/render.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/render.go` and moved here so the source no longer carries them. Each entry is the original comment.

## formatDuration renders a duration the way a player actually thinks

Removed from `internal/bot/handlers/render.go:9`.

~~~~text
formatDuration renders a duration the way a player actually thinks
about time - "2h 15m" or "3d 6h", not "8100s" - by picking the two
largest non-zero units (seconds only appear on their own once under a
minute). Negative or zero durations always show "0s" rather than a
negative number, since every caller uses this for a countdown that's
either still running or has already elapsed.
~~~~


## This file holds small, dependency-free helpers used to render richer

Removed from `internal/bot/handlers/render.go:44`.

~~~~text
This file holds small, dependency-free helpers used to render richer
Telegram messages via HTML parse mode (bold/italic/underline/code/
blockquote) instead of plain text. Telegram's Bot API HTML mode only
recognizes a small whitelist of tags and requires the three reserved
characters "&", "<", ">" to be escaped in any text that isn't meant
to be a tag - that includes ALL user-supplied strings (descriptions,
custom names, feedback text, chat messages) since a raw "<" or "&" in
user text will make Telegram reject the whole message with a
"can't parse entities" 400 error. Game-generated strings built purely
from our own constants/numbers don't need escaping, but anything that
passed through user input MUST go through htmlEscape first.

Usage pattern for an existing plain c.Send(text) call site:

	return c.Send(htmlBold("TITLE")+"\n"+body, telebot.ModeHTML)

with any interpolated user text wrapped as htmlEscape(userText).
~~~~


## htmlEscape makes a string safe to place inside an HTML-parse-mode

Removed from `internal/bot/handlers/render.go:62`.

~~~~text
htmlEscape makes a string safe to place inside an HTML-parse-mode
Telegram message. Order matters: "&" must be escaped first, or it
would double-escape the "&" produced by escaping "<"/">".
~~~~


## resourceEmoji gives a consistent icon per resource/item type across

Removed from `internal/bot/handlers/render.go:101`.

~~~~text
resourceEmoji gives a consistent icon per resource/item type across
every panel that lists tradeable stock (Exchange, Vault, Silo,
Deconstruct, etc.) instead of each screen picking its own.
~~~~
