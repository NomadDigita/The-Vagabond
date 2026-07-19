package governor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

// MemoryScope namespaces this feature's conversational history inside
// ai_memory so it never collides with Fleet Commander, Guild
// Assistant, etc., which will use their own scope strings.
const MemoryScope = "planet_governor"

// ErrNoEncampment is returned when the calling player has no
// registered base yet (e.g. hasn't finished onboarding).
var ErrNoEncampment = errors.New("governor: player has no encampment")

// Governor is the Phase B entry point. One instance is shared across
// all requests (it holds only a DB pool and the shared AI Service,
// both already safe for concurrent use).
type Governor struct {
	DB *sql.DB
	AI *ai.Service
}

func New(db *sql.DB, service *ai.Service) *Governor {
	return &Governor{DB: db, AI: service}
}

// BuildSnapshot loads the current state of a player's base. Every
// COALESCE default mirrors internal/engine/resource.RunResourcePass so
// the Governor's picture of "level 1 / not built yet" agrees with the
// tick engine's own defaults.
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

// Recommend produces a fresh AI recommendation for the given player's
// base. It stores both the prompt and the response in ai_memory
// (scoped to MemoryScope) so a later call to Recommend — or any future
// feature that reads the same scope — has continuity without needing
// to re-derive it from raw DB state.
//
// This is advisory-only: nothing in this method mutates encampments,
// modules, or any other game table. See PROJECT_MASTER_PLAN.md §4 for
// why autopilot execution is deliberately not implemented yet.
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
		MaxTokens:   2048,
		Temperature: 0.3,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("governor: ai completion failed: %w", err)
	}

	rec := ParseRecommendation(resp.Text)

	if g.AI.Memory != nil {
		_ = g.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleAssistant, Content: resp.Text})
	}

	return rec, nil
}

// AutopilotSetting reports whether a player has opted into autopilot
// for their base. Reading/writing this preference is implemented now
// so the UI and data model exist, but — importantly — nothing in this
// package currently *acts* on a true value. No code path calls
// upgrade/build/repair automatically. That execution engine is
// explicitly deferred (see PROJECT_MASTER_PLAN.md §4) so it can be
// built and tested deliberately rather than bolted on here.
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

// SetAutopilot stores the player's preference. See the doc comment on
// AutopilotSetting: this does not currently cause any autonomous
// action to occur.
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
