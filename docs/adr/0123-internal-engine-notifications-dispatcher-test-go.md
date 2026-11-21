# ADR 0123: Decisions from `internal/engine/notifications/dispatcher_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/notifications/dispatcher_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## TestRecordFailedAttempt_GivesUpAfterMaxAttempts is a regression test

Removed from `internal/engine/notifications/dispatcher_test.go:8`.

~~~~text
TestRecordFailedAttempt_GivesUpAfterMaxAttempts is a regression test
for a real, reported bug: a permanently-undeliverable notification
(blocked bot, malformed HTML, oversized message, stale chat) used to
retry forever with no limit. Since drainQueue polls a fixed LIMIT 10
oldest-first batch, enough such stuck rows would occupy every slot in
every future batch and starve every genuinely new notification behind
them - for any player, any feature, not just the one that can't
receive messages. This verifies the give-up-after-N-attempts fix.
~~~~


## TestRecordFailedAttempt_DoesNotAffectOtherNotifications proves the fix

Removed from `internal/engine/notifications/dispatcher_test.go:65`.

~~~~text
TestRecordFailedAttempt_DoesNotAffectOtherNotifications proves the fix
is scoped to the specific failing row: a healthy, still-pending
notification for a different message must be untouched by another
notification's failures - this is what stops one poison message from
blocking everything else once the give-up logic is in place.
~~~~
