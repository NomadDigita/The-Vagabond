# ADR 0076: Decisions from `internal/bot/handlers/feedback_test.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/feedback_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestDoSubmitFeedback_StoresAndNotifiesEveryAdminImmediately is the

Removed from `internal/bot/handlers/feedback_test.go:21`.

~~~~text
TestDoSubmitFeedback_StoresAndNotifiesEveryAdminImmediately is the
direct test for this milestone's actual ask: feedback should reach
admins immediately, not just sit in a table nobody queries.
~~~~


## TestDoSubmitFeedback_NoAdminsConfiguredStillStoresSubmission covers

Removed from `internal/bot/handlers/feedback_test.go:52`.

~~~~text
TestDoSubmitFeedback_NoAdminsConfiguredStillStoresSubmission covers
the degenerate case (no ADMIN_IDS configured) - the submission must
still be saved even if there's nobody to notify.
~~~~


## TestFeedbackSenderLabel_IncludesUsernameWhenPresent verifies the

Removed from `internal/bot/handlers/feedback_test.go:72`.

~~~~text
TestFeedbackSenderLabel_IncludesUsernameWhenPresent verifies the
admin-facing label formatting, and that it doesn't collide in name or
behavior with ranking.go's unrelated displayName function.
~~~~


## TestProfileHandler_FeedbackPendingFlow_DirectMapManipulation verifies

Removed from `internal/bot/handlers/feedback_test.go:101`.

~~~~text
TestProfileHandler_FeedbackPendingFlow_DirectMapManipulation verifies
the pending-input bookkeeping directly (set on button tap, consumed
and cleared on the next message) without needing a telebot.Context
mock - mirrors how admin.go's own pending-input map is exercised
elsewhere in this codebase.
~~~~
