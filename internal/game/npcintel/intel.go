package npcintel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
	"github.com/NomadDigita/The-Vagabond/internal/game/content"
)

// MemoryScope namespaces this feature's conversational history inside
// ai_memory, distinct from every other Phase B+ scope.
const MemoryScope = "npc_intel"

// ErrNoEncampment is returned when the calling player has no
// registered base yet (and therefore no level to scale the Nest to,
// and no fleet to read).
var ErrNoEncampment = errors.New("npcintel: player has no encampment")

// Intel is the Phase I entry point.
type Intel struct {
	DB *sql.DB
	AI *ai.Service
}

func New(db *sql.DB, service *ai.Service) *Intel {
	return &Intel{DB: db, AI: service}
}

// BuildSnapshot loads the calling player's level (to scale the Nest
// via the same content.RogueNestComposition Fleet Commander and
// /recon_ai already use, so this never disagrees with either) and
// their current mobile fleet from workshop_inventory.
func (in *Intel) BuildSnapshot(ctx context.Context, userID int64) (*Snapshot, error) {
	var level int
	if err := in.DB.QueryRowContext(ctx, `SELECT level FROM encampments WHERE user_id = $1`, userID).Scan(&level); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoEncampment
		}
		return nil, fmt.Errorf("npcintel: load player level: %w", err)
	}

	nest := content.RogueNestComposition(level)
	snapshot := Snapshot{
		PlayerLevel: level,
		Nest: NestProfile{
			ThreatTier:      content.ThreatTier(level),
			Soldiers:        nest.Soldiers,
			Mechs:           nest.Mechs,
			Drones:          nest.Drones,
			Jets:            nest.Jets,
			LightLaserLvl:   nest.LightLaserLvl,
			HeavyLaserLvl:   nest.HeavyLaserLvl,
			GaussCannonLvl:  nest.GaussCannonLvl,
			IonCannonLvl:    nest.IonCannonLvl,
			PlasmaTurretLvl: nest.PlasmaTurretLvl,
			Guardians:       nest.Guardians,
			Observers:       nest.Observers,
			Shields:         nest.Shields,
			HeroSuperpower:  nest.HeroSuperpower,
		},
	}

	fleet, err := in.buildFleet(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("npcintel: load fleet: %w", err)
	}
	snapshot.Fleet = fleet

	return &snapshot, nil
}

// buildFleet reads only the mobile combat units from
// workshop_inventory — Guardians/Observers are garrison-only (never
// leave the base on a raid, per their own doc comments in
// internal/engine/tick/engine.go) so they're deliberately excluded
// from the player's own FleetProfile even though they appear on the
// Nest side.
func (in *Intel) buildFleet(ctx context.Context, userID int64) (FleetProfile, error) {
	var f FleetProfile
	err := in.DB.QueryRowContext(ctx, `
		SELECT
			COALESCE(w.soldiers, 0), COALESCE(w.mechs, 0), COALESCE(w.drones, 0), COALESCE(w.jets, 0),
			COALESCE(w.destroyers, 0), COALESCE(w.bombers, 0), COALESCE(w.wraiths, 0),
			COALESCE(w.liberators, 0), COALESCE(w.battlecruisers, 0)
		FROM encampments e
		LEFT JOIN workshop_inventory w ON w.encampment_id = e.id
		WHERE e.user_id = $1`, userID).Scan(
		&f.Soldiers, &f.Mechs, &f.Drones, &f.Jets,
		&f.Destroyers, &f.Bombers, &f.Wraiths, &f.Liberators, &f.Battlecruisers)
	if err != nil {
		return FleetProfile{}, err
	}
	return f, nil
}

// Recommend produces a fresh AI tactical read for the given player
// against the Rogue Drone Nest scaled to their level. It stores both
// turns in ai_memory under MemoryScope.
//
// Read-only: nothing in this method launches a raid or moves any unit
// — it only reads and advises.
func (in *Intel) Recommend(ctx context.Context, userID int64) (*Recommendation, error) {
	snapshot, err := in.BuildSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	userPrompt := BuildUserPrompt(*snapshot)

	if in.AI.Memory != nil {
		_ = in.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleUser, Content: userPrompt})
	}

	resp, err := in.AI.Complete(ctx, ai.CompletionRequest{
		Feature:     string(ai.FeatureNPCIntel),
		UserID:      userID,
		System:      SystemPrompt,
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: userPrompt}},
		MaxTokens:   2048,
		Temperature: 0.3,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("npcintel: ai completion failed: %w", err)
	}

	rec := ParseRecommendation(resp.Text)

	if in.AI.Memory != nil {
		_ = in.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleAssistant, Content: resp.Text})
	}

	return rec, nil
}
