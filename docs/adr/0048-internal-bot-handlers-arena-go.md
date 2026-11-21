# ADR 0048: Decisions from `internal/bot/handlers/arena.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/arena.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Inline buttons and the persistent bottom keyboard need two

Removed from `internal/bot/handlers/arena.go:77`.

~~~~text
Inline buttons and the persistent bottom keyboard need two
separate messages - Telegram only allows one reply_markup type
per message, so they can never be combined into a single send.
~~~~
