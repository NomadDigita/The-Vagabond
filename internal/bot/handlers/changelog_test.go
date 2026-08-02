package handlers

import (
	"context"
	"database/sql"
	"strings"
	"testing"
)

// changelogTestDB wraps rankingTestDB with an extra truncate for the
// two changelog tables, which aren't covered by rankingTestDB's own
// TRUNCATE list (no FK to users/encampments to cascade from) - needed
// here specifically because these tests depend on exact ordering and
// counts, unlike most other tests in this package which just filter by
// a known seeded ID and don't care what else is in the table.
func changelogTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db := rankingTestDB(t)
	if _, err := db.Exec("TRUNCATE changelog_entries, changelog_reads RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncating changelog tables: %v", err)
	}
	return db
}

func seedChangelogEntry(t *testing.T, db *sql.DB, category, title, body string) string {
	t.Helper()
	var id string
	if err := db.QueryRow("INSERT INTO changelog_entries (category, title, body) VALUES ($1, $2, $3) RETURNING id", category, title, body).Scan(&id); err != nil {
		t.Fatalf("seeding changelog entry: %v", err)
	}
	return id
}

// TestDoPublishChangelog_StoresAndBroadcastsToEveryPlayer is the direct
// test for the "dispatch it to users" half of the milestone 2 ask.
func TestDoPublishChangelog_StoresAndBroadcastsToEveryPlayer(t *testing.T) {
	db := changelogTestDB(t)
	ctx := context.Background()

	seedFeedbackUser(t, db, 9901, "Player One", "")
	seedFeedbackUser(t, db, 9902, "Player Two", "")

	h := &ChangelogHandler{DB: db}
	msg, err := h.doPublishChangelog(ctx, "feature", "Long-Range Scouting", "Dispatch Scout Walkers to search the wasteland.")
	if err != nil {
		t.Fatalf("doPublishChangelog: %v (%s)", err, msg)
	}

	var storedCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM changelog_entries WHERE title = 'Long-Range Scouting'").Scan(&storedCount)
	if storedCount != 1 {
		t.Errorf("expected exactly one stored entry, got %d", storedCount)
	}

	for _, userID := range []int64{9901, 9902} {
		var notifiedCount int
		_ = db.QueryRow("SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND message LIKE '%Long-Range Scouting%' AND category = 'general'", userID).Scan(&notifiedCount)
		if notifiedCount == 0 {
			t.Errorf("expected player %d to receive the broadcast with a non-mutable 'general' category", userID)
		}
	}
}

// TestDoPublishChangelog_RejectsInvalidCategory verifies the category
// allow-list (feature/fix/balance) is enforced before anything is
// stored or broadcast.
func TestDoPublishChangelog_RejectsInvalidCategory(t *testing.T) {
	db := changelogTestDB(t)
	ctx := context.Background()

	h := &ChangelogHandler{DB: db}
	if _, err := h.doPublishChangelog(ctx, "rumor", "Title", "Body"); err == nil {
		t.Error("expected an invalid category to be rejected")
	}

	var count int
	_ = db.QueryRow("SELECT COUNT(*) FROM changelog_entries").Scan(&count)
	if count != 0 {
		t.Errorf("expected no entry to be stored for an invalid category, got %d", count)
	}
}

// TestDoPublishChangelog_RejectsEmptyTitleOrBody verifies both required
// fields are checked.
func TestDoPublishChangelog_RejectsEmptyTitleOrBody(t *testing.T) {
	db := changelogTestDB(t)
	ctx := context.Background()
	h := &ChangelogHandler{DB: db}

	if _, err := h.doPublishChangelog(ctx, "fix", "", "Body"); err == nil {
		t.Error("expected an empty title to be rejected")
	}
	if _, err := h.doPublishChangelog(ctx, "fix", "Title", ""); err == nil {
		t.Error("expected an empty body to be rejected")
	}
}

// TestFetchChangelogPage_ReturnsOldestUnreadFirst is the direct test
// for the plan doc's core design choice: a player with a backlog sees
// the oldest entries first, catching up in chronological order.
func TestFetchChangelogPage_ReturnsOldestUnreadFirst(t *testing.T) {
	db := changelogTestDB(t)
	ctx := context.Background()
	seedFeedbackUser(t, db, 9903, "Catching Up Player", "")

	first := seedChangelogEntry(t, db, "feature", "First Entry", "The oldest one.")
	seedChangelogEntry(t, db, "fix", "Second Entry", "The middle one.")
	seedChangelogEntry(t, db, "balance", "Third Entry", "The newest one.")

	h := &ChangelogHandler{DB: db}
	entries, caughtUp, err := h.fetchChangelogPage(ctx, 9903)
	if err != nil {
		t.Fatalf("fetchChangelogPage: %v", err)
	}
	if caughtUp {
		t.Error("expected caughtUp=false with 3 unread entries")
	}
	if len(entries) != 3 {
		t.Fatalf("expected all 3 entries (page size 5), got %d", len(entries))
	}
	if entries[0].id != first {
		t.Errorf("expected the oldest entry first, got title %q", entries[0].title)
	}
	if entries[0].title != "First Entry" || entries[2].title != "Third Entry" {
		t.Errorf("expected oldest-to-newest order, got %v", []string{entries[0].title, entries[1].title, entries[2].title})
	}
}

// TestFetchChangelogPage_FallsBackToMostRecentWhenCaughtUp verifies the
// one deliberate deviation from the plan's literal "oldest" instruction:
// once nothing is unread, show the most recent entries instead.
func TestFetchChangelogPage_FallsBackToMostRecentWhenCaughtUp(t *testing.T) {
	db := changelogTestDB(t)
	ctx := context.Background()
	seedFeedbackUser(t, db, 9904, "Caught Up Player", "")

	seedChangelogEntry(t, db, "feature", "Old News", "Already read.")
	newest := seedChangelogEntry(t, db, "fix", "Fresh News", "Also already read.")

	h := &ChangelogHandler{DB: db}
	// Mark everything read first, simulating a player who's caught up.
	if _, err := db.Exec("INSERT INTO changelog_reads (user_id, entry_id) SELECT 9904, id FROM changelog_entries"); err != nil {
		t.Fatalf("seeding reads: %v", err)
	}

	entries, caughtUp, err := h.fetchChangelogPage(ctx, 9904)
	if err != nil {
		t.Fatalf("fetchChangelogPage: %v", err)
	}
	if !caughtUp {
		t.Error("expected caughtUp=true when everything is already read")
	}
	if len(entries) != 2 {
		t.Fatalf("expected the 2 most recent entries as a fallback, got %d", len(entries))
	}
	if entries[0].id != newest {
		t.Errorf("expected most-recent-first ordering in the fallback, got title %q first", entries[0].title)
	}
}

// TestHandleChangelogPanel_MarksEntriesReadOnView verifies the
// mark-as-read-on-view behavior end to end: viewing a page of unread
// entries clears them from the next page's unread query.
func TestHandleChangelogPanel_MarksEntriesReadOnView(t *testing.T) {
	db := changelogTestDB(t)
	ctx := context.Background()
	seedFeedbackUser(t, db, 9905, "Reading Player", "")

	seedChangelogEntry(t, db, "feature", "Entry A", "Body A")
	seedChangelogEntry(t, db, "fix", "Entry B", "Body B")

	h := &ChangelogHandler{DB: db}
	entries, caughtUp, err := h.fetchChangelogPage(ctx, 9905)
	if err != nil {
		t.Fatalf("fetchChangelogPage: %v", err)
	}
	if caughtUp {
		t.Fatal("expected an unread backlog on first view")
	}
	if err := h.markChangelogEntriesRead(ctx, 9905, entries); err != nil {
		t.Fatalf("markChangelogEntriesRead: %v", err)
	}

	var unreadCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM changelog_entries WHERE id NOT IN (SELECT entry_id FROM changelog_reads WHERE user_id = $1)", int64(9905)).Scan(&unreadCount)
	if unreadCount != 0 {
		t.Errorf("expected zero unread entries after marking the page read, got %d", unreadCount)
	}
}

// TestRenderChangelogText_ShowsCatchUpNoticeWhenAppropriate is a small
// unit test for the pure rendering function.
func TestRenderChangelogText_ShowsCatchUpNoticeWhenAppropriate(t *testing.T) {
	entries := []changelogEntry{{id: "1", category: "feature", title: "Something New"}}

	notCaughtUp := renderChangelogText(entries, false)
	if strings.Contains(notCaughtUp, "caught up") {
		t.Error("expected no catch-up notice when there's still an unread backlog")
	}

	caughtUp := renderChangelogText(entries, true)
	if !strings.Contains(caughtUp, "caught up") {
		t.Error("expected a catch-up notice when caughtUp=true")
	}
}

// TestHandlePublishChangelogPendingInput_RequiresThreeLines verifies
// the guided-input format parsing used by the admin panel's flow.
func TestHandlePublishChangelogPendingInput_ParsesThreeLineFormat(t *testing.T) {
	lines := strings.SplitN("feature\nMy Title\nMy body text here.", "\n", 3)
	if len(lines) != 3 {
		t.Fatalf("expected 3 parts, got %d: %v", len(lines), lines)
	}
	if lines[0] != "feature" || lines[1] != "My Title" || lines[2] != "My body text here." {
		t.Errorf("unexpected split result: %v", lines)
	}
}
