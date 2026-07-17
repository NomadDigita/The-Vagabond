package researchplanner

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

// MemoryScope namespaces this feature's conversational history inside
// ai_memory, distinct from every other Phase B+ scope.
const MemoryScope = "research_planner"

// ErrNoEncampment is returned when the calling player has no
// registered base yet.
var ErrNoEncampment = errors.New("researchplanner: player has no encampment")

// Planner is the Phase E entry point.
type Planner struct {
	DB *sql.DB
	AI *ai.Service
}

func New(db *sql.DB, service *ai.Service) *Planner {
	return &Planner{DB: db, AI: service}
}

// BuildSnapshot loads the current tech tree state of a player's base.
// goal may be empty ("infer it") or one of ValidGoals().
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

// fetchResearchLevels reads all 7 tech levels for an encampment,
// initializing the row if it doesn't exist yet — matching
// handlers.ResearchHandler.fetchResearchLevels exactly so the two
// never disagree about a brand-new player's starting state.
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

// Recommend produces a fresh AI recommendation for the given player's
// research order. It stores both turns in ai_memory under MemoryScope.
//
// Advisory-only: nothing in this method spends a single Neuro Core or
// otherwise mutates research_states/resources — that stays exclusively
// in handlers.ResearchHandler.HandleUpgradeTechCallback, which the
// player must tap themselves.
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
		MaxTokens:   1024,
		Temperature: 0.3,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("researchplanner: ai completion failed: %w", err)
	}

	rec := ParseRecommendation(resp.Text)

	if p.AI.Memory != nil {
		_ = p.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleAssistant, Content: resp.Text})
	}

	return rec, nil
}
