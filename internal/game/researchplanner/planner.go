package researchplanner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

const MemoryScope = "research_planner"

var ErrNoEncampment = errors.New("researchplanner: player has no encampment")

type Planner struct {
	DB *sql.DB
	AI *ai.Service
}

func New(db *sql.DB, service *ai.Service) *Planner {
	return &Planner{DB: db, AI: service}
}

func (p *Planner) BuildSnapshot(ctx context.Context, userID int64, goal string) (*Snapshot, error) {
	var s Snapshot
	if err := p.DB.QueryRowContext(ctx, `SELECT id, name, level FROM encampments WHERE user_id = $1`, userID).
		Scan(&s.EncampmentID, &s.Name, &s.Level); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoEncampment
		}
		return nil, fmt.Errorf("researchplanner: load encampment: %w", err)
	}

	if IsValidGoal(goal) {
		s.RequestedGoal = Goal(goal)
	}

	levels, err := p.fetchResearchLevels(ctx, s.EncampmentID)
	if err != nil {
		return nil, fmt.Errorf("researchplanner: load research levels: %w", err)
	}
	s.Nodes = BuildTechNodes(levels)

	if err := p.DB.QueryRowContext(ctx, `SELECT neuro_cores FROM resources WHERE encampment_id = $1`, s.EncampmentID).
		Scan(&s.NeuroCores); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("researchplanner: load neuro cores: %w", err)
	}

	return &s, nil
}

func (p *Planner) fetchResearchLevels(ctx context.Context, campID string) (map[string]int, error) {
	levels := make(map[string]int)

	row := p.DB.QueryRowContext(ctx, `
		SELECT econ_tech_lvl, production_tech_lvl, integrity_tech_lvl,
		       defense_tech_lvl, intel_tech_lvl, speed_tech_lvl, military_tech_lvl
		FROM research_states WHERE encampment_id = $1`, campID)

	var econ, production, integrity, defense, intel, speed, military int
	err := row.Scan(&econ, &production, &integrity, &defense, &intel, &speed, &military)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = p.DB.ExecContext(ctx, "INSERT INTO research_states (encampment_id) VALUES ($1) ON CONFLICT (encampment_id) DO NOTHING", campID)
		econ, production, integrity, defense, intel, speed, military = 1, 1, 1, 1, 1, 1, 1
	} else if err != nil {
		return nil, err
	}

	levels["econ"] = econ
	levels["production"] = production
	levels["integrity"] = integrity
	levels["defense"] = defense
	levels["intel"] = intel
	levels["speed"] = speed
	levels["military"] = military

	return levels, nil
}

func (p *Planner) Recommend(ctx context.Context, userID int64, goal string) (*Recommendation, error) {
	snapshot, err := p.BuildSnapshot(ctx, userID, goal)
	if err != nil {
		return nil, err
	}

	userPrompt := BuildUserPrompt(*snapshot)

	if p.AI.Memory != nil {
		_ = p.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleUser, Content: userPrompt})
	}

	resp, err := p.AI.Complete(ctx, ai.CompletionRequest{
		Feature:     string(ai.FeatureResearchPlan),
		UserID:      userID,
		System:      SystemPrompt,
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: userPrompt}},
		MaxTokens:   4096,
		Temperature: 0.3,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("researchplanner: ai completion failed: %w", err)
	}

	rec := ParseRecommendation(resp.Text, resp.StopReason)

	if p.AI.Memory != nil {
		_ = p.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleAssistant, Content: resp.Text})
	}

	return rec, nil
}
