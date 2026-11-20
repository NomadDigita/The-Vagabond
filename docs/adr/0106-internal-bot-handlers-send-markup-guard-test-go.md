# ADR 0106: Decisions from `internal/bot/handlers/send_markup_guard_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/send_markup_guard_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestNoDoubleReplyMarkupInSingleSendCall is a regression guard for a

Removed from `internal/bot/handlers/send_markup_guard_test.go:11`.

~~~~text
TestNoDoubleReplyMarkupInSingleSendCall is a regression guard for a
real bug found via a player screenshot: HandleAdminPanel called

	c.Send(panelText, telebot.ModeHTML, selector, keyboards.AdminNavigation())

passing BOTH an inline-keyboard *telebot.ReplyMarkup (selector) and a
reply-keyboard one (keyboards.AdminNavigation()) to a single c.Send
call. telebot only honors one *telebot.ReplyMarkup per message - the
second one silently wins or loses depending on option-processing
order, and here it meant the inline selector never rendered at all.
Every admin action button (Tick, Inject, Gift Premium, Broadcast,
Publish Changelog, all of them) was invisible - not a missing
feature, a send call quietly eating its own keyboard. This is
exactly the "single-ReplyMarkup collision" this codebase's own
sendPanelWithNav/sendPanelWithNavHTML helpers (navhelper.go) exist to
prevent by sending the nav update and the panel as two separate
messages - the fix was routing HandleAdminPanel through that helper
like every other panel already does.

This test scans every non-test handler source file for the same
anti-pattern: a c.Send(...) call whose argument list contains both an
inline-selector-looking identifier and a keyboards.XNavigation()
call. It's a source-text scan rather than full Go parsing (simpler,
and precise enough for this specific shape), deliberately erring
toward catching too much rather than too little - a false positive
here just means someone double-checks a c.Send call that turns out
to be fine, whereas a false negative silently ships another dead
keyboard.
~~~~


## A bare "selector" identifier used as a *telebot.ReplyMarkup

Removed from `internal/bot/handlers/send_markup_guard_test.go:47`.

~~~~text
A bare "selector" identifier used as a *telebot.ReplyMarkup
argument - deliberately not matching "selector.Data(" or
"selector.Row(" (those build buttons, not pass the markup itself).
~~~~
