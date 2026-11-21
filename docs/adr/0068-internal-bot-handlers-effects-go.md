# ADR 0068: Decisions from `internal/bot/handlers/effects.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/effects.go` and moved here so the source no longer carries them. Each entry is the original comment.

## This file adds support for Telegram's message-effect animations

Removed from `internal/bot/handlers/effects.go:10`.

~~~~text
This file adds support for Telegram's message-effect animations
(Bot API 7.10+ `message_effect_id` parameter on sendMessage) - a
small one-shot animation that plays for the recipient when a message
arrives, the same effect a human gets from long-pressing a message
and picking one from Telegram's own effect picker.

2026-08-06 direct request, after this project's telebot.v3 fork
(go-telebot/telebot v3.3.8) was confirmed to have no typed field for
it anywhere in SendOptions. Rather than vendoring a patched copy of
the library or hand-rolling a bypass around it, this goes through
Bot.Raw - telebot's own sanctioned escape hatch (see its doc comment
in the vendored api.go: "Raw lets you call any method of Bot API
manually... handles API errors, so you only need to unwrap result").
That's the library's intended answer to exactly this situation - a
real Bot API field the typed wrapper hasn't caught up to yet - not a
workaround of it.
~~~~


## Defined once in internal/engine/notifications (shared by both this

Removed from `internal/bot/handlers/effects.go:27`.

~~~~text
Defined once in internal/engine/notifications (shared by both this
synchronous path and the async tick-engine queuing path - see
notifications.go/preferences.go) and re-exported here so existing/
future call sites in this package can keep writing the shorter
handlers.EffectFire etc.
~~~~


## sendRichMessage sends htmlContent as a Telegram Rich Message (Bot API

Removed from `internal/bot/handlers/effects.go:41`.

~~~~text
sendRichMessage sends htmlContent as a Telegram Rich Message (Bot API
10.1, June 2026 - method sendRichMessage) instead of a plain
sendMessage. Not exposed by this project's vendored telebot.v3 fork
(v3.3.8, which predates Bot API 10.1 entirely), so - same reasoning
as sendWithEffect above - this goes through Bot.Raw rather than a
hand-rolled bypass or a vendored library patch.

htmlContent is parsed as Rich HTML style (InputRichMessage.html),
which is a near-superset of the plain ModeHTML this codebase already
uses everywhere: every tag htmlBold/htmlItalic/htmlCode/htmlQuote/
htmlSpoiler/htmlUnderline already emit (<b>, <i>, <code>,
<blockquote>, <tg-spoiler>, <u>) still works unchanged, PLUS
<table>, <details>, <h1>-<h6>, <hr/>, <aside> pull-quotes, and more
that plain sendMessage's HTML parse mode has no equivalent for at
all. This makes it safe to reuse this codebase's existing html*
helpers verbatim when building rich content - see exchange.go's
listings table for the first real use.

Falls back to a normal c.Send (plain ModeHTML, same content) if the
raw sendRichMessage call fails for any reason - same fail-open
reasoning as sendWithEffect: a nicer-looking table is never worth
losing the underlying message over, whether that's an older client,
a future Bot API change, or Telegram rejecting something about the
markup. Since Rich HTML style is a superset of plain ModeHTML for
every tag this codebase's helpers produce, the fallback renders
correctly (just without native tables/details/headings) rather than
failing outright.
~~~~


## filterReplyMarkup drops anything from opts that sendRichMessage

Removed from `internal/bot/handlers/effects.go:95`.

~~~~text
filterReplyMarkup drops anything from opts that sendRichMessage
doesn't also accept (it has no text/parse_mode parameter of its own -
content lives entirely in rich_message), so sendRichMessage's
fallback path can reuse whatever *telebot.ReplyMarkup a caller passed
without also re-passing a stray telebot.ParseMode a caller might have
included out of habit.
~~~~


## sendWithEffect sends text to the chat behind c with a message effect

Removed from `internal/bot/handlers/effects.go:111`.

~~~~text
sendWithEffect sends text to the chat behind c with a message effect
attached, otherwise behaving like c.Send(text, opts...) - the same
telebot.ModeHTML/*telebot.ReplyMarkup option values already used
everywhere else in this package work here unchanged. Effects are
cosmetic and best-effort by design: if the raw call fails for any
reason (unexpected Bot API change, rate limit, etc.) this silently
falls back to a normal c.Send rather than losing the message
entirely - the animation is a nice-to-have, the message itself never
is.
~~~~


## Best-effort animation - fall back to a plain send so the

Removed from `internal/bot/handlers/effects.go:149`.

~~~~text
Best-effort animation - fall back to a plain send so the
actual game content (raid result, boss kill, etc.) still
reaches the player even if the effect itself was rejected.
~~~~
