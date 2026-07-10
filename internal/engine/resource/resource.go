package resource

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
)

type Processor struct {
	DB *sql.DB
}

func NewProcessor(db *sql.DB) *Processor {
	return &Processor{DB: db}
}

type EncampmentState struct {
	ID             string
	Scrap          float64
	Rations        float64
	Energy         float64
	TentLvl        int
	ScrapHeapLvl   int
	GeneratorLvl   int
	TroopCount     int
	LoanAmount     float64
	BuggyCount     int
	ShipCount      int
	JetCount       int
	DefenseTechLvl    int
	ProductionTechLvl int
	SalvageLvl        int
}

func (p *Processor) RunResourcePass(ctx context.Context, tx *sql.Tx) error {
	var activeWeather string
	_ = tx.QueryRowContext(ctx, "SELECT active_weather FROM world_state WHERE id = 1").Scan(&activeWeather)

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
			COALESCE((SELECT jets FROM workshop_inventory w WHERE w.encampment_id = e.id), 0) as jet_count,
			COALESCE((SELECT res.defense_tech_lvl FROM research_states res WHERE res.encampment_id = e.id), 1) as defense_tech_lvl,
			COALESCE((SELECT res.production_tech_lvl FROM research_states res WHERE res.encampment_id = e.id), 1) as production_tech_lvl,
			COALESCE((SELECT mut.salvage_lvl FROM mutation_states mut WHERE mut.encampment_id = e.id), 1) as salvage_lvl
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
			&s.DefenseTechLvl, &s.ProductionTechLvl, &s.SalvageLvl,
		)
		if err != nil {
			log.Printf("Error scanning encampment state row: %v", err)
			continue
		}
		states = append(states, s)
	}

	for _, s := range states {
		overclockBonus := float64(s.ProductionTechLvl-1) * 0.20
		salvageBonus := float64(s.SalvageLvl-1) * 0.15
		
		scrapGenerated := (0.25 * float64(s.ScrapHeapLvl)) * (1.0 + overclockBonus + salvageBonus)
		rationsGenerated := 0.10
		energyGenerated := 0.05 * float64(s.GeneratorLvl)

		switch activeWeather {
		case "solar_flare":
			energyGenerated *= 2.0
		case "radiation_storm":
			energyGenerated *= 0.5
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

		rationsConsumed := float64(s.TroopCount) * 0.05
		energyConsumed := (float64(s.BuggyCount) * 0.02) + (float64(s.ShipCount) * 0.05) + (float64(s.JetCount) * 0.10)

		storageCap := float64(s.TentLvl) * 500.0

		// Surplus Preservation System: Only caps new passive allocations. Pre-existing balances are preserved.
		newScrap := s.Scrap
		if s.Scrap < storageCap {
			newScrap = math.Min(s.Scrap+scrapGenerated, storageCap)
		}

		newRations := s.Rations
		rationsDiff := rationsGenerated - rationsConsumed
		if rationsDiff > 0 {
			if s.Rations < storageCap {
				newRations = math.Min(s.Rations+rationsDiff, storageCap)
			}
		} else {
			newRations = math.Max(s.Rations+rationsDiff, 0.0)
		}

		newEnergy := s.Energy
		energyDiff := energyGenerated - energyConsumed
		if energyDiff > 0 {
			if s.Energy < storageCap {
				newEnergy = math.Min(s.Energy+energyDiff, storageCap)
			}
		} else {
			newEnergy = math.Max(s.Energy+energyDiff, 0.0)
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