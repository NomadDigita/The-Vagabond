# ADR 0110: Decisions from `internal/bot/handlers/world.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/world.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Phase 7 (item 12): world events are per-continent now, so this

Removed from `internal/bot/handlers/world.go:33`.

~~~~text
Phase 7 (item 12): world events are per-continent now, so this
feed shows a line per continent instead of one global status.
~~~~


## 3. Write dynamic dispatch to global world_news radio bulletin feed.

Removed from `internal/bot/handlers/world.go:206`.

~~~~text
3. Write dynamic dispatch to global world_news radio bulletin feed.
No htmlEscape here: HandleWorldFeed already htmlEscape()s the entire
aggregated news feed text at render time, so a raw player name/message
is safe by the time it reaches a screen (same pattern documented in
starvation.go's render.go).
~~~~
