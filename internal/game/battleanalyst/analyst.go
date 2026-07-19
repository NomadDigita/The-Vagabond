package battleanalyst

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

// MemoryScope namespaces this feature's conversational history inside
// ai_memory, distinct from every other Phase B+ scope.
const MemoryScope = "battle_analyst"

// ErrNoEncampment is returned when the calling player has no
// registered base yet.
var ErrNoEncampment = errors.New("battleanalyst: player has no encampment")

// Analyst is the Phase F entry point.
type Analyst struct {
	DB *sql.DB
	AI *ai.Service
}

func New(db *sql.DB, service *ai.Service) *Analyst {
	return &Analyst{DB: db, AI: service}
}

// BuildSnapshot loads a player's full combat record: completed raids
// as attacker, completed raids as defender, and arena battles matched
// by their current username.
func (a *Analyst) BuildSnapshot(ctx context.Context, userID int64) (*Snapshot, error) {
	var s Snapshot
	var username string
	if err := a.DB.QueryRowContext(ctx, `
		SELECT e.id, e.name, e.level, u.username
		FROM encampments e
		JOIN users u ON u.telegram_id = e.user_id
		WHERE e.user_id = $1`, userID).
		Scan(&s.EncampmentID, &s.Name, &s.Level, &username); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoEncampment
		}
		return nil, fmt.Errorf("battleanalyst: load encampment: %w", err)
	}

	attackStats, err := a.buildAttackerStats(ctx, s.EncampmentID)
	if err != nil {
		return nil, fmt.Errorf("battleanalyst: load attacker stats: %w", err)
	}
	s.AsAttacker = attackStats

	defendStats, err := a.buildDefenderStats(ctx, s.EncampmentID)
	if err != nil {
		return nil, fmt.Errorf("battleanalyst: load defender stats: %w", err)
	}
	s.AsDefender = defendStats

	arenaStats, err := a.buildArenaStats(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("battleanalyst: load arena stats: %w", err)
	}
	s.Arena = arenaStats

	return &s, nil
}

// buildAttackerStats mirrors fleetcommander.BuildCombatHistory's exact
// query and "apparent win" heuristic (stolen resources > 0), extended
// here to also total the stolen value itself. Kept as an independent
// query rather than calling into fleetcommander — this package has no
// dependency on fleetcommander per the usual per-package isolation
// trade-off already documented in researchplanner's doc comment.
func (a *Analyst) buildAttackerStats(ctx context.Context, encampmentID string) (RaidStats, error) {
	rows, err := a.DB.QueryContext(ctx, `
		SELECT attacker_losses, (stolen_scrap + stolen_metal + stolen_crystal) AS stolen_value
		FROM raids
		WHERE attacker_id = $1 AND state = 'completed'`, encampmentID)
	if err != nil {
		return RaidStats{}, err
	}
	defer rows.Close()

	var stats RaidStats
	for rows.Next() {
		var losses int
		var stolen float64
		if err := rows.Scan(&losses, &stolen); err != nil {
			return RaidStats{}, err
		}
		stats.TotalRaids++
		stats.TotalLosses += losses
		stats.TotalStolenValue += stolen
		if stolen > 0 {
			stats.ApparentWins++
		}
	}
	if err := rows.Err(); err != nil {
		return RaidStats{}, err
	}
	if stats.TotalRaids > 0 {
		stats.AverageLosses = float64(stats.TotalLosses) / float64(stats.TotalRaids)
	}
	return stats, nil
}

// buildDefenderStats reads the same raids table from the opposite
// side. "Apparent win" for a defender is the inverse heuristic: the
// attacker walked away with nothing stolen, meaning the defense held.
func (a *Analyst) buildDefenderStats(ctx context.Context, encampmentID string) (RaidStats, error) {
	rows, err := a.DB.QueryContext(ctx, `
		SELECT defender_losses, (stolen_scrap + stolen_metal + stolen_crystal) AS stolen_value
		FROM raids
		WHERE defender_id = $1 AND state = 'completed'`, encampmentID)
	if err != nil {
		return RaidStats{}, err
	}
	defer rows.Close()

	var stats RaidStats
	for rows.Next() {
		var losses int
		var stolen float64
		if err := rows.Scan(&losses, &stolen); err != nil {
			return RaidStats{}, err
		}
		stats.TotalRaids++
		stats.TotalLosses += losses
		if stolen == 0 {
			stats.ApparentWins++
		}
	}
	if err := rows.Err(); err != nil {
		return RaidStats{}, err
	}
	if stats.TotalRaids > 0 {
		stats.AverageLosses = float64(stats.TotalLosses) / float64(stats.TotalRaids)
	}
	return stats, nil
}

// buildArenaStats matches arena_battles by the player's *current*
// username, since that table stores usernames as plain strings rather
// than a user_id foreign key (see cmd/bot/main.go's CREATE TABLE). A
// player who changed their Telegram username after a past arena
// battle will have that older battle silently excluded here — a real,
// known limitation of the underlying schema, not something this
// package can work around, and flagged as such in
// PROJECT_MASTER_PLAN.md's Known Issues (§4) alongside the similar
// win/loss heuristic already documented for raids.
func (a *Analyst) buildArenaStats(ctx context.Context, username string) (ArenaStats, error) {
	var stats ArenaStats
	if username == "" {
		return stats, nil
	}
	if err := a.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM arena_battles WHERE winner_username = $1`, username).Scan(&stats.Wins); err != nil {
		return ArenaStats{}, err
	}
	if err := a.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM arena_battles WHERE loser_username = $1`, username).Scan(&stats.Losses); err != nil {
		return ArenaStats{}, err
	}
	return stats, nil
}

// Recommend produces a fresh AI analysis of the given player's combat
// record. It stores both turns in ai_memory under MemoryScope.
//
// Read-only: nothing in this method changes any raid, arena battle, or
// unit — it only reads and summarizes what already happened.
func (a *Analyst) Recommend(ctx context.Context, userID int64) (*Recommendation, error) {
	snapshot, err := a.BuildSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	userPrompt := BuildUserPrompt(*snapshot)

	if a.AI.Memory != nil {
		_ = a.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleUser, Content: userPrompt})
	}

	resp, err := a.AI.Complete(ctx, ai.CompletionRequest{
		Feature:     string(ai.FeatureBattleAnalyst),
		UserID:      userID,
		System:      SystemPrompt,
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: userPrompt}},
		MaxTokens:   2048,
		Temperature: 0.3,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("battleanalyst: ai completion failed: %w", err)
	}

	rec := ParseRecommendation(resp.Text)

	if a.AI.Memory != nil {
		_ = a.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleAssistant, Content: resp.Text})
	}

	return rec, nil
}
