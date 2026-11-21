# ADR 0053: Decisions from `internal/bot/handlers/camp.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/camp.go` and moved here so the source no longer carries them. Each entry is the original comment.

## defenseModule describes one Defense Grid structure. All of these ride on

Removed from `internal/bot/handlers/camp.go:146`.

~~~~text
defenseModule describes one Defense Grid structure. All of these ride on
the same generic `modules` table + HandleUpgradeCallback pipeline used by
tent/scrap_heap/generator, so adding a new turret type needs no new
schema or upgrade logic - just an entry here.
~~~~


## infrastructureModules mirrors defenseGridModules but for economy/utility

Removed from `internal/bot/handlers/camp.go:200`.

~~~~text
infrastructureModules mirrors defenseGridModules but for economy/utility
buildings. Same generic modules table + HandleUpgradeCallback pipeline -
no new upgrade logic needed, just new type strings.
~~~~
