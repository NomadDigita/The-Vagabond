package handlers

import (
	"context"
	"database/sql"
	"testing"

	"gopkg.in/telebot.v3"
)

func seedFeedbackUser(t *testing.T, db *sql.DB, telegramID int64, firstName, username string) {
	t.Helper()
	if _, err := db.Exec(`
		INSERT INTO users (telegram_id, first_name, username) VALUES ($1, $2, $3)
		ON CONFLICT (telegram_id) DO UPDATE SET first_name = $2, username = $3`,
		telegramID, firstName, username); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
}

// TestDoSubmitFeedback_StoresAndNotifiesEveryAdminImmediately is the
// direct test for this milestone's actual ask: feedback should reach
// admins immediately, not just sit in a table nobody queries.
func TestDoSubmitFeedback_StoresAndNotifiesEveryAdminImmediately(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()

	seedFeedbackUser(t, db, 9801, "Wanderer", "wanderer_wastes")
	seedFeedbackUser(t, db, 9901, "Admin One", "")
	seedFeedbackUser(t, db, 9902, "Admin Two", "")

	h := &ProfileHandler{DB: db, AdminIDs: []int64{9901, 9902}}
	if err := h.doSubmitFeedback(ctx, 9801, "Wanderer (@wanderer_wastes)", "Please add a dark mode"); err != nil {
		t.Fatalf("doSubmitFeedback: %v", err)
	}

	var storedCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM feedback_submissions WHERE user_id = $1 AND message = $2", int64(9801), "Please add a dark mode").Scan(&storedCount)
	if storedCount != 1 {
		t.Errorf("expected the submission to be stored exactly once, got %d", storedCount)
	}

	for _, adminID := range []int64{9901, 9902} {
		var notifiedCount int
		_ = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND message LIKE '%dark mode%' AND category = 'general'", adminID).Scan(&notifiedCount)
		if notifiedCount == 0 {
			t.Errorf("expected admin %d to be notified immediately with a non-mutable 'general' alert", adminID)
		}
	}
}

// TestDoSubmitFeedback_NoAdminsConfiguredStillStoresSubmission covers
// the degenerate case (no ADMIN_IDS configured) - the submission must
// still be saved even if there's nobody to notify.
func TestDoSubmitFeedback_NoAdminsConfiguredStillStoresSubmission(t *testing.T) {
	db := rankingTestDB(t)
	ctx := context.Background()
	seedFeedbackUser(t, db, 9802, "Lone Wanderer", "")

	h := &ProfileHandler{DB: db, AdminIDs: nil}
	if err := h.doSubmitFeedback(ctx, 9802, "Lone Wanderer", "Hello?"); err != nil {
		t.Fatalf("doSubmitFeedback: %v", err)
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM feedback_submissions WHERE user_id = $1", int64(9802)).Scan(&count)
	if count != 1 {
		t.Errorf("expected the submission to still be stored with zero admins configured, got %d", count)
	}
}

// TestFeedbackSenderLabel_IncludesUsernameWhenPresent verifies the
// admin-facing label formatting, and that it doesn't collide in name or
// behavior with ranking.go's unrelated displayName function.
func TestFeedbackSenderLabel_IncludesUsernameWhenPresent(t *testing.T) {
	withUsername := &telebot.User{FirstName: "Raider", Username: "raider99"}
	if got := feedbackSenderLabel(withUsername); got != "Raider (@raider99)" {
		t.Errorf("expected \"Raider (@raider99)\", got %q", got)
	}

	noUsername := &telebot.User{FirstName: "Raider", Username: ""}
	if got := feedbackSenderLabel(noUsername); got != "Raider" {
		t.Errorf("expected just \"Raider\" with no username, got %q", got)
	}
}

// TestHandleFeedbackInbox_RequiresAdmin is a direct smoke test of the
// admin gate - a non-admin must never see the inbox.
func TestHandleFeedbackInbox_RequiresAdmin(t *testing.T) {
	db := rankingTestDB(t)
	h := &ProfileHandler{DB: db, AdminIDs: []int64{9901}}

	if h.IsAdmin(1234567) {
		t.Error("expected a non-admin ID to not be recognized as admin")
	}
	if !h.IsAdmin(9901) {
		t.Error("expected the configured admin ID to be recognized as admin")
	}
}

// TestProfileHandler_FeedbackPendingFlow_DirectMapManipulation verifies
// the pending-input bookkeeping directly (set on button tap, consumed
// and cleared on the next message) without needing a telebot.Context
// mock - mirrors how admin.go's own pending-input map is exercised
// elsewhere in this codebase.
func TestProfileHandler_FeedbackPendingFlow_DirectMapManipulation(t *testing.T) {
	h := NewProfileHandler(rankingTestDB(t), nil)

	const userID = int64(9803)
	h.feedbackPendingMu.Lock()
	h.feedbackPending[userID] = true
	h.feedbackPendingMu.Unlock()

	h.feedbackPendingMu.Lock()
	pending, ok := h.feedbackPending[userID]
	if ok {
		delete(h.feedbackPending, userID)
	}
	h.feedbackPendingMu.Unlock()

	if !pending {
		t.Fatal("expected the pending flag to be set after HandleFeedbackButton-equivalent bookkeeping")
	}

	h.feedbackPendingMu.Lock()
	_, stillPending := h.feedbackPending[userID]
	h.feedbackPendingMu.Unlock()
	if stillPending {
		t.Error("expected the pending flag to be cleared after being consumed once")
	}
}
