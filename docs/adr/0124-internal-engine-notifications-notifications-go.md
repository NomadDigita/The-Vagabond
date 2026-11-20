# ADR 0124: Decisions from `internal/engine/notifications/notifications.go`

Status: Accepted

These decisions were written as comments in `internal/engine/notifications/notifications.go` and moved here so the source no longer carries them. Each entry is the original comment.

## The six message-effect IDs Telegram exposes for free to every bot via

Removed from `internal/engine/notifications/notifications.go:14`.

~~~~text
The six message-effect IDs Telegram exposes for free to every bot via
the message_effect_id parameter (Bot API 7.10+) - identical to what a
human sees in the official client's own long-press effect picker.
There is a much larger set of numeric IDs floating around various
community-maintained lists (accompanying Telegram Premium's
emoji-status picker, not the message-effect picker), but those
aren't valid values for message_effect_id and were deliberately
excluded here - sending an unrecognized ID risks Telegram rejecting
the whole message outright, and this project has no way to verify a
wider list against a live Bot API instance from this sandbox.
Cross-checked (2026-08-06) against a community-maintained ID
reference against the six effects Telegram's own client UI has
offered since message effects launched - see
BUGS_AND_INCONSISTENCIES.md's 2026-08-06 entry for the source.

Defined in this package, not handlers, so both the synchronous
bot-handler path (internal/bot/handlers/effects.go) and the async
tick-engine queuing path (drainQueue below) share one source of
truth instead of two copies drifting out of sync.
~~~~


## maxNotificationAttempts is how many times drainQueue retries a

Removed from `internal/engine/notifications/notifications.go:48`.

~~~~text
maxNotificationAttempts is how many times drainQueue retries a
notification before giving up on it - see notifications' schema
comment on failed_attempts for why this exists: without a cap, one
permanently-undeliverable message can occupy a slot in every future
LIMIT-10 batch forever, starving every genuinely new notification
behind it for every player, not just the one who can't receive it.
~~~~


## recordFailedAttempt increments a notification's failed_attempts and

Removed from `internal/engine/notifications/notifications.go:56`.

~~~~text
recordFailedAttempt increments a notification's failed_attempts and
reports whether the caller should give up on it (in which case it's
marked is_sent = TRUE so it stops occupying a slot in every future
drainQueue batch) rather than retry it again. Pulled out as its own
function specifically so this has a unit-testable seam independent of
needing a real, network-connected *telebot.Bot to exercise drainQueue's
send-failure path end-to-end.
~~~~


## Same defense-in-depth as the tick engine (internal/engine/

Removed from `internal/engine/notifications/notifications.go:91`.

~~~~text
Same defense-in-depth as the tick engine (internal/engine/
tick/engine.go): an unrecovered panic inside drainQueue would
otherwise kill this goroutine permanently, silently stopping
all future outbound player notifications with no crash log
pointing at the cause.
~~~~


## looksLikeHTML reports whether a queued message was deliberately built

Removed from `internal/engine/notifications/notifications.go:121`.

~~~~text
looksLikeHTML reports whether a queued message was deliberately built
with Telegram HTML tags (via the render helpers in battlereport.go /
bot/handlers/render.go) as opposed to being a plain-text string that
simply happens to contain some other "<" character. Checking for a
real closing tag rather than a bare "<" keeps this from false-
positiving on stray angle brackets in old plain messages.
~~~~


## AI factions have synthetic negative user_id rows (see isRealPlayer

Removed from `internal/engine/notifications/notifications.go:139`.

~~~~text
AI factions have synthetic negative user_id rows (see isRealPlayer
in internal/engine/tick/engine.go and cmd/bot/main.go's AI
civilization seeding) with no Telegram session behind them at all.
Most of this codebase's ~50 "INSERT INTO notifications" call sites
(construction-complete, combat reports, etc.) don't check
isRealPlayer before queuing - confirmed live via Render logs
(2026-08-02): "Dispatcher giving up on notification ... to -908002
after 5 failed attempts" for a routine construction-complete alert.
Rather than retrofitting every insert site (same reasoning as the
dedup step below), sweep any that already got queued before ever
attempting delivery, so they don't burn 5 guaranteed-failed
Telegram calls apiece or sit in the table forever.
~~~~


## Deduplication (Phase 7 milestone 2): a burst of the same event -

Removed from `internal/engine/notifications/notifications.go:186`.

~~~~text
Deduplication (Phase 7 milestone 2): a burst of the same event -
e.g. several near-simultaneous route-status pings - can queue the
identical (user_id, message) pair more than once before a player
has had a chance to read any of them. Rather than retrofitting
every one of this codebase's many INSERT INTO notifications call
sites to check for an existing duplicate before queuing, coalesce
here: send the message once per (user, text) pair still pending in
this batch, and mark every duplicate row sent alongside it. This
only merges rows that are byte-identical, so it can never conflate
two different alerts that happen to arrive close together.

effectID rides along on the first row seen for a given dedup key -
every duplicate of the same (user, text) pair should carry the
same effect anyway since they're the same event, so which one
"wins" doesn't matter in practice.
~~~~


## Auto-detect HTML-formatted messages rather than assuming every

Removed from `internal/engine/notifications/notifications.go:221`.

~~~~text
Auto-detect HTML-formatted messages rather than assuming every
queued notification across the whole codebase is HTML-safe.
Only ~a handful of call sites (starting with battle reports)
have been audited and have their dynamic text escaped so far;
the rest still build plain strings the old way. Sending THOSE
with ModeHTML forced on would risk a "can't parse entities"
400 error the moment a player-set name contains a raw &/</>.
A message that was actually built with the render helpers will
contain one of these tags; anything else is sent exactly as
before (plain text, zero behavior change).
~~~~


## message_effect_id isn't in this project's vendored

Removed from `internal/engine/notifications/notifications.go:239`.

~~~~text
message_effect_id isn't in this project's vendored
telebot.v3 fork's typed SendOptions, so this goes through
Bot.Raw exactly like effects.go's sendWithEffect does for
the synchronous path - see that file's doc comment for
why Raw (telebot's own sanctioned escape hatch for Bot
API fields the wrapper hasn't caught up to) rather than a
hand-rolled bypass. Falls back to a normal Bot.Send below
if the raw call fails, so a rejected/stale effect ID can
never cost a player the actual notification.
~~~~


## A permanently-undeliverable message (blocked bot, malformed

Removed from `internal/engine/notifications/notifications.go:264`.

~~~~text
A permanently-undeliverable message (blocked bot, malformed
HTML, oversized text, stale chat) must not retry forever:
with a fixed LIMIT 10 oldest-first batch, enough stuck rows
occupy every slot in every future batch and starve every
genuinely new notification behind them - for any player,
any feature. Give up after maxNotificationAttempts rather
than silently blocking the whole queue.
~~~~


## Mark every duplicate in this batch as sent together, not just

Removed from `internal/engine/notifications/notifications.go:284`.

~~~~text
Mark every duplicate in this batch as sent together, not just
the one that was actually delivered.
~~~~
