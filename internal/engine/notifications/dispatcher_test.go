package notifications

import (
	"context"
	"testing"
)

// TestRecordFailedAttempt_GivesUpAfterMaxAttempts is a regression test
// for a real, reported bug: a permanently-undeliverable notification
// (blocked bot, malformed HTML, oversized message, stale chat) used to
// retry forever with no limit. Since drainQueue polls a fixed LIMIT 10
// oldest-first batch, enough such stuck rows would occupy every slot in
// every future batch and starve every genuinely new notification behind
// them - for any player, any feature, not just the one that can't
// receive messages. This verifies the give-up-after-N-attempts fix.
func TestRecordFailedAttempt_GivesUpAfterMaxAttempts(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", int64(9701)); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	var id string
	if err := db.QueryRow("INSERT INTO notifications (user_id, message) VALUES ($1, 'undeliverable test message') RETURNING id", int64(9701)).Scan(&id); err != nil {
		t.Fatalf("seeding notification: %v", err)
	}

	for i := 1; i < maxNotificationAttempts; i++ {
		gaveUp, err := recordFailedAttempt(ctx, db, id)
		if err != nil {
			t.Fatalf("recordFailedAttempt (attempt %d): %v", i, err)
		}
		if gaveUp {
			t.Fatalf("expected no give-up before reaching maxNotificationAttempts (attempt %d)", i)
		}
		var isSent bool
		_ = db.QueryRow("SELECT is_sent FROM notifications WHERE id = $1", id).Scan(&isSent)
		if isSent {
			t.Fatalf("expected the notification to remain unsent while still under the retry limit (attempt %d)", i)
		}
	}

	// The maxNotificationAttempts-th failure should trigger give-up.
	gaveUp, err := recordFailedAttempt(ctx, db, id)
	if err != nil {
		t.Fatalf("recordFailedAttempt (final attempt): %v", err)
	}
	if !gaveUp {
		t.Error("expected recordFailedAttempt to give up after reaching maxNotificationAttempts")
	}

	var isSent bool
	var failedAttempts int
	if err := db.QueryRow("SELECT is_sent, failed_attempts FROM notifications WHERE id = $1", id).Scan(&isSent, &failedAttempts); err != nil {
		t.Fatalf("reading final notification state: %v", err)
	}
	if !isSent {
		t.Error("expected the abandoned notification to be marked is_sent = TRUE so it stops blocking the queue")
	}
	if failedAttempts != maxNotificationAttempts {
		t.Errorf("expected failed_attempts = %d, got %d", maxNotificationAttempts, failedAttempts)
	}
}

// TestRecordFailedAttempt_DoesNotAffectOtherNotifications proves the fix
// is scoped to the specific failing row: a healthy, still-pending
// notification for a different message must be untouched by another
// notification's failures - this is what stops one poison message from
// blocking everything else once the give-up logic is in place.
func TestRecordFailedAttempt_DoesNotAffectOtherNotifications(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()

	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", int64(9702)); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	var poisonID, healthyID string
	if err := db.QueryRow("INSERT INTO notifications (user_id, message) VALUES ($1, 'poison message') RETURNING id", int64(9702)).Scan(&poisonID); err != nil {
		t.Fatalf("seeding poison notification: %v", err)
	}
	if err := db.QueryRow("INSERT INTO notifications (user_id, message) VALUES ($1, 'a perfectly healthy message') RETURNING id", int64(9702)).Scan(&healthyID); err != nil {
		t.Fatalf("seeding healthy notification: %v", err)
	}

	for i := 0; i < maxNotificationAttempts; i++ {
		if _, err := recordFailedAttempt(ctx, db, poisonID); err != nil {
			t.Fatalf("recordFailedAttempt: %v", err)
		}
	}

	var healthySent bool
	var healthyAttempts int
	if err := db.QueryRow("SELECT is_sent, failed_attempts FROM notifications WHERE id = $1", healthyID).Scan(&healthySent, &healthyAttempts); err != nil {
		t.Fatalf("reading healthy notification: %v", err)
	}
	if healthySent {
		t.Error("expected the healthy notification to be untouched by the poison message's failures")
	}
	if healthyAttempts != 0 {
		t.Errorf("expected the healthy notification's failed_attempts to remain 0, got %d", healthyAttempts)
	}
}
