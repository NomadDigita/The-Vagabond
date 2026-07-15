package fleetcommander

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
	"github.com/NomadDigita/The-Vagabond/internal/game/content"
)

// MemoryScope namespaces this feature's conversational history inside
// ai_memory, distinct from governor.MemoryScope and any future scope.
const MemoryScope = "fleet_commander"

// ErrNoEncampment is returned when the calling player has no
// registered base yet.
var ErrNoEncampment = errors.New("fleetcommander: player has no encampment")

// Commander is the Phase C entry point.
type Commander struct {
	DB *sql.DB
	AI *ai.Service
}

func New(db *sql.DB, service *ai.Service) *Commander {
	return &Commander{DB: db, AI: service}
}

// unitColumns lists every workshop_inventory unit column this package
// knows about. Kept as an explicit list (rather than SELECT *) so a
// schema change is a deliberate one-line addition here, not a silent
// behavior change.
var unitColumns = []string{
	"soldiers", "drones", "jets", "mechs", "nukes", "buggies", "ships",
	"haulers", "tankers", "rigs", "miners", "destroyers", "bombers",
	"scouts", "battlecruisers", "deathstars", "fusion_tanks", "nuclear_shields",
}

// BuildOwnFleet loads the calling player's current unit composition.
func (c *Commander) BuildOwnFleet(ctx context.Context, userID int64) (FleetComposition, string, int, error) {
	var encampmentID string
	var level int
	if err := c.DB.QueryRowContext(ctx, `SELECT id, level FROM encampments WHERE user_id = $1`, userID).Scan(&encampmentID, &level); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", 0, ErrNoEncampment
		}
		return nil, "", 0, fmt.Errorf("fleetcommander: load encampment: %w", err)
	}

	query := fmt.Sprintf(`SELECT %s FROM workshop_inventory WHERE encampment_id = $1`, columnList())
	dest := make([]any, len(unitColumns))
	vals := make([]int, len(unitColumns))
	for i := range vals {
		dest[i] = &vals[i]
	}
	err := c.DB.QueryRowContext(ctx, query, encampmentID).Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		// No workshop_inventory row yet (brand new player) — an empty
		// fleet is a valid, meaningful state, not an error.
		return FleetComposition{}, encampmentID, level, nil
	}
	if err != nil {
		return nil, "", 0, fmt.Errorf("fleetcommander: load fleet: %w", err)
	}

	comp := make(FleetComposition, len(unitColumns))
	for i, name := range unitColumns {
		comp[name] = vals[i]
	}
	return comp, encampmentID, level, nil
}

func columnList() string {
	out := ""
	for i, c := range unitColumns {
		if i > 0 {
			out += ", "
		}
		out += c
	}
	return out
}

// BuildRogueNestTarget produces a PvE TargetProfile scaled to the
// player's level, reusing the exact same composition logic already
// shown to players via /recon (internal/game/content.RogueNestComposition)
// so the Fleet Commander's view of the nest never disagrees with what
// the static recon report already told the player.
func BuildRogueNestTarget(campLevel int) TargetProfile {
	nest := content.RogueNestComposition(campLevel)
	return TargetProfile{
		Name:       "Rogue Drone Nest",
		IsPvE:      true,
		ThreatTier: content.ThreatTier(campLevel),
		Garrison: FleetComposition{
			"soldiers": nest.Soldiers,
			"mechs":    nest.Mechs,
			"drones":   nest.Drones,
			"jets":     nest.Jets,
		},
		TurretBonus: nest.TurretBonus,
	}
}

// BuildCombatHistory summarizes the player's last N completed raids as
// attacker. "Win" here is a heuristic — the raids table has no
// explicit outcome column, so a raid is counted as an apparent win if
// it yielded any stolen resources. This is flagged as a known
// approximation in PROJECT_MASTER_PLAN.md; a future session should
// replace it if/when the SpaceHunt combat branch adds an authoritative
// outcome column.
func (c *Commander) BuildCombatHistory(ctx context.Context, encampmentID string, limit int) (CombatHistorySummary, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := c.DB.QueryContext(ctx, `
		SELECT attacker_losses, (stolen_scrap + stolen_metal + stolen_crystal) > 0 AS apparent_win
		FROM raids
		WHERE attacker_id = $1 AND state = 'completed'
		ORDER BY created_at DESC
		LIMIT $2`, encampmentID, limit)
	if err != nil {
		return CombatHistorySummary{}, fmt.Errorf("fleetcommander: load combat history: %w", err)
	}
	defer rows.Close()

	var summary CombatHistorySummary
	for rows.Next() {
		var losses int
		var win bool
		if err := rows.Scan(&losses, &win); err != nil {
			return CombatHistorySummary{}, fmt.Errorf("fleetcommander: scan raid row: %w", err)
		}
		summary.RaidsAnalyzed++
		summary.TotalLosses += losses
		if win {
			summary.ApparentWins++
		}
	}
	if err := rows.Err(); err != nil {
		return CombatHistorySummary{}, err
	}
	if summary.RaidsAnalyzed > 0 {
		summary.AverageLosses = float64(summary.TotalLosses) / float64(summary.RaidsAnalyzed)
	}
	return summary, nil
}

// Recommend produces a fresh AI recommendation for a player's fleet
// against the (currently PvE-only, see PROJECT_MASTER_PLAN.md) rogue
// nest target. It stores both turns in ai_memory under MemoryScope.
//
// This is advisory-only: nothing in this method launches a raid or
// moves any unit.
func (c *Commander) Recommend(ctx context.Context, userID int64) (*Recommendation, error) {
	ownFleet, encampmentID, level, err := c.BuildOwnFleet(ctx, userID)
	if err != nil {
		return nil, err
	}

	target := BuildRogueNestTarget(level)

	history, err := c.BuildCombatHistory(ctx, encampmentID, 20)
	if err != nil {
		return nil, err
	}

	userPrompt := BuildUserPrompt(ownFleet, target, history)

	if c.AI.Memory != nil {
		_ = c.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleUser, Content: userPrompt})
	}

	resp, err := c.AI.Complete(ctx, ai.CompletionRequest{
		Feature:     string(ai.FeatureFleetCommander),
		UserID:      userID,
		System:      SystemPrompt,
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: userPrompt}},
		MaxTokens:   1024,
		Temperature: 0.3,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("fleetcommander: ai completion failed: %w", err)
	}

	rec := ParseRecommendation(resp.Text)

	if c.AI.Memory != nil {
		_ = c.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleAssistant, Content: resp.Text})
	}

	return rec, nil
}
