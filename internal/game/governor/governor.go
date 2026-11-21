package governor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

const MemoryScope = "planet_governor"

var ErrNoEncampment = errors.New("governor: player has no encampment")

type Governor struct {
	DB *sql.DB
	AI *ai.Service
}

func New(db *sql.DB, service *ai.Service) *Governor {
	return &Governor{DB: db, AI: service}
}

func (g *Governor) BuildSnapshot(ctx context.Context, userID int64) (*Snapshot, error) {
	var s Snapshot
	err := g.DB.QueryRowContext(ctx, `
		SELECT e.id, e.name, e.level,
			r.scrap, r.rations, r.electricity, r.metal, r.crystal, r.hydrogen, r.dollars,
			COALESCE(w.soldiers, 0), COALESCE(w.buggies, 0), COALESCE(w.ships, 0),
			COALESCE(rs.defense_tech_lvl, 1), COALESCE(rs.production_tech_lvl, 1)
		FROM encampments e
		JOIN resources r ON r.encampment_id = e.id
		LEFT JOIN workshop_inventory w ON w.encampment_id = e.id
		LEFT JOIN research_states rs ON rs.encampment_id = e.id
		WHERE e.user_id = $1`, userID,
	).Scan(
		&s.EncampmentID, &s.Name, &s.Level,
		&s.Scrap, &s.Rations, &s.Electricity, &s.Metal, &s.Crystal, &s.Hydrogen, &s.Dollars,
		&s.Soldiers, &s.Buggies, &s.Ships,
		&s.DefenseTechLvl, &s.ProductionTechLvl,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNoEncampment
	}
	if err != nil {
		return nil, fmt.Errorf("governor: load base state: %w", err)
	}

	rows, err := g.DB.QueryContext(ctx, `SELECT type, level FROM modules WHERE encampment_id = $1`, s.EncampmentID)
	if err != nil {
		return nil, fmt.Errorf("governor: load modules: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var m ModuleState
		if err := rows.Scan(&m.Type, &m.Level); err != nil {
			return nil, fmt.Errorf("governor: scan module row: %w", err)
		}
		s.Modules = append(s.Modules, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return &s, nil
}

func (g *Governor) Recommend(ctx context.Context, userID int64) (*Recommendation, error) {
	snapshot, err := g.BuildSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	userPrompt := BuildUserPrompt(*snapshot)

	if g.AI.Memory != nil {
		_ = g.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleUser, Content: userPrompt})
	}

	resp, err := g.AI.Complete(ctx, ai.CompletionRequest{
		Feature:     string(ai.FeaturePlanetGovernor),
		UserID:      userID,
		System:      SystemPrompt,
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: userPrompt}},
		MaxTokens:   4096,
		Temperature: 0.3,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("governor: ai completion failed: %w", err)
	}

	rec := ParseRecommendation(resp.Text, resp.StopReason)

	if g.AI.Memory != nil {
		_ = g.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleAssistant, Content: resp.Text})
	}

	return rec, nil
}

func (g *Governor) AutopilotSetting(ctx context.Context, userID int64) (bool, error) {
	var enabled sql.NullBool
	err := g.DB.QueryRowContext(ctx, `
		SELECT gs.autopilot_enabled
		FROM governor_settings gs
		JOIN encampments e ON e.id = gs.encampment_id
		WHERE e.user_id = $1`, userID,
	).Scan(&enabled)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return enabled.Valid && enabled.Bool, nil
}

func (g *Governor) SetAutopilot(ctx context.Context, userID int64, enabled bool) error {
	var encampmentID string
	if err := g.DB.QueryRowContext(ctx, `SELECT id FROM encampments WHERE user_id = $1`, userID).Scan(&encampmentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoEncampment
		}
		return err
	}
	_, err := g.DB.ExecContext(ctx, `
		INSERT INTO governor_settings (encampment_id, autopilot_enabled, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (encampment_id) DO UPDATE SET autopilot_enabled = $2, updated_at = now()`,
		encampmentID, enabled)
	return err
}
