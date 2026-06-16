package resource

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

// Processor handles calculating production, consumption, and storage limits.
type Processor struct {
	DB *sql.DB
}

// NewProcessor builds a production resource engine.
func NewProcessor(db *sql.DB) *Processor {
	return &Processor{DB: db}
}

// EncampmentState models database parameters during tick calculations.
type EncampmentState struct {
	ID           string
	Scrap        float64
	Rations      float64
	Energy       float64
	TentLvl      int
	ScrapHeapLvl int
	GeneratorLvl int
	TroopCount   int
}

// RunResourcePass calculates the changes in resource state inside the active transaction.
func (p *Processor) RunResourcePass(ctx context.Context, tx *sql.Tx) error {
	// 1. Gather encampments alongside module levels and total troops
	query := `
		SELECT 
			e.id, r.scrap, r.rations, r.energy,
			COALESCE((SELECT m.level FROM modules m WHERE m.encampment_id = e.id AND m.type = 'tent'), 1) as tent_lvl,
			COALESCE((SELECT m.level FROM modules m WHERE m.encampment_id = e.id AND m.type = 'scrap_heap'), 1) as heap_lvl,
			COALESCE((SELECT m.level FROM modules m WHERE m.encampment_id = e.id AND m.type = 'generator'), 1) as gen_lvl,
			COALESCE((SELECT SUM(u.quantity) FROM units u WHERE u.encampment_id = e.id), 0) as troop_count
		FROM encampments e
		JOIN resources r ON r.encampment_id = e.id`

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed querying encampment states: %w", err)
	}
	defer rows.Close()

	var states []EncampmentState
	for rows.Next() {
		var s EncampmentState
		if err := rows.Scan(&s.ID, &s.Scrap, &s.Rations, &s.Energy, &s.TentLvl, &s.ScrapHeapLvl, &s.GeneratorLvl, &s.TroopCount); err != nil {
			log.Printf("Error scanning encampment resource state row: %v", err)
			continue
		}
		states = append(states, s)
	}

	// 2. Apply formulas and calculate resource state updates
	for _, s := range states {
		// Production logic: scales with module levels
		scrapGenerated := 0.25 * float64(s.ScrapHeapLvl)
		rationsGenerated := 0.10
		energyGenerated := 0.05 * float64(s.GeneratorLvl)

		// Consumption logic: each unit consumes 0.05 rations per tick
		rationsConsumed := float64(s.TroopCount) * 0.05

		// Set final values
		newScrap := s.Scrap + scrapGenerated
		newRations := s.Rations + rationsGenerated - rationsConsumed
		newEnergy := s.Energy + energyGenerated

		// Enforce safety floor
		if newRations < 0 {
			newRations = 0
		}
		if newEnergy < 0 {
			newEnergy = 0
		}

		// Enforce absolute storage caps based on Tent upgrades (base cap 500)
		storageCap := float64(s.TentLvl) * 500.0
		if newScrap > storageCap {
			newScrap = storageCap
		}
		if newRations > storageCap {
			newRations = storageCap
		}
		if newEnergy > storageCap {
			newEnergy = storageCap
		}

		// Commit updates to the database
		updateQuery := `
			UPDATE resources 
			SET scrap = $1, rations = $2, energy = $3, last_ticked_at = CURRENT_TIMESTAMP 
			WHERE encampment_id = $4`

		_, err := tx.ExecContext(ctx, updateQuery, newScrap, newRations, newEnergy, s.ID)
		if err != nil {
			return fmt.Errorf("failed executing resource state write back: %w", err)
		}
	}

	return nil
}
