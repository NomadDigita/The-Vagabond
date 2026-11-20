# ADR 0095: Decisions from `internal/bot/handlers/profile.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/profile.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Mirrors admin.go's pendingMu/pending pattern (see its own doc

Removed from `internal/bot/handlers/profile.go:22`.

~~~~text
Mirrors admin.go's pendingMu/pending pattern (see its own doc
comment for the full reasoning): tracks which player is mid-flow
on the "💬 Send Feedback" button, so their next free-text message
is captured as the feedback body instead of falling through to
normal NLP parsing. In-memory rather than DB-backed like
onboarding's naming flow - losing this on a restart just means
the player taps the button again, low enough stakes not to need
persistence.
~~~~


## HandleProfilePanel is the "📊 Player Profile" mother-keyboard entry

Removed from `internal/bot/handlers/profile.go:47`.

~~~~text
HandleProfilePanel is the "📊 Player Profile" mother-keyboard entry
point - plants the ProfileNavigation child keyboard. Every button on
it fires its own report/panel directly (same leaf-action shape as
Jobs/Advisors).
~~~~


## route_status/scout_status live in notification_preferences, not

Removed from `internal/bot/handlers/profile.go:151`.

~~~~text
route_status/scout_status live in notification_preferences, not
users - see MMO_WORLD_EVOLUTION_PLAN.md Phase 7 milestone 2 /
internal/engine/notifications/preferences.go's MutableCategories.
These are deliberately the ONLY notification categories a player
can mute here; combat, discovery, and supply-loss alerts stay on
users.notify_on_* or aren't gated at all, by design.
~~~~


## Codes are normally set the moment a player picks a faction, but

Removed from `internal/bot/handlers/profile.go:211`.

~~~~text
Codes are normally set the moment a player picks a faction, but
fall back to generating one here (e.g. a player runs /refer before
ever finishing onboarding). Same deterministic, collision-free
generator either way, so the value is identical regardless of
which path sets it first.
~~~~


## Telegram first names are free-form user input and can

Removed from `internal/bot/handlers/profile.go:274`.

~~~~text
Telegram first names are free-form user input and can
legitimately be long - clip the RAW name to a safe
rune length before escaping, not after (escaping
first and letting htmlTable's own clipping cut the
result could slice through an entity like "&amp;"
and produce invalid HTML Telegram would reject the
whole message for - see ai.HTMLTable's doc comment).
~~~~


## feedbackSenderLabel builds "First Name (@username)" for the admin

Removed from `internal/bot/handlers/profile.go:304`.

~~~~text
feedbackSenderLabel builds "First Name (@username)" for the admin
alert, matching clan.go's broadcast label convention. Named
distinctly from ranking.go's displayName (a different signature, same
package) to avoid any confusion between the two.
~~~~


## doSubmitFeedback is the testable core shared by the slash command and

Removed from `internal/bot/handlers/profile.go:317`.

~~~~text
doSubmitFeedback is the testable core shared by the slash command and
the button-driven pending-input flow below, matching this session's
established doX convention (doSetTaxRate, doGhostProtocol, etc.):
stores the submission, then - the actual gap this closes, per
FEEDBACK_CHANGELOG_NLP_PLAN.md milestone 1 - immediately notifies
every admin, rather than leaving feedback sitting in a table nobody
thinks to query. Category "general" (non-mutable): an admin muting
routine pings should never cost them real player feedback.
~~~~


## HandleFeedbackButton starts the pending-input flow: the player's next

Removed from `internal/bot/handlers/profile.go:356`.

~~~~text
HandleFeedbackButton starts the pending-input flow: the player's next
text message is captured as their feedback instead of being routed
through nlp.HandleTextMessage - see HandleFeedbackPendingInput below,
wired ahead of nlp.HandleTextMessage in main.go exactly like admin.go
and onboarding.go's own pending-input flows.
~~~~


## HandleFeedbackPendingInput consumes a player's next free-text message

Removed from `internal/bot/handlers/profile.go:375`.

~~~~text
HandleFeedbackPendingInput consumes a player's next free-text message
if (and only if) they're mid-flow from HandleFeedbackButton above.
Returns handled=false immediately for anyone with nothing pending, so
normal NLP text parsing continues completely unaffected - see
main.go's OnText registration for how this chains alongside admin.go
and onboarding.go's equivalents.
~~~~


## feedbackInboxPageSize matches changelog's "at least 5" convention

Removed from `internal/bot/handlers/profile.go:410`.

~~~~text
feedbackInboxPageSize matches changelog's "at least 5" convention
from FEEDBACK_CHANGELOG_NLP_PLAN.md, applied here too for the same
reason: a short, skimmable page rather than a wall of history.
~~~~


## HandleFeedbackInbox (/feedback_inbox, admin-only) lists the most

Removed from `internal/bot/handlers/profile.go:415`.

~~~~text
HandleFeedbackInbox (/feedback_inbox, admin-only) lists the most
recent submissions - so a submission is never visible only in a chat
log if every admin happened to be offline when it was delivered.
~~~~
