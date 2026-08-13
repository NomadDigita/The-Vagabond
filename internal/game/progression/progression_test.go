package progression

import (
	"context"
	"database/sql"
	"os"
	"testing"

	_ "github.com/lib/pq"

	"github.com/NomadDigita/The-Vagabond/internal/db/schema"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("SCHEMA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SCHEMA_TEST_DATABASE_URL not set; skipping real-database progression test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range schema.Statements() {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("applying schema: %v\n%s", err, stmt)
		}
	}
	if _, err := db.Exec(`TRUNCATE player_event_counts, player_achievements, player_quest_progress, resources, encampments, coordinates, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncating tables: %v", err)
	}
	return db
}

// seedProgressionPlayer seeds a user + encampment + resources row
// (needed since RecordEvent's reward crediting looks up the
// encampment by user_id) with plenty of storage headroom so reward
// crediting isn't accidentally clamped in a way that would make a test
// assertion ambiguous between "reward wasn't paid" and "reward was
// paid but clamped."
func seedProgressionPlayer(t *testing.T, db *sql.DB, userID int64) string {
	t.Helper()
	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", userID); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	var coordID string
	if err := db.QueryRow(`
		INSERT INTO coordinates (x, y, biome, region, terrain) VALUES ($1, $1, 'plains', 'TestRegion', 'flat')
		RETURNING id`, userID).Scan(&coordID); err != nil {
		t.Fatalf("seeding coordinate: %v", err)
	}
	var campID string
	if err := db.QueryRow(`
		INSERT INTO encampments (user_id, name, coordinate_id, extension_lvl) VALUES ($1, 'Progression Test Camp', $2, 40)
		RETURNING id`, userID, coordID).Scan(&campID); err != nil {
		t.Fatalf("seeding encampment: %v", err)
	}
	if _, err := db.Exec("INSERT INTO resources (encampment_id, dollars) VALUES ($1, 0)", campID); err != nil {
		t.Fatalf("seeding resources: %v", err)
	}
	return campID
}

// TestRecordEvent_UnlocksAchievementAtThreshold is the direct test for
// NEXT_GENERATION_FEATURES_PLAN.md Phase 1: reaching an achievement's
// threshold unlocks it exactly once and credits its reward.
func TestRecordEvent_UnlocksAchievementAtThreshold(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	const userID = 90001
	seedProgressionPlayer(t, db, userID)

	// "first_raid" unlocks at TriggerRaidCompleted threshold 1.
	if err := RecordEvent(ctx, db, userID, TriggerRaidCompleted, 1); err != nil {
		t.Fatalf("RecordEvent: unexpected error: %v", err)
	}

	var unlockedAt sql.NullTime
	if err := db.QueryRow("SELECT unlocked_at FROM player_achievements WHERE user_id = $1 AND achievement_key = 'first_raid'", userID).Scan(&unlockedAt); err != nil {
		t.Fatalf("expected first_raid to be unlocked: %v", err)
	}
	if !unlockedAt.Valid {
		t.Error("expected unlocked_at to be set")
	}

	var dollars float64
	if err := db.QueryRow(`
		SELECT r.dollars FROM resources r JOIN encampments e ON e.id = r.encampment_id WHERE e.user_id = $1`, userID).Scan(&dollars); err != nil {
		t.Fatalf("reading resources: %v", err)
	}
	if dollars != 250 { // first_raid's RewardDollars
		t.Errorf("expected 250 dollars credited for first_raid, got %v", dollars)
	}
}

// TestRecordEvent_UnlocksTieredAchievementsAsCounterClimbs confirms
// multiple achievements sharing a trigger unlock independently as the
// lifetime counter crosses each of their thresholds, and that an
// already-unlocked achievement is never re-unlocked (no duplicate
// reward) on a later call.
func TestRecordEvent_UnlocksTieredAchievementsAsCounterClimbs(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	const userID = 90002
	seedProgressionPlayer(t, db, userID)

	// first_raid (1), raider_10 (10) both share TriggerRaidCompleted.
	for i := 0; i < 10; i++ {
		if err := RecordEvent(ctx, db, userID, TriggerRaidCompleted, 1); err != nil {
			t.Fatalf("RecordEvent iteration %d: unexpected error: %v", i, err)
		}
	}

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM player_achievements WHERE user_id = $1 AND achievement_key IN ('first_raid', 'raider_10')", userID).Scan(&count); err != nil {
		t.Fatalf("counting achievements: %v", err)
	}
	if count != 2 {
		t.Errorf("expected both first_raid and raider_10 unlocked after 10 raids, got %d unlocked", count)
	}

	var eventCount int
	if err := db.QueryRow("SELECT count FROM player_event_counts WHERE user_id = $1 AND trigger = $2", userID, TriggerRaidCompleted).Scan(&eventCount); err != nil {
		t.Fatalf("reading event count: %v", err)
	}
	if eventCount != 10 {
		t.Errorf("expected lifetime counter to be 10, got %d", eventCount)
	}

	// Reward should be first_raid (250) + raider_10 (1000) = 1250, not
	// paid twice for first_raid despite 10 RecordEvent calls.
	var dollars float64
	if err := db.QueryRow(`
		SELECT r.dollars FROM resources r JOIN encampments e ON e.id = r.encampment_id WHERE e.user_id = $1`, userID).Scan(&dollars); err != nil {
		t.Fatalf("reading resources: %v", err)
	}
	if dollars != 1250 {
		t.Errorf("expected exactly 1250 dollars (250+1000, no double-pay), got %v", dollars)
	}
}

// TestRecordEvent_AdvancesQuestProgressAndMarksComplete is the direct
// test for Phase 2: progress accumulates toward a quest's goal and
// completed_at is set once the goal is reached, but the reward is NOT
// auto-credited for a real player - it stays unclaimed until
// ClaimQuest is called (see the next test).
func TestRecordEvent_AdvancesQuestProgressAndMarksComplete(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	const userID = 90003
	seedProgressionPlayer(t, db, userID)

	// daily_market needs 2 TriggerMarketPurchase events.
	if err := RecordEvent(ctx, db, userID, TriggerMarketPurchase, 1); err != nil {
		t.Fatalf("RecordEvent: unexpected error: %v", err)
	}

	ps := PeriodStart(PeriodDaily)
	var progress int
	var completedAt sql.NullTime
	if err := db.QueryRow("SELECT progress, completed_at FROM player_quest_progress WHERE user_id = $1 AND quest_key = 'daily_market' AND period_start = $2", userID, ps).Scan(&progress, &completedAt); err != nil {
		t.Fatalf("reading quest progress: %v", err)
	}
	if progress != 1 {
		t.Errorf("expected progress 1 after one purchase, got %d", progress)
	}
	if completedAt.Valid {
		t.Error("expected daily_market to not yet be complete after only 1 of 2 purchases")
	}

	if err := RecordEvent(ctx, db, userID, TriggerMarketPurchase, 1); err != nil {
		t.Fatalf("RecordEvent (2nd): unexpected error: %v", err)
	}
	if err := db.QueryRow("SELECT progress, completed_at FROM player_quest_progress WHERE user_id = $1 AND quest_key = 'daily_market' AND period_start = $2", userID, ps).Scan(&progress, &completedAt); err != nil {
		t.Fatalf("reading quest progress (2nd): %v", err)
	}
	if progress != 2 {
		t.Errorf("expected progress 2 after two purchases, got %d", progress)
	}
	if !completedAt.Valid {
		t.Error("expected daily_market to be complete after 2 of 2 purchases")
	}

	// Reward should NOT be auto-credited for the QUEST specifically
	// before claiming - but note first_purchase (an achievement
	// sharing TriggerMarketPurchase, threshold 1) DOES auto-unlock and
	// pay its own 250 reward from the very first RecordEvent call
	// above, since achievement rewards are always auto-granted
	// (unlike quest rewards). 250 here is entirely first_purchase's
	// achievement payout, not any quest payout.
	var dollars float64
	_ = db.QueryRow(`SELECT r.dollars FROM resources r JOIN encampments e ON e.id = r.encampment_id WHERE e.user_id = $1`, userID).Scan(&dollars)
	if dollars != 250 {
		t.Errorf("expected exactly 250 dollars (first_purchase achievement only, no quest payout before claiming), got %v", dollars)
	}
}

// TestClaimQuest_PaysRewardOnceAndRejectsDoubleClaim covers the actual
// claim flow a real player uses from the /quests panel.
func TestClaimQuest_PaysRewardOnceAndRejectsDoubleClaim(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	const userID = 90004
	seedProgressionPlayer(t, db, userID)

	if err := RecordEvent(ctx, db, userID, TriggerScoutMissionDispatch, 1); err != nil {
		t.Fatalf("RecordEvent: unexpected error: %v", err)
	}

	if err := ClaimQuest(ctx, db, userID, "daily_scout"); err != nil {
		t.Fatalf("ClaimQuest: unexpected error: %v", err)
	}

	// Expected total: first_scout achievement (250, auto-unlocked by
	// the RecordEvent call above, threshold 1 on the same trigger) +
	// daily_scout quest reward (200, just claimed) = 450.
	var dollars float64
	if err := db.QueryRow(`SELECT r.dollars FROM resources r JOIN encampments e ON e.id = r.encampment_id WHERE e.user_id = $1`, userID).Scan(&dollars); err != nil {
		t.Fatalf("reading resources: %v", err)
	}
	if dollars != 450 {
		t.Errorf("expected 450 dollars (250 first_scout achievement + 200 daily_scout quest claim), got %v", dollars)
	}

	if err := ClaimQuest(ctx, db, userID, "daily_scout"); err == nil {
		t.Error("expected a second claim of the same completed quest to fail")
	}

	// Confirm no double-pay from the rejected second claim.
	_ = db.QueryRow(`SELECT r.dollars FROM resources r JOIN encampments e ON e.id = r.encampment_id WHERE e.user_id = $1`, userID).Scan(&dollars)
	if dollars != 450 {
		t.Errorf("expected dollars to remain 450 after a rejected double-claim, got %v", dollars)
	}
}

// TestClaimQuest_RejectsClaimBeforeCompletion confirms an incomplete
// quest can't be claimed.
func TestClaimQuest_RejectsClaimBeforeCompletion(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	const userID = 90005
	seedProgressionPlayer(t, db, userID)

	// daily_market needs 2 purchases; only record 1.
	if err := RecordEvent(ctx, db, userID, TriggerMarketPurchase, 1); err != nil {
		t.Fatalf("RecordEvent: unexpected error: %v", err)
	}

	if err := ClaimQuest(ctx, db, userID, "daily_market"); err == nil {
		t.Error("expected claiming an incomplete quest to fail")
	}
}

// TestRecordEvent_AIFactionAutoClaims is the direct test for the
// AI-parity design in NEXT_GENERATION_FEATURES_PLAN.md Phase 2: an AI
// faction (negative user_id) has no /quests panel to visit, so
// completing a quest auto-claims and credits the reward immediately
// rather than leaving it permanently stranded.
func TestRecordEvent_AIFactionAutoClaims(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	const aiUserID = -90006
	seedProgressionPlayer(t, db, aiUserID)

	if err := RecordEvent(ctx, db, aiUserID, TriggerScoutMissionDispatch, 1); err != nil {
		t.Fatalf("RecordEvent: unexpected error: %v", err)
	}

	var claimedAt sql.NullTime
	if err := db.QueryRow("SELECT claimed_at FROM player_quest_progress WHERE user_id = $1 AND quest_key = 'daily_scout'", aiUserID).Scan(&claimedAt); err != nil {
		t.Fatalf("reading quest progress: %v", err)
	}
	if !claimedAt.Valid {
		t.Error("expected an AI faction's completed quest to be auto-claimed")
	}

	var dollars float64
	if err := db.QueryRow(`SELECT r.dollars FROM resources r JOIN encampments e ON e.id = r.encampment_id WHERE e.user_id = $1`, aiUserID).Scan(&dollars); err != nil {
		t.Fatalf("reading resources: %v", err)
	}
	// first_scout achievement (250, same-trigger threshold 1) +
	// daily_scout quest auto-claim (200) = 450.
	if dollars != 450 {
		t.Errorf("expected 450 dollars (250 first_scout achievement + 200 daily_scout auto-claim), got %v", dollars)
	}
}

// TestRecordEvent_UnknownTriggerIsANoOp confirms a trigger key with no
// matching achievement or quest definition doesn't error - a safe
// no-op, so future call sites can pass a trigger that isn't yet used
// by any definition without breaking anything.
func TestRecordEvent_UnknownTriggerIsANoOp(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	const userID = 90007
	seedProgressionPlayer(t, db, userID)

	if err := RecordEvent(ctx, db, userID, "totally_unused_trigger_key", 1); err != nil {
		t.Fatalf("expected an unmatched trigger to be a safe no-op, got error: %v", err)
	}

	var achievementCount, questCount int
	_ = db.QueryRow("SELECT COUNT(*) FROM player_achievements WHERE user_id = $1", userID).Scan(&achievementCount)
	_ = db.QueryRow("SELECT COUNT(*) FROM player_quest_progress WHERE user_id = $1", userID).Scan(&questCount)
	if achievementCount != 0 || questCount != 0 {
		t.Errorf("expected no achievements/quests touched by an unmatched trigger, got %d achievements, %d quest rows", achievementCount, questCount)
	}
}
