package resource

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

type Processor struct {
	DB *sql.DB
}

func NewProcessor(db *sql.DB) *Processor {
	return &Processor{DB: db}
}

type EncampmentState struct {
	ID           string
	Scrap        float64
	Rations      float64
	Energy       float64
	TentLvl      int
	ScrapHeapLvl int
	GeneratorLvl int
	TroopCount   int
	LoanAmount   float64
	BuggyCount   int
	ShipCount    int
	JetCount     int
}

func (p *Processor) RunResourcePass(ctx context.Context, tx *sql.Tx) error {
	// Query active global weather front
	var activeWeather string
	_ = tx.QueryRowContext(ctx, "SELECT active_weather FROM world_state WHERE id = 1").Scan(&activeWeather)

	// --- EXPANDED LOGISTICS FLEET STATE QUERY (Phase 3) ---
	query := `
		SELECT 
			e.id, r.scrap, r.rations, r.energy,
			COALESCE((SELECT m.level FROM modules m WHERE m.encampment_id = e.id AND m.type = 'tent'), 1) as tent_lvl,
			COALESCE((SELECT m.level FROM modules m WHERE m.encampment_id = e.id AND m.type = 'scrap_heap'), 1) as heap_lvl,
			COALESCE((SELECT m.level FROM modules m WHERE m.encampment_id = e.id AND m.type = 'generator'), 1) as gen_lvl,
			COALESCE((SELECT w.soldiers FROM workshop_inventory w WHERE w.encampment_id = e.id), 0) as troop_count,
			COALESCE((SELECT b.loan_amount FROM bank_accounts b WHERE b.encampment_id = e.id), 0) as loan_amount,
			COALESCE((SELECT buggies FROM workshop_inventory w WHERE w.encampment_id = e.id), 0) as buggy_count,
			COALESCE((SELECT ships FROM workshop_inventory w WHERE w.encampment_id = e.id), 0) as ship_count,
			COALESCE((SELECT jets FROM workshop_inventory w WHERE w.encampment_id = e.id), 0) as jet_count
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
		err := rows.Scan(
			&s.ID, &s.Scrap, &s.Rations, &s.Energy, 
			&s.TentLvl, &s.ScrapHeapLvl, &s.GeneratorLvl, 
			&s.TroopCount, &s.LoanAmount, 
			&s.BuggyCount, &s.ShipCount, &s.JetCount,
		)
		if err != nil {
			log.Printf("Error scanning encampment state row: %v", err)
			continue
		}
		states = append(states, s)
	}

	for _, s := range states {
		scrapGenerated := 0.25 * float64(s.ScrapHeapLvl)
		rationsGenerated := 0.10
		energyGenerated := 0.05 * float64(s.GeneratorLvl)

		// Apply Dynamic Weather Multipliers
		switch activeWeather {
		case "solar_flare":
			energyGenerated *= 2.0 // Solar panels get 2x power
		case "radiation_storm":
			energyGenerated *= 0.5 // Cloud cover drops solar efficiency by 50%
		}

		var taxDeducted float64
		if s.LoanAmount > 0 {
			taxDeducted = scrapGenerated * 0.15
			if taxDeducted > s.LoanAmount {
				taxDeducted = s.LoanAmount
			}
			scrapGenerated -= taxDeducted

			_, _ = tx.ExecContext(ctx, "UPDATE bank_accounts SET loan_amount = GREATEST(loan_amount - $1, 0) WHERE encampment_id = $2", taxDeducted, s.ID)
		}

		// --- ADVANCED MILITARY & VEHICLE MAINTENANCE UPKEEP (Phase 3) ---
		rationsConsumed := float64(s.TroopCount) * 0.05
		energyConsumed := (float64(s.BuggyCount) * 0.02) + (float64(s.ShipCount) * 0.05) + (float64(s.JetCount) * 0.10)

		newScrap := s.Scrap + scrapGenerated
		newRations := s.Rations + rationsGenerated - rationsConsumed
		newEnergy := s.Energy + energyGenerated - energyConsumed

		if newRations < 0 {
			newRations = 0
		}
		if newEnergy < 0 {
			newEnergy = 0
		}

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

		updateQuery := `
			UPDATE resources 
			SET scrap = $1, rations = $2, energy = $3, last_ticked_at = CURRENT_TIMESTAMP 
			WHERE encampment_id = $4`
		
		_, err = tx.ExecContext(ctx, updateQuery, newScrap, newRations, newEnergy, s.ID)
		if err != nil {
			return fmt.Errorf("failed executing resource state write back: %w", err)
		}
	}

	return nil
}