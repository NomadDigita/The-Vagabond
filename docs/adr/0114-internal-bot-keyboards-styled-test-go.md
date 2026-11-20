# ADR 0114: Decisions from `internal/bot/keyboards/styled_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/keyboards/styled_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## telebotCallbackRegex is copied verbatim from telebot.v3@v3.3.8's

Removed from `internal/bot/keyboards/styled_test.go:11`.

~~~~text
telebotCallbackRegex is copied verbatim from telebot.v3@v3.3.8's
bot.go (the unexported cbackRx the library uses to recognize an
incoming callback query and route it to a bot.Handle("\f<unique>", ...)
registration). This test proves a styled button's callback_data still
matches that exact pattern, so pressing one reaches the same handler
a non-styled button with the same Unique/Data would - no changes
needed anywhere else in the codebase.
~~~~


## A URL button (or any button that isn't a callback button at all)

Removed from `internal/bot/keyboards/styled_test.go:66`.

~~~~text
A URL button (or any button that isn't a callback button at all)
must not get a synthesized callback_data - that would make
Telegram treat it as a callback button instead of a URL button.
~~~~


## Bot API 9.4's actual accepted values, per

Removed from `internal/bot/keyboards/styled_test.go:136`.

~~~~text
Bot API 9.4's actual accepted values, per
https://core.telegram.org/bots/api#inlinekeyboardbutton - if these
ever drift from what Telegram expects, every styled button in the
game silently renders with no color at all (an unrecognized style
value is simply ignored, not rejected), so pin them explicitly.
~~~~


## TestToRawButton_CallbackDataStaysWithinTelegramsLimit is a general

Removed from `internal/bot/keyboards/styled_test.go:152`.

~~~~text
TestToRawButton_CallbackDataStaysWithinTelegramsLimit is a general
regression guard for the same class of bug found and fixed elsewhere
in this codebase for road_encounter/rbe
(callback_data_length_test.go): a callback_data over Telegram's
64-byte cap doesn't error, it just makes that button silently never
respond to a tap. Any future styled button built with too much data
crammed into Unique+Data would hit this exact failure mode - verify
toRawButton's output length directly, independent of any particular
call site remembering to test it themselves.
~~~~


## 36 bytes, representative of a real Postgres UUID

Removed from `internal/bot/keyboards/styled_test.go:162`.

~~~~text
36 bytes, representative of a real Postgres UUID
~~~~


## TestToRawButton_DetectsOversizedCallbackData confirms the specific

Removed from `internal/bot/keyboards/styled_test.go:171`.

~~~~text
TestToRawButton_DetectsOversizedCallbackData confirms the specific
shape that broke road_encounter before its fix (two UUIDs plus a
unique/action prefix) is exactly what SendStyled's length guard is
designed to catch. SendStyled's own guard can't be exercised without
a live *telebot.Bot (its unexported HTTP client can't be swapped for
a test server from outside the telebot package), but the guard is a
direct length check on this same value, so this pins the boundary
condition it depends on.
~~~~
