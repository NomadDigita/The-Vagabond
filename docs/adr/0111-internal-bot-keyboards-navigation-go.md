# ADR 0111: Decisions from `internal/bot/keyboards/navigation.go`

Status: Accepted

These decisions were written as comments in `internal/bot/keyboards/navigation.go` and moved here so the source no longer carries them. Each entry is the original comment.

## BUGFIX: handlers.HandleSiloPanel (nuke/piercing missile launches)

Removed from `internal/bot/keyboards/navigation.go:73`.

~~~~text
BUGFIX: handlers.HandleSiloPanel (nuke/piercing missile launches)
was fully implemented and registered at bot.Handle("☢️ Strategic
Silo", ...) in main.go, but no menu.Text() anywhere ever rendered
that exact string - a systematic audit (every bot.Handle text
button vs every menu.Text() call across keyboards/ and handlers/)
found this was the only combat-facing dead button. The only way
to reach it was typing /silo blind. Nukes and Piercing Missiles
were never actually unlaunchable - the launch panel was just
unreachable.
~~~~


## BUGFIX: same systematic audit as CombatNavigation's Strategic Silo

Removed from `internal/bot/keyboards/navigation.go:106`.

~~~~text
BUGFIX: same systematic audit as CombatNavigation's Strategic Silo
button above - handlers.HandleWarehouseReserves (a straightforward
resource-totals panel) was registered at bot.Handle("📦 Warehouse
Reserves", ...) but never rendered on any menu.
~~~~


## JobsNavigation builds the custom submenu for one-off "Odd Jobs" utility

Removed from `internal/bot/keyboards/navigation.go:141`.

~~~~text
JobsNavigation builds the custom submenu for one-off "Odd Jobs" utility
actions (previously slash-command-only, with no button discoverability
at all - every other section in this game has had a mother/child
keyboard pair since the original navigation.go; Jobs was the one
exception). Every button here fires its job directly rather than
opening a further sub-panel, matching how a leaf action inside any
other child menu (e.g. Admin's "⚡ Force Master Tick") behaves - no
extra reply-keyboard replant happens on tap, so this same keyboard
just stays on screen between jobs.

Deliberately excludes jobs.go's four *Alias handlers
(HandleManualScanAlias/HandleAutoScanAlias/HandleAdvancedScanAlias/
HandlePublishTradeAlias) - those aren't real actions, just text
pointers telling the player which *other* panel to use instead, and
would read as broken buttons here ("tap this button to be told to go
tap a different button elsewhere").
~~~~


## AdvisorsNavigation builds the custom submenu for the AI Advisory Corps

Removed from `internal/bot/keyboards/navigation.go:181`.

~~~~text
AdvisorsNavigation builds the custom submenu for the AI Advisory Corps
(PROJECT_MASTER_PLAN.md's Phase E-J roadmap - 8 distinct AI advisor
personas that were entirely slash-command-only before this, with zero
button discoverability anywhere in the game, not even from the
thematically-adjacent "🧠 Automation Agent" panel). Same "leaf action,
no further sub-panel" shape as JobsNavigation - every button here
renders its advisor's full report directly.
~~~~


## ProfileNavigation builds the custom submenu for player-facing info

Removed from `internal/bot/keyboards/navigation.go:212`.

~~~~text
ProfileNavigation builds the custom submenu for player-facing info
panels and settings. Every button here was slash-command-only before
this - notably /settings itself, which had zero button access despite
being where the route_status notification mute toggle actually lives.
~~~~
