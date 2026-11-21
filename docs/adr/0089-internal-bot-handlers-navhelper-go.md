# ADR 0089: Decisions from `internal/bot/handlers/navhelper.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/navhelper.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Standard short captions for the nav-transition message sendPanelWithNav

Removed from `internal/bot/handlers/navhelper.go:5`.

~~~~text
Standard short captions for the nav-transition message sendPanelWithNav
sends ahead of the real panel content. Kept centralized so every call
site announces the same way instead of inventing its own wording.
~~~~


## sendPanelWithNav sends a panel that needs both its own inline action

Removed from `internal/bot/handlers/navhelper.go:17`.

~~~~text
sendPanelWithNav sends a panel that needs both its own inline action
buttons AND a persistent bottom navigation bar update.

Telegram's Bot API only allows ONE reply_markup per message - it's an
inline keyboard, OR a persistent ReplyKeyboardMarkup, never both at
once. telebot's option parser reflects that: when multiple
*telebot.ReplyMarkup values are passed to c.Send, the LAST one simply
overwrites the others rather than merging, so `c.Send(text, selector,
someNavKeyboard)` silently discards selector's inline buttons every
time. (See SPACEHUNT_PHASE7_LOG.md item 3's post-mortem for the full
incident writeup - this shipped broken across a number of panels
before being caught.)

The correct way to get both onscreen is two messages: a short one
that plants the new persistent keyboard, then the real panel content
with just its inline selector attached (which leaves the
just-planted persistent keyboard alone, since Telegram only replaces
it when a NEW reply_markup of that kind is sent).
~~~~


## sendPanelWithNavHTML is identical to sendPanelWithNav but renders the

Removed from `internal/bot/handlers/navhelper.go:42`.

~~~~text
sendPanelWithNavHTML is identical to sendPanelWithNav but renders the
panel body with Telegram HTML parse mode. Only use this at a call
site where `text` has already been built with the htmlBold/htmlItalic/
htmlCode/htmlEscape helpers in render.go - any raw "<", ">", or "&"
left un-escaped in interpolated data (player names, descriptions,
etc.) will make Telegram reject the whole message.
~~~~


## sendPanelWithNavRich is sendPanelWithNavHTML's Rich-Message

Removed from `internal/bot/handlers/navhelper.go:55`.

~~~~text
sendPanelWithNavRich is sendPanelWithNavHTML's Rich-Message
counterpart (see sendRichMessage in effects.go) - use at a call site
that wants native <table>/<details>/<h1-6> rendering, not just plain
ModeHTML's tag set. Same two-message nav-then-panel structure and
same htmlBold/htmlItalic/htmlCode/htmlEscape escaping discipline
applies to `text` here as sendPanelWithNavHTML - Rich HTML style is
a superset of plain ModeHTML, not a different escaping scheme.
~~~~


## weatherLine gives the Wasteland Radio feed's one-line-per-continent

Removed from `internal/bot/handlers/navhelper.go:69`.

~~~~text
weatherLine gives the Wasteland Radio feed's one-line-per-continent
description for a given active event type ("" or "nominal" for clear
conditions). See internal/engine/world/weather.go's eventHeadline for
the matching news-flash wording, and each mechanical consumer
(internal/engine/tick/engine.go, internal/engine/resource/resource.go,
this package's combat.go/camp.go) for where the effects actually
apply.
~~~~
