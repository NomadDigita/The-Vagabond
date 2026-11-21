# ADR 0090: Decisions from `internal/bot/handlers/nlp.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/nlp.go` and moved here so the source no longer carries them. Each entry is the original comment.

## fallbackNotRecognizedMsg is shown when nothing - not a hardcoded

Removed from `internal/bot/handlers/nlp.go:14`.

~~~~text
fallbackNotRecognizedMsg is shown when nothing - not a hardcoded
shortcut, not a lexical pattern, not the AI command interpreter -
matched the player's free text. Extracted to a constant since the
AI fallback path (added in Milestone 3, see
FEEDBACK_CHANGELOG_NLP_PLAN.md) also degrades to this exact message
on any error, so both call sites stay in sync.
~~~~


## Interpreter is Milestone 3's natural-language command

Removed from `internal/bot/handlers/nlp.go:36`.

~~~~text
Interpreter is Milestone 3's natural-language command
interpreter (FEEDBACK_CHANGELOG_NLP_PLAN.md) - the final
fallback in HandleTextMessage's chain, after every hardcoded
shortcut and lexical pattern below. May be nil (e.g. in tests
that don't need it), in which case that fallback is skipped
entirely and behavior is identical to pre-Milestone-3.
~~~~


## --- 7. AI COMMAND INTERPRETER (Milestone 3) ---

Removed from `internal/bot/handlers/nlp.go:156`.

~~~~text
--- 7. AI COMMAND INTERPRETER (Milestone 3) ---
Tried before the broad lexical "contains" heuristics below,
since those are fuzzy navigation shortcuts (e.g. any message
containing "scout" routing to the Raid Board) that would
otherwise swallow a specific, actionable command like "send 5
scouts" before the smarter classifier ever saw it. If nothing
here is handled (no Interpreter configured, no live AI provider,
budget exceeded, or the model itself didn't find a confident
match), control falls through to section 8 exactly as before
Milestone 3 - this is purely additive.
~~~~


## tryNaturalLanguageCommand is Milestone 3's entry point

Removed from `internal/bot/handlers/nlp.go:196`.

~~~~text
tryNaturalLanguageCommand is Milestone 3's entry point
(FEEDBACK_CHANGELOG_NLP_PLAN.md): it hands the player's raw message
to the AI command interpreter and, if a confident result came back,
handles it (executing read-only actions immediately, or showing a
Confirm/Cancel card for anything that spends a resource or commits
forces) and reports handled=true. Returns handled=false for every
case that should fall through to the existing lexical shortcuts
instead - including "no Interpreter wired", "AI disabled/budget
exceeded/erroring", and "no live provider configured" (the mock
provider's placeholder text is intentionally never shown here,
unlike the dedicated AI Advisor panels elsewhere that do show it -
a player typing ordinary free text never explicitly asked for an AI
feature, so silently degrading to the pre-Milestone-3 experience is
the better default until a real provider key is set).
~~~~


## dispatchParsedCommand routes a matched, allow-listed command to its

Removed from `internal/bot/handlers/nlp.go:233`.

~~~~text
dispatchParsedCommand routes a matched, allow-listed command to its
execution: read-only actions run immediately through the exact same
panel handlers the buttons use; mutating actions show a
Confirm/Cancel card first (see Action.RequiresConfirmation).
~~~~


## confirmListMarketItem renders the Confirm/Cancel card for a parsed

Removed from `internal/bot/handlers/nlp.go:262`.

~~~~text
confirmListMarketItem renders the Confirm/Cancel card for a parsed
list_market_item command. Nothing here touches the database - the
actual listing is only posted if the player taps Confirm, via
HandleNLPConfirmCallback calling the same doPostListing core the
button-driven Exchange panel uses. Supports both a cash asking
price (ask_type "dollars") and a barter ask (ask_type is another
resource) - see doPostListing's doc comment in exchange.go.
~~~~


## confirmBuyMarketItem renders the Confirm/Cancel card for a parsed

Removed from `internal/bot/handlers/nlp.go:305`.

~~~~text
confirmBuyMarketItem renders the Confirm/Cancel card for a parsed
buy_market_item command. Nothing here touches the database - the
search for a matching listing AND the purchase itself only happen
if the player taps Confirm, via HandleNLPConfirmCallback calling
doBuyMarketItem. The card is deliberately worded as a search-and-buy
("buy the best available X for up to $Y"), not a promise of an
exact quantity/price, since doBuyMarketItem only finds out which
listing (if any) matches at the moment Confirm is tapped - there's
no listing to preview yet at card-render time, unlike
confirmListMarketItem where every detail is already fully specified
by the player's own message.
~~~~


## confirmDispatchScoutMission renders the Confirm/Cancel card for a

Removed from `internal/bot/handlers/nlp.go:339`.

~~~~text
confirmDispatchScoutMission renders the Confirm/Cancel card for a
parsed dispatch_scout_mission command. See confirmListMarketItem's
doc comment - same "nothing executes until Confirm" contract.
~~~~


## nlpCardOutcome edits the original Confirm/Cancel card in place so its

Removed from `internal/bot/handlers/nlp.go:358`.

~~~~text
nlpCardOutcome edits the original Confirm/Cancel card in place so its
buttons stop being tappable the instant an outcome is known - fixes
the 2026-08-06 report of Cancel leaving the card (and its still-live
Confirm button) on screen, so a second tap after "Cancelled" silently
re-ran the action anyway. Every other Confirm/Cancel flow in this
codebase (see sendCancelledCard/HandleHyperSpeedConfirmCallback in
jobs.go) already edits the card on both outcomes; this brings the
AI-parsed-command cards in line with that pattern instead of leaving
them as a standing toast-only exception.

The full-length result also lands in the edited card text (Telegram
allows up to 4096 chars there) rather than only in the small toast
popup fired by c.Respond, which Telegram visually clips after a short
run of characters - the second reported bug ("notification reply is
truncated"). The toast still fires too, as a quick non-blocking ack,
but callers no longer have to cram the whole explanation into it.
~~~~


## HandleNLPConfirmCallback fires when a player taps the green Confirm

Removed from `internal/bot/handlers/nlp.go:379`.

~~~~text
HandleNLPConfirmCallback fires when a player taps the green Confirm
button on an AI-parsed command's card. It re-derives the caller's
encampment server-side (never trusts a campID from callback_data)
and executes through the exact same core function the button-driven
UI uses - doPostListing / doDispatchScoutMission - so a
natural-language "list 300k scrap for $500" enforces identical
validation to tapping the Exchange panel's own buttons. Every branch
ends by editing the card via nlpCardOutcome (see its doc comment) so
the Confirm/Cancel buttons can never be tapped a second time.
~~~~


## HandleNLPCancelCallback fires when a player taps the red Cancel

Removed from `internal/bot/handlers/nlp.go:474`.

~~~~text
HandleNLPCancelCallback fires when a player taps the red Cancel
button on an AI-parsed command's card. Nothing was ever committed
before this point, so this just closes out the card - critically,
via nlpCardOutcome's c.Edit, not a bare toast, so the Confirm button
actually goes away instead of staying live for a stray follow-up tap
(see nlpCardOutcome's doc comment for the full 2026-08-06 bug report).
~~~~
