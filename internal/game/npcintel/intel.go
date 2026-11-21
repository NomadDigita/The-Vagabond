package npcintel

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
	"github.com/NomadDigita/The-Vagabond/internal/game/content"
)

const MemoryScope = "npc_intel"

var ErrNoEncampment = errors.New("npcintel: player has no encampment")

type Intel struct {
	DB *sql.DB
	AI *ai.Service
}

func New(db *sql.DB, service *ai.Service) *Intel {
	return &Intel{DB: db, AI: service}
}

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
		MaxTokens:   4096,
		Temperature: 0.3,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("npcintel: ai completion failed: %w", err)
	}

	rec := ParseRecommendation(resp.Text, resp.StopReason)

	if in.AI.Memory != nil {
		_ = in.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleAssistant, Content: resp.Text})
	}

	return rec, nil
}
