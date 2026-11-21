package progression

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
)

type Querier interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

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

type AchievementDef struct {
	Key           string
	Name          string
	Description   string
	Icon          string
	Category      string
	Trigger       string
	Threshold     int
	RewardDollars float64
}

var Achievements = []AchievementDef{
	{Key: "first_raid", Name: "First Blood", Description: "Complete your first raid.", Icon: "⚔️", Category: "combat", Trigger: TriggerRaidCompleted, Threshold: 1, RewardDollars: 250},
	{Key: "raider_10", Name: "Seasoned Raider", Description: "Complete 10 raids.", Icon: "⚔️", Category: "combat", Trigger: TriggerRaidCompleted, Threshold: 10, RewardDollars: 1000},
	{Key: "raider_50", Name: "Wasteland Warlord", Description: "Complete 50 raids.", Icon: "⚔️", Category: "combat", Trigger: TriggerRaidCompleted, Threshold: 50, RewardDollars: 5000},
	{Key: "conqueror", Name: "Conqueror", Description: "Deliver the finishing blow to an AI faction.", Icon: "🪦", Category: "combat", Trigger: TriggerFactionDefeated, Threshold: 1, RewardDollars: 2000},

	{Key: "first_purchase", Name: "First Trade", Description: "Buy your first listing on the market exchange.", Icon: "🛍️", Category: "economy", Trigger: TriggerMarketPurchase, Threshold: 1, RewardDollars: 250},
	{Key: "trader_25", Name: "Market Regular", Description: "Complete 25 market purchases.", Icon: "🛍️", Category: "economy", Trigger: TriggerMarketPurchase, Threshold: 25, RewardDollars: 1500},
	{Key: "trader_100", Name: "Trade Baron", Description: "Complete 100 market purchases.", Icon: "🛍️", Category: "economy", Trigger: TriggerMarketPurchase, Threshold: 100, RewardDollars: 6000},
	{Key: "relic_hunter", Name: "Relic Hunter", Description: "Claim a Relic Convoy.", Icon: "🏺", Category: "economy", Trigger: TriggerRelicClaimed, Threshold: 1, RewardDollars: 3000},

	{Key: "first_scout", Name: "First Steps", Description: "Dispatch your first scout mission.", Icon: "🔭", Category: "exploration", Trigger: TriggerScoutMissionDispatch, Threshold: 1, RewardDollars: 250},
	{Key: "scout_10", Name: "Wasteland Cartographer", Description: "Dispatch 10 scout missions.", Icon: "🔭", Category: "exploration", Trigger: TriggerScoutMissionDispatch, Threshold: 10, RewardDollars: 1500},
	{Key: "scout_50", Name: "Master Pathfinder", Description: "Dispatch 50 scout missions.", Icon: "🔭", Category: "exploration", Trigger: TriggerScoutMissionDispatch, Threshold: 50, RewardDollars: 6000},

	{Key: "first_clan", Name: "Joined the Cause", Description: "Join a Clan.", Icon: "🏴", Category: "social", Trigger: TriggerClanJoined, Threshold: 1, RewardDollars: 500},
	{Key: "constellation_founder", Name: "Star Aligner", Description: "Form or join an active Constellation Alliance.", Icon: "✨", Category: "social", Trigger: TriggerConstellationFormed, Threshold: 1, RewardDollars: 3000},
	{Key: "generous_1", Name: "Generous Outpost", Description: "Make your first Clan donation.", Icon: "🎁", Category: "social", Trigger: TriggerClanDonation, Threshold: 1, RewardDollars: 250},
	{Key: "generous_25", Name: "Clan Pillar", Description: "Make 25 Clan donations.", Icon: "🎁", Category: "social", Trigger: TriggerClanDonation, Threshold: 25, RewardDollars: 3000},

	{Key: "raider_200", Name: "Living Legend", Description: "Complete 200 raids.", Icon: "🏆", Category: "combat", Trigger: TriggerRaidCompleted, Threshold: 200, RewardDollars: 15000},
	{Key: "trader_500", Name: "Exchange Tycoon", Description: "Complete 500 market purchases.", Icon: "🏆", Category: "economy", Trigger: TriggerMarketPurchase, Threshold: 500, RewardDollars: 20000},
	{Key: "scout_200", Name: "Wasteland Oracle", Description: "Dispatch 200 scout missions.", Icon: "🏆", Category: "exploration", Trigger: TriggerScoutMissionDispatch, Threshold: 200, RewardDollars: 20000},
}

type QuestPeriod string

const (
	PeriodDaily  QuestPeriod = "daily"
	PeriodWeekly QuestPeriod = "weekly"
	PeriodOnce   QuestPeriod = "once"
)

type QuestDef struct {
	Key                string
	Description        string
	Period             QuestPeriod
	Trigger            string
	GoalAmount         int
	RewardDollars      float64
	RewardResourceType string
	RewardResourceAmt  float64
}

var Quests = []QuestDef{
	{Key: "daily_raid", Description: "Complete 1 raid.", Period: PeriodDaily, Trigger: TriggerRaidCompleted, GoalAmount: 1, RewardDollars: 200},
	{Key: "daily_market", Description: "Complete 2 market purchases.", Period: PeriodDaily, Trigger: TriggerMarketPurchase, GoalAmount: 2, RewardDollars: 200},
	{Key: "daily_scout", Description: "Dispatch 1 scout mission.", Period: PeriodDaily, Trigger: TriggerScoutMissionDispatch, GoalAmount: 1, RewardDollars: 200},
	{Key: "daily_donate", Description: "Make 1 Clan donation.", Period: PeriodDaily, Trigger: TriggerClanDonation, GoalAmount: 1, RewardDollars: 200},

	{Key: "weekly_raid", Description: "Complete 10 raids.", Period: PeriodWeekly, Trigger: TriggerRaidCompleted, GoalAmount: 10, RewardDollars: 2500},
	{Key: "weekly_market", Description: "Complete 10 market purchases.", Period: PeriodWeekly, Trigger: TriggerMarketPurchase, GoalAmount: 10, RewardDollars: 2500},

	{Key: "onboarding_raid", Description: "🎯 Getting Started: Launch your first raid.", Period: PeriodOnce, Trigger: TriggerRaidCompleted, GoalAmount: 1, RewardDollars: 500},
	{Key: "onboarding_research", Description: "🎯 Getting Started: Upgrade a research track.", Period: PeriodOnce, Trigger: TriggerResearchUpgrade, GoalAmount: 1, RewardDollars: 500},
	{Key: "onboarding_clan", Description: "🎯 Getting Started: Join or found a Clan.", Period: PeriodOnce, Trigger: TriggerClanJoined, GoalAmount: 1, RewardDollars: 500},
	{Key: "onboarding_market", Description: "🎯 Getting Started: Make your first market purchase.", Period: PeriodOnce, Trigger: TriggerMarketPurchase, GoalAmount: 1, RewardDollars: 500},
}

func PeriodStart(period QuestPeriod) time.Time {
	return periodStart(period, time.Now())
}

func periodStart(period QuestPeriod, now time.Time) time.Time {
	if period == PeriodOnce {
		return time.Unix(0, 0).UTC()
	}
	now = now.UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	if period == PeriodDaily {
		return today
	}
	offset := (int(today.Weekday()) + 6) % 7
	return today.AddDate(0, 0, -offset)
}

func campIDFor(ctx context.Context, q Querier, userID int64) string {
	var campID string
	_ = q.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", userID).Scan(&campID)
	return campID
}

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
	if userID > 0 {
		_ = notifications.Queue(ctx, q, userID, message, "general")
	}
}

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
