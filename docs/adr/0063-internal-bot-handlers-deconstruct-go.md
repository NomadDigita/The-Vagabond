# ADR 0063: Decisions from `internal/bot/handlers/deconstruct.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/deconstruct.go` and moved here so the source no longer carries them. Each entry is the original comment.

## deconstructRefund describes what a single unit gives back when scrapped,

Removed from `internal/bot/handlers/deconstruct.go:25`.

~~~~text
deconstructRefund describes what a single unit gives back when scrapped,
at a flat 40% of its original build cost (see factory.go HandleCraftCallback).
~~~~


## HandleDeconstructCommand implements "/deconstruct <n> <unit>" as a bulk

Removed from `internal/bot/handlers/deconstruct.go:61`.

~~~~text
HandleDeconstructCommand implements "/deconstruct <n> <unit>" as a bulk
text shortcut alongside the existing one-tap-per-unit panel button. With
no arguments it falls back to the original panel view unchanged.
~~~~


## Handle known irregular plurals that a plain TrimSuffix("s") misses.

Removed from `internal/bot/handlers/deconstruct.go:89`.

~~~~text
Handle known irregular plurals that a plain TrimSuffix("s") misses.
~~~~
