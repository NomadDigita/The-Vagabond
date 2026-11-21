package guildassistant

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

const MemoryScope = "guild_assistant"

const memberCap = 15

const inactivityLookbackDays = 7

var (
	ErrNoClan    = errors.New("guildassistant: user is not in a clan")
	ErrNotLeader = errors.New("guildassistant: user is not the clan leader")
)

type Assistant struct {
	DB *sql.DB
	AI *ai.Service
}

func New(db *sql.DB, service *ai.Service) *Assistant {
	return &Assistant{DB: db, AI: service}
}

func (a *Assistant) BuildSnapshot(ctx context.Context, userID int64) (*Snapshot, error) {
	var s Snapshot
	var role string
	if err := a.DB.QueryRowContext(ctx, `
		SELECT c.id, c.name, c.recruiting, uc.role
		FROM user_clans uc
		JOIN clans c ON c.id = uc.clan_id
		WHERE uc.user_id = $1`, userID).
		Scan(&s.ClanID, &s.Name, &s.Recruiting, &role); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoClan
		}
		return nil, fmt.Errorf("guildassistant: load clan: %w", err)
	}
	if role != "Leader" {
		return nil, ErrNotLeader
	}
	s.MemberCap = memberCap

	if err := a.DB.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(e.level), 0),
			COALESCE(SUM(w.soldiers), 0) * 10 + COALESCE(SUM(w.mechs), 0) * 150,
			COUNT(*) FILTER (WHERE u.last_active < CURRENT_TIMESTAMP - ($2 || ' days')::interval)
		FROM user_clans uc
		JOIN users u ON u.telegram_id = uc.user_id
		JOIN encampments e ON e.user_id = uc.user_id
		LEFT JOIN workshop_inventory w ON w.encampment_id = e.id
		WHERE uc.clan_id = $1`, s.ClanID, inactivityLookbackDays).
		Scan(&s.MemberCount, &s.CombinedLevel, &s.MilitaryPower, &s.InactiveMembers); err != nil {
		return nil, fmt.Errorf("guildassistant: load roster stats: %w", err)
	}

	applicants, err := a.buildApplicants(ctx, s.ClanID)
	if err != nil {
		return nil, fmt.Errorf("guildassistant: load applicants: %w", err)
	}
	s.PendingApplicants = applicants

	war, err := a.buildWarRecord(ctx, s.ClanID)
	if err != nil {
		return nil, fmt.Errorf("guildassistant: load war record: %w", err)
	}
	s.War = war

	return &s, nil
}

func (a *Assistant) buildApplicants(ctx context.Context, clanID string) ([]Applicant, error) {
	rows, err := a.DB.QueryContext(ctx, `
		SELECT u.username, COALESCE(e.level, 0)
		FROM clan_applications ca
		JOIN users u ON u.telegram_id = ca.user_id
		LEFT JOIN encampments e ON e.user_id = ca.user_id
		WHERE ca.clan_id = $1 AND ca.status = 'pending'`, clanID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var applicants []Applicant
	for rows.Next() {
		var app Applicant
		if err := rows.Scan(&app.Username, &app.Level); err != nil {
			return nil, err
		}
		applicants = append(applicants, app)
	}
	return applicants, rows.Err()
}

func (a *Assistant) buildWarRecord(ctx context.Context, clanID string) (WarRecord, error) {
	var w WarRecord

	activeQuery := `
		SELECT CASE WHEN cw.clan_a_id = $1 THEN cb.name ELSE ca.name END,
		       CASE WHEN cw.clan_a_id = $1 THEN cw.score_a ELSE cw.score_b END,
		       CASE WHEN cw.clan_a_id = $1 THEN cw.score_b ELSE cw.score_a END
		FROM clan_wars cw
		JOIN clans ca ON ca.id = cw.clan_a_id
		JOIN clans cb ON cb.id = cw.clan_b_id
		WHERE (cw.clan_a_id = $1 OR cw.clan_b_id = $1) AND cw.status = 'active'
		LIMIT 1`
	if err := a.DB.QueryRowContext(ctx, activeQuery, clanID).Scan(&w.OpponentName, &w.OurScore, &w.TheirScore); err == nil {
		w.InActiveWar = true
	} else if !errors.Is(err, sql.ErrNoRows) {
		return WarRecord{}, err
	}

	rows, err := a.DB.QueryContext(ctx, `
		SELECT CASE WHEN clan_a_id = $1 THEN score_a ELSE score_b END,
		       CASE WHEN clan_a_id = $1 THEN score_b ELSE score_a END
		FROM clan_wars
		WHERE (clan_a_id = $1 OR clan_b_id = $1) AND status = 'completed'`, clanID)
	if err != nil {
		return WarRecord{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var ourScore, theirScore float64
		if err := rows.Scan(&ourScore, &theirScore); err != nil {
			return WarRecord{}, err
		}
		w.CompletedWars++
		if ourScore > theirScore {
			w.Wins++
		} else if theirScore > ourScore {
			w.Losses++
		}
	}
	if err := rows.Err(); err != nil {
		return WarRecord{}, err
	}

	return w, nil
}

func (a *Assistant) Recommend(ctx context.Context, userID int64) (*Recommendation, error) {
	snapshot, err := a.BuildSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	userPrompt := BuildUserPrompt(*snapshot)

	if a.AI.Memory != nil {
		_ = a.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleUser, Content: userPrompt})
	}

	resp, err := a.AI.Complete(ctx, ai.CompletionRequest{
		Feature:     string(ai.FeatureGuildAssistant),
		UserID:      userID,
		System:      SystemPrompt,
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: userPrompt}},
		MaxTokens:   4096,
		Temperature: 0.3,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("guildassistant: ai completion failed: %w", err)
	}

	rec := ParseRecommendation(resp.Text, resp.StopReason)

	if a.AI.Memory != nil {
		_ = a.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleAssistant, Content: resp.Text})
	}

	return rec, nil
}
