# ADR 0113: Decisions from `internal/bot/keyboards/styled.go`

Status: Accepted

These decisions were written as comments in `internal/bot/keyboards/styled.go` and moved here so the source no longer carries them. Each entry is the original comment.

## styled.go adds support for Telegram Bot API 9.4's button "style"

Removed from `internal/bot/keyboards/styled.go:1`.

~~~~text
styled.go adds support for Telegram Bot API 9.4's button "style"
field (added Feb 9, 2026: bots can now mark a button primary/success/
danger and Telegram renders it blue/green/red).

telebot.v3 (the library this project is pinned to) doesn't model this
field yet - its InlineButton struct has no Style property, and its
json.Marshaler would silently drop anything bolted on. So this
bypasses the typed button path entirely for styled sends: it builds
the raw sendMessage JSON by hand and fires it through
telebot.Bot.Raw(), which accepts any interface{} and POSTs it directly
to the Bot API - no library upgrade required, and nothing here reaches
into telebot's internals.

The one thing that has to be replicated exactly is telebot's
callback_data format, so a styled button's press still reaches the
same bot.Handle("\funique", ...) registrations every other button in
this codebase already uses. See options.go's processButtons in
telebot.v3@v3.3.8: callback_data is always "\f" + unique, or
"\f" + unique + "|" + data if data is non-empty. styledCallbackData
below reproduces that byte-for-byte.
~~~~


## ButtonStyle is one of the three colors Telegram actually offers -

Removed from `internal/bot/keyboards/styled.go:31`.

~~~~text
ButtonStyle is one of the three colors Telegram actually offers -
there's no arbitrary palette, just these three (see Bot API 9.4's
KeyboardButtonStyle: bg_primary, bg_danger, bg_success), so callers
pick one of these constants rather than a free-form string.
~~~~


## StyleDanger renders red - for destructive or hostile actions

Removed from `internal/bot/keyboards/styled.go:38`.

~~~~text
StyleDanger renders red - for destructive or hostile actions
(Attack, Abort, Kick).
~~~~


## StyleSuccess renders green - for safe, confirming, or peaceful

Removed from `internal/bot/keyboards/styled.go:41`.

~~~~text
StyleSuccess renders green - for safe, confirming, or peaceful
actions (Continue, Join, Confirm, Recover).
~~~~


## StylePrimary renders blue - for neutral navigation or informational

Removed from `internal/bot/keyboards/styled.go:44`.

~~~~text
StylePrimary renders blue - for neutral navigation or informational
actions that aren't a destructive/safe binary choice.
~~~~


## StyledBtn pairs a normal telebot.Btn (built exactly the way every

Removed from `internal/bot/keyboards/styled.go:49`.

~~~~text
StyledBtn pairs a normal telebot.Btn (built exactly the way every
other button in this codebase already is, e.g. via
selector.Data(text, unique, args...)) with a color. Embedding Btn
means callers don't learn a second button-building API - they just
wrap the Btn they already have.
~~~~


## rawInlineButton is the literal Bot API 9.4 wire shape for one inline

Removed from `internal/bot/keyboards/styled.go:67`.

~~~~text
rawInlineButton is the literal Bot API 9.4 wire shape for one inline
button, including the style field telebot.InlineButton doesn't have
yet. Field names/tags match
https://core.telegram.org/bots/api#inlinekeyboardbutton plus the new
style addition exactly.
~~~~


## styledCallbackData reproduces telebot.v3@v3.3.8's options.go

Removed from `internal/bot/keyboards/styled.go:79`.

~~~~text
styledCallbackData reproduces telebot.v3@v3.3.8's options.go
processButtons formatting exactly, so a press on a styled button
still matches the same bot.Handle("\f<unique>", ...) registration a
non-styled button with the same Unique/Data would.
~~~~


## telegramCallbackDataLimit is Telegram's hard cap on callback_data

Removed from `internal/bot/keyboards/styled.go:93`.

~~~~text
telegramCallbackDataLimit is Telegram's hard cap on callback_data
(see https://core.telegram.org/bots/api#inlinekeyboardbutton). Exceed
it and Telegram doesn't reject the message or the button - it just
silently drops that one button's tap forever, no error surfaced
anywhere. This exact bug was found and fixed elsewhere in this
codebase for road_encounter/rbe (see callback_data_length_test.go);
guarded here at the point where styled buttons are actually sent, not
just in per-call-site tests, since a future call site could easily
reintroduce it without anyone noticing until a player reports a dead
button.
~~~~


## rawSendMessage is the minimal subset of sendMessage's parameters this

Removed from `internal/bot/keyboards/styled.go:126`.

~~~~text
rawSendMessage is the minimal subset of sendMessage's parameters this
package needs - just enough to carry text, HTML formatting, and a
styled inline keyboard. Everything else behaves exactly like a normal
telebot c.Send(text, telebot.ModeHTML) call.
~~~~


## SendStyled sends text with a colored inline keyboard to whatever

Removed from `internal/bot/keyboards/styled.go:139`.

~~~~text
SendStyled sends text with a colored inline keyboard to whatever
c.Chat() resolves to - the same recipient c.Send() would use. Text is
always sent as HTML (every call site building StyledBtn rows also
wants htmlBold/htmlCode-formatted text, matching this game's existing
panel style), so callers don't need to pass a parse mode.
~~~~


## marshalRowsForTest exposes the exact outgoing inline_keyboard JSON for

Removed from `internal/bot/keyboards/styled.go:172`.

~~~~text
marshalRowsForTest exposes the exact outgoing inline_keyboard JSON for
a row of styled buttons without needing a live *telebot.Bot/Raw()
round trip - styled_test.go uses this to verify the wire shape (field
names, omitted-when-empty behavior, callback_data format) directly.
~~~~
