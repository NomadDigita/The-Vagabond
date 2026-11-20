# ADR 0112: Decisions from `internal/bot/keyboards/navigation_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/keyboards/navigation_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## allowedSharedText is the small set of button labels that are

Removed from `internal/bot/keyboards/navigation_test.go:9`.

~~~~text
allowedSharedText is the small set of button labels that are
deliberately identical across multiple keyboards, because they're
meant to route to the exact same handler regardless of which child
menu you tapped it from (e.g. every child keyboard's "⬅️ Back to HQ"
registers once, in main.go, to onboarding.HandleStart). Anything NOT
in this set that appears on more than one keyboard is a real bug: two
different buttons with identical text can only ever call one handler,
so whichever bot.Handle registration in main.go happens to be
evaluated silently wins and the other button does the wrong thing.
~~~~


## allKeyboards must be kept in sync with every *Navigation function in

Removed from `internal/bot/keyboards/navigation_test.go:22`.

~~~~text
allKeyboards must be kept in sync with every *Navigation function in
this package - deliberately a manual list (not reflection) so adding a
new keyboard function forces a conscious decision to add it here too.
~~~~


## TestNoButtonTextDuplicatedWithinASingleKeyboard catches the simpler,

Removed from `internal/bot/keyboards/navigation_test.go:63`.

~~~~text
TestNoButtonTextDuplicatedWithinASingleKeyboard catches the simpler,
more obviously broken case: the same button appearing twice on one
keyboard (a copy-paste mistake), which telebot would also silently
only ever route to whichever registration wins.
~~~~


## TestJobsNavigationHasBackButton is a small, specific regression guard:

Removed from `internal/bot/keyboards/navigation_test.go:81`.

~~~~text
TestJobsNavigationHasBackButton is a small, specific regression guard:
every child keyboard in this package must offer a way back to
MainNavigation, and it's easy to forget when adding a new one (there's
no compiler check for "did I add a Back button").
~~~~


## TestMainNavigationHasOddJobsEntry guards the mother-keyboard side of

Removed from `internal/bot/keyboards/navigation_test.go:100`.

~~~~text
TestMainNavigationHasOddJobsEntry guards the mother-keyboard side of
the mother/child pair - Jobs must be reachable from the root menu, not
just have a child keyboard that nothing links to.
~~~~
