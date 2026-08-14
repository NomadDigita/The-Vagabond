// Package progression is the shared engine behind
// NEXT_GENERATION_FEATURES_PLAN.md Phase 1 (Achievements & Milestones)
// and Phase 2 (Daily/Weekly Quest Board) - built together per that
// document's own suggested build order, since both are event-driven
// off the same real action call sites.
//
// Definitions (which achievements/quests exist, what triggers them,
// what they reward) are static Go data, not DB tables - the same
// convention this codebase already uses for other seed-flavor content
// (weather.go's eventPool, relicconvoys.go's relicNames,
// factiondefeat.go's epitaphTemplates). Only per-player progress is
// persisted.
//
// A single call, RecordEvent, is the one integration point every real
// action handler needs - it updates the player's lifetime counter for
// a trigger, unlocks any achievement whose threshold is now met, and
// advances any active quest sharing that trigger as its goal type, all
// in one round trip. Callers don't need to know which achievements or
// quests exist, only which trigger key their action corresponds to.
package progression

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
)

// Querier is satisfied by both *sql.DB and *sql.Tx, matching the same
// convention notifications.querier and storagecap.Queryer already
// establish - callers already holding an open transaction (most tick-
// engine call sites) reuse it directly; handler call sites without one
// (most bot handlers) pass their *sql.DB.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

// Trigger keys - the vocabulary every RecordEvent call site and every
// AchievementDef/QuestDef below share. Adding a new achievement or
// quest that reuses an EXISTING trigger needs zero new call sites;
// adding one that needs a genuinely new signal does need one new
// RecordEvent call at the relevant action's completion point.
const (
	TriggerRaidCompleted        = "raid_completed"
	TriggerMarketPurchase       = "market_purchase"
	TriggerScoutMissionDispatch = "scout_mission_dispatch"
	TriggerClanJoined           = "clan_joined"
	TriggerRelicClaimed         = "relic_claimed"
	TriggerFactionDefeated      = "faction_defeated"
	TriggerConstellationFormed  = "constellation_formed"
	TriggerClanDonation         = "clan_donation"
	TriggerResearchUpgrade      = "research_upgrade"
)

// AchievementDef is a permanent, one-time (per player) unlock. Trigger
// + Threshold together mean "unlock once the player's lifetime counter
// for Trigger reaches Threshold" - a Threshold of 1 is a simple
// one-shot achievement, a higher Threshold is a milestone.
type AchievementDef struct {
	Key           string
	Name          string
	Description   string
	Icon          string
	Category      string // combat, economy, exploration, social
	Trigger       string
	Threshold     int
	RewardDollars float64
}

// Achievements is the full launch catalog - deliberately a smaller,
// real, fully-wired set (18) rather than the plan doc's illustrative
// "40-50" figure, matching this project's own established practice of
// shipping a genuine v1 and expanding data-only later (adding another
// entry here needs no new code if it reuses an existing Trigger).
var Achievements = []AchievementDef{
	// --- Combat ---
	{Key: "first_raid", Name: "First Blood", Description: "Complete your first raid.", Icon: "⚔️", Category: "combat", Trigger: TriggerRaidCompleted, Threshold: 1, RewardDollars: 250},
	{Key: "raider_10", Name: "Seasoned Raider", Description: "Complete 10 raids.", Icon: "⚔️", Category: "combat", Trigger: TriggerRaidCompleted, Threshold: 10, RewardDollars: 1000},
	{Key: "raider_50", Name: "Wasteland Warlord", Description: "Complete 50 raids.", Icon: "⚔️", Category: "combat", Trigger: TriggerRaidCompleted, Threshold: 50, RewardDollars: 5000},
	{Key: "conqueror", Name: "Conqueror", Description: "Deliver the finishing blow to an AI faction.", Icon: "🪦", Category: "combat", Trigger: TriggerFactionDefeated, Threshold: 1, RewardDollars: 2000},

	// --- Economy ---
	{Key: "first_purchase", Name: "First Trade", Description: "Buy your first listing on the market exchange.", Icon: "🛍️", Category: "economy", Trigger: TriggerMarketPurchase, Threshold: 1, RewardDollars: 250},
	{Key: "trader_25", Name: "Market Regular", Description: "Complete 25 market purchases.", Icon: "🛍️", Category: "economy", Trigger: TriggerMarketPurchase, Threshold: 25, RewardDollars: 1500},
	{Key: "trader_100", Name: "Trade Baron", Description: "Complete 100 market purchases.", Icon: "🛍️", Category: "economy", Trigger: TriggerMarketPurchase, Threshold: 100, RewardDollars: 6000},
	{Key: "relic_hunter", Name: "Relic Hunter", Description: "Claim a Relic Convoy.", Icon: "🏺", Category: "economy", Trigger: TriggerRelicClaimed, Threshold: 1, RewardDollars: 3000},

	// --- Exploration ---
	{Key: "first_scout", Name: "First Steps", Description: "Dispatch your first scout mission.", Icon: "🔭", Category: "exploration", Trigger: TriggerScoutMissionDispatch, Threshold: 1, RewardDollars: 250},
	{Key: "scout_10", Name: "Wasteland Cartographer", Description: "Dispatch 10 scout missions.", Icon: "🔭", Category: "exploration", Trigger: TriggerScoutMissionDispatch, Threshold: 10, RewardDollars: 1500},
	{Key: "scout_50", Name: "Master Pathfinder", Description: "Dispatch 50 scout missions.", Icon: "🔭", Category: "exploration", Trigger: TriggerScoutMissionDispatch, Threshold: 50, RewardDollars: 6000},

	// --- Social ---
	{Key: "first_clan", Name: "Joined the Cause", Description: "Join a Clan.", Icon: "🏴", Category: "social", Trigger: TriggerClanJoined, Threshold: 1, RewardDollars: 500},
	{Key: "constellation_founder", Name: "Star Aligner", Description: "Form or join an active Constellation Alliance.", Icon: "✨", Category: "social", Trigger: TriggerConstellationFormed, Threshold: 1, RewardDollars: 3000},
	{Key: "generous_1", Name: "Generous Outpost", Description: "Make your first Clan donation.", Icon: "🎁", Category: "social", Trigger: TriggerClanDonation, Threshold: 1, RewardDollars: 250},
	{Key: "generous_25", Name: "Clan Pillar", Description: "Make 25 Clan donations.", Icon: "🎁", Category: "social", Trigger: TriggerClanDonation, Threshold: 25, RewardDollars: 3000},

	// --- Combined-arms milestones (deliberately reuse existing
	// triggers at higher thresholds rather than inventing new ones,
	// demonstrating the "adding an achievement needs no new code"
	// property this package is designed around) ---
	{Key: "raider_200", Name: "Living Legend", Description: "Complete 200 raids.", Icon: "🏆", Category: "combat", Trigger: TriggerRaidCompleted, Threshold: 200, RewardDollars: 15000},
	{Key: "trader_500", Name: "Exchange Tycoon", Description: "Complete 500 market purchases.", Icon: "🏆", Category: "economy", Trigger: TriggerMarketPurchase, Threshold: 500, RewardDollars: 20000},
	{Key: "scout_200", Name: "Wasteland Oracle", Description: "Dispatch 200 scout missions.", Icon: "🏆", Category: "exploration", Trigger: TriggerScoutMissionDispatch, Threshold: 200, RewardDollars: 20000},
}

// QuestPeriod is "daily" or "weekly" - determines how a quest's
// period_start is computed (see periodStart below) and therefore how
// often its progress resets.
type QuestPeriod string

const (
	PeriodDaily  QuestPeriod = "daily"
	PeriodWeekly QuestPeriod = "weekly"
	// PeriodOnce is NEXT_GENERATION_FEATURES_PLAN.md Phase 17's
	// "Getting Started" checklist - a one-time onboarding quest chain
	// that reuses this same progress-tracking machinery instead of
	// building a third one. Its period_start never advances (see
	// periodStart below), so a PeriodOnce quest's single
	// player_quest_progress row is permanent, not reset like
	// daily/weekly quests are.
	PeriodOnce QuestPeriod = "once"
)

// QuestDef is a repeatable, period-scoped goal. Unlike achievements,
// completing a quest doesn't auto-grant its reward for a real player -
// see ClaimQuest below - a real player has to visit /quests and tap
// Claim, giving the panel a reason to be visited. An AI faction
// (negative user_id) has no panel to visit, so RecordEvent auto-claims
// on its behalf at the moment of completion instead of leaving it
// permanently unclaimed - see RecordEvent's own comment.
type QuestDef struct {
	Key                string
	Description        string
	Period             QuestPeriod
	Trigger            string
	GoalAmount         int
	RewardDollars      float64
	RewardResourceType string  // "" if the reward is dollars-only
	RewardResourceAmt  float64 // ignored if RewardResourceType == ""
}

// Quests is the fixed daily/weekly board - deliberately a fixed set
// shown every period rather than a randomly-rerolled 3-of-N subset
// (NEXT_GENERATION_FEATURES_PLAN.md's Phase 2 draft described
// rotation; this is a documented scope simplification for the initial
// build - see that doc's Phase 2 status update after this commit).
var Quests = []QuestDef{
	{Key: "daily_raid", Description: "Complete 1 raid.", Period: PeriodDaily, Trigger: TriggerRaidCompleted, GoalAmount: 1, RewardDollars: 200},
	{Key: "daily_market", Description: "Complete 2 market purchases.", Period: PeriodDaily, Trigger: TriggerMarketPurchase, GoalAmount: 2, RewardDollars: 200},
	{Key: "daily_scout", Description: "Dispatch 1 scout mission.", Period: PeriodDaily, Trigger: TriggerScoutMissionDispatch, GoalAmount: 1, RewardDollars: 200},
	{Key: "daily_donate", Description: "Make 1 Clan donation.", Period: PeriodDaily, Trigger: TriggerClanDonation, GoalAmount: 1, RewardDollars: 200},

	{Key: "weekly_raid", Description: "Complete 10 raids.", Period: PeriodWeekly, Trigger: TriggerRaidCompleted, GoalAmount: 10, RewardDollars: 2500},
	{Key: "weekly_market", Description: "Complete 10 market purchases.", Period: PeriodWeekly, Trigger: TriggerMarketPurchase, GoalAmount: 10, RewardDollars: 2500},

	// --- NEXT_GENERATION_FEATURES_PLAN.md Phase 17: "Getting Started"
	// checklist. A one-time (PeriodOnce - never resets) onboarding
	// quest chain walking a new player through the four actions this
	// project's own onboarding audit found were the biggest early
	// systems a brand-new survivor might never discover organically:
	// their first raid, first research upgrade, first Clan join, and
	// first market interaction. Deliberately reuses the exact same
	// trigger keys the daily/weekly board and several achievements
	// already use - a new player completing these also makes progress
	// on "first_raid"/"first_clan"/etc. achievements and any active
	// daily quest sharing the same trigger, for free, in the same
	// RecordEvent call.
	{Key: "onboarding_raid", Description: "🎯 Getting Started: Launch your first raid.", Period: PeriodOnce, Trigger: TriggerRaidCompleted, GoalAmount: 1, RewardDollars: 500},
	{Key: "onboarding_research", Description: "🎯 Getting Started: Upgrade a research track.", Period: PeriodOnce, Trigger: TriggerResearchUpgrade, GoalAmount: 1, RewardDollars: 500},
	{Key: "onboarding_clan", Description: "🎯 Getting Started: Join or found a Clan.", Period: PeriodOnce, Trigger: TriggerClanJoined, GoalAmount: 1, RewardDollars: 500},
	{Key: "onboarding_market", Description: "🎯 Getting Started: Make your first market purchase.", Period: PeriodOnce, Trigger: TriggerMarketPurchase, GoalAmount: 1, RewardDollars: 500},
}

// PeriodStart is the exported form of periodStart, for display-only
// lookups from outside this package (the /quests panel needs to query
// player_quest_progress by the same period_start key RecordEvent
// writes with, without duplicating the Monday-boundary math).
func PeriodStart(period QuestPeriod) time.Time {
	return periodStart(period, time.Now())
}

// periodStart returns the boundary timestamp for a quest's current
// period - the start of today (UTC) for daily quests, the most recent
// Monday 00:00 UTC for weekly quests. Used as part of
// player_quest_progress's composite key, so a new period is naturally
// a fresh row rather than needing a separate rollover/reset tick phase.
func periodStart(period QuestPeriod, now time.Time) time.Time {
	if period == PeriodOnce {
		// A fixed, never-advancing boundary - every PeriodOnce quest
		// keys to the same single row forever, rather than a fresh row
		// each period the way daily/weekly quests do.
		return time.Unix(0, 0).UTC()
	}
	now = now.UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if period == PeriodDaily {
		return today
	}
	// ISO week: Monday start. time.Weekday Sunday=0 ... Saturday=6.
	offset := (int(today.Weekday()) + 6) % 7 // days since Monday
	return today.AddDate(0, 0, -offset)
}

// campIDFor resolves a userID's encampment id - needed to credit
// reward dollars via storagecap. Returns "" if not found (e.g. an AI
// faction whose row was deleted, or a malformed userID) - callers
// treat that as "nothing to credit" rather than an error, matching
// this codebase's general "best-effort side-channel, never block the
// action it's attached to" convention for achievement/notification
// side effects.
func campIDFor(ctx context.Context, q Querier, userID int64) string {
	var campID string
	_ = q.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", userID).Scan(&campID)
	return campID
}

// creditReward pays dollars (storage-cap clamped, same as every other
// resource-gain path in this codebase) to userID's encampment and
// queues a notification. Silently does nothing if the encampment can't
// be resolved.
func creditReward(ctx context.Context, q Querier, userID int64, dollars float64, message string) {
	campID := campIDFor(ctx, q, userID)
	if campID == "" {
		return
	}
	var current float64
	_ = q.QueryRowContext(ctx, "SELECT dollars FROM resources WHERE encampment_id = $1", campID).Scan(&current)
	storageCap := storagecap.CapFor(ctx, q, campID)
	newDollars, _ := storagecap.Clamp(current, dollars, storageCap)
	_, _ = q.ExecContext(ctx, "UPDATE resources SET dollars = $1 WHERE encampment_id = $2", newDollars, campID)
	if userID > 0 { // only real players read notifications
		_ = notifications.Queue(ctx, q, userID, message, "general")
	}
}

// RecordEvent is the single integration point every real action
// handler calls at its own natural completion point (e.g.
// doBuyListingQty after a successful purchase, doDispatchScoutMission
// after dispatch, checkForFactionDefeat when crediting a real finishing
// attacker). It:
//
//  1. Increments the player's lifetime counter for trigger by amount.
//  2. Unlocks any Achievements whose Threshold is now met and not
//     already unlocked, crediting RewardDollars and notifying.
//  3. Advances progress on any currently-active Quest sharing this
//     trigger, marking it completed once GoalAmount is reached. For a
//     real player (positive userID), the reward sits unclaimed until
//     they tap Claim on /quests - see ClaimQuest. For an AI faction
//     (negative userID, which has no panel to visit), completion
//     auto-claims immediately so the reward isn't permanently stranded -
//     matching NEXT_GENERATION_FEATURES_PLAN.md Phase 2's AI-parity
//     note.
//
// Errors are logged-and-swallowed by callers (this is always called
// with `_ = progression.RecordEvent(...)`), matching this codebase's
// established convention for notification/side-effect calls that
// should never block or roll back the primary action they're attached
// to.
func RecordEvent(ctx context.Context, q Querier, userID int64, trigger string, amount int) error {
	var newCount int
	if err := q.QueryRowContext(ctx, `
		INSERT INTO player_event_counts (user_id, trigger, count)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, trigger) DO UPDATE SET count = player_event_counts.count + $3
		RETURNING count`, userID, trigger, amount).Scan(&newCount); err != nil {
		return fmt.Errorf("updating event count: %w", err)
	}

	for _, a := range Achievements {
		if a.Trigger != trigger || newCount < a.Threshold {
			continue
		}
		result, err := q.ExecContext(ctx, `
			INSERT INTO player_achievements (user_id, achievement_key, unlocked_at)
			VALUES ($1, $2, CURRENT_TIMESTAMP) ON CONFLICT DO NOTHING`, userID, a.Key)
		if err != nil {
			return fmt.Errorf("unlocking achievement %s: %w", a.Key, err)
		}
		if affected, _ := result.RowsAffected(); affected > 0 {
			msg := fmt.Sprintf("🏆 %s: %s %s - %s. +%s awarded!",
				"ACHIEVEMENT UNLOCKED", a.Icon, a.Name, a.Description, fmt.Sprintf("$%.0f", a.RewardDollars))
			creditReward(ctx, q, userID, a.RewardDollars, msg)
		}
	}

	now := time.Now()
	for _, qd := range Quests {
		if qd.Trigger != trigger {
			continue
		}
		ps := periodStart(qd.Period, now)
		var progress int
		var completedAt sql.NullTime
		err := q.QueryRowContext(ctx, `
			INSERT INTO player_quest_progress (user_id, quest_key, period_start, progress)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (user_id, quest_key, period_start) DO UPDATE SET progress = player_quest_progress.progress + $4
			RETURNING progress, completed_at`, userID, qd.Key, ps, amount).Scan(&progress, &completedAt)
		if err != nil {
			return fmt.Errorf("updating quest progress %s: %w", qd.Key, err)
		}
		if progress >= qd.GoalAmount && !completedAt.Valid {
			if _, err := q.ExecContext(ctx, `
				UPDATE player_quest_progress SET completed_at = CURRENT_TIMESTAMP
				WHERE user_id = $1 AND quest_key = $2 AND period_start = $3`, userID, qd.Key, ps); err != nil {
				return fmt.Errorf("completing quest %s: %w", qd.Key, err)
			}
			if userID < 0 {
				if err := claimQuestLocked(ctx, q, userID, qd, ps); err != nil {
					return fmt.Errorf("auto-claiming AI quest %s: %w", qd.Key, err)
				}
			}
		}
	}
	return nil
}

// claimQuestLocked grants a quest's reward and marks it claimed - the
// shared core behind both RecordEvent's AI auto-claim path and
// ClaimQuest's real-player panel path below.
func claimQuestLocked(ctx context.Context, q Querier, userID int64, qd QuestDef, ps time.Time) error {
	if _, err := q.ExecContext(ctx, `
		UPDATE player_quest_progress SET claimed_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND quest_key = $2 AND period_start = $3`, userID, qd.Key, ps); err != nil {
		return err
	}
	msg := fmt.Sprintf("📋 QUEST COMPLETE: %s - +%s awarded!", qd.Description, fmt.Sprintf("$%.0f", qd.RewardDollars))
	creditReward(ctx, q, userID, qd.RewardDollars, msg)
	if qd.RewardResourceType != "" {
		campID := campIDFor(ctx, q, userID)
		if campID != "" {
			var current float64
			_ = q.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM resources WHERE encampment_id = $1", qd.RewardResourceType), campID).Scan(&current)
			storageCap := storagecap.CapFor(ctx, q, campID)
			newVal, _ := storagecap.Clamp(current, qd.RewardResourceAmt, storageCap)
			_, _ = q.ExecContext(ctx, fmt.Sprintf("UPDATE resources SET %s = $1 WHERE encampment_id = $2", qd.RewardResourceType), newVal, campID)
		}
	}
	return nil
}

// ClaimQuest is called from the /quests panel's Claim button for a
// real player. Returns an error if the quest isn't complete, doesn't
// exist, or was already claimed - the handler translates that into a
// user-facing message.
func ClaimQuest(ctx context.Context, q Querier, userID int64, questKey string) error {
	var qd *QuestDef
	for i := range Quests {
		if Quests[i].Key == questKey {
			qd = &Quests[i]
			break
		}
	}
	if qd == nil {
		return fmt.Errorf("unknown quest %s", questKey)
	}
	ps := periodStart(qd.Period, time.Now())

	var completedAt, claimedAt sql.NullTime
	if err := q.QueryRowContext(ctx, `
		SELECT completed_at, claimed_at FROM player_quest_progress
		WHERE user_id = $1 AND quest_key = $2 AND period_start = $3`, userID, questKey, ps).Scan(&completedAt, &claimedAt); err != nil {
		return fmt.Errorf("quest not found or not started: %w", err)
	}
	if !completedAt.Valid {
		return fmt.Errorf("quest not yet complete")
	}
	if claimedAt.Valid {
		return fmt.Errorf("quest already claimed")
	}
	return claimQuestLocked(ctx, q, userID, *qd, ps)
}
