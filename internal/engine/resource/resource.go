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
}

func (p *Processor) RunResourcePass(ctx context.Context, tx *sql.Tx) error {
	// Gather encampments, module levels, troop count, and outstanding loans
	query := `
		SELECT 
			e.id, r.scrap, r.rations, r.energy,
			COALESCE((SELECT m.level FROM modules m WHERE m.encampment_id = e.id AND m.type = 'tent'), 1) as tent_lvl,
			COALESCE((SELECT m.level FROM modules m WHERE m.encampment_id = e.id AND m.type = 'scrap_heap'), 1) as heap_lvl,
			COALESCE((SELECT m.level FROM modules m WHERE m.encampment_id = e.id AND m.type = 'generator'), 1) as gen_lvl,
			COALESCE((SELECT SUM(u.quantity) FROM units u WHERE u.encampment_id = e.id), 0) as troop_count,
			COALESCE((SELECT b.loan_amount FROM bank_accounts b WHERE b.encampment_id = e.id), 0) as loan_amount
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
		if err := rows.Scan(&s.ID, &s.Scrap, &s.Rations, &s.Energy, &s.TentLvl, &s.ScrapHeapLvl, &s.GeneratorLvl, &s.TroopCount, &s.LoanAmount); err != nil {
			log.Printf("Error scanning encampment state row: %v", err)
			continue
		}
		states = append(states, s)
	}

	for _, s := range states {
		scrapGenerated := 0.25 * float64(s.ScrapHeapLvl)
		rationsGenerated := 0.10
		energyGenerated := 0.05 * float64(s.GeneratorLvl)

		// Apply Bank Committee loan repayment tax (15% deduction on raw scrap yield)
		var taxDeducted float64
		if s.LoanAmount > 0 {
			taxDeducted = scrapGenerated * 0.15
			if taxDeducted > s.LoanAmount {
				taxDeducted = s.LoanAmount
			}
			scrapGenerated -= taxDeducted

			// Pay down loan
			_, _ = tx.ExecContext(ctx, "UPDATE bank_accounts SET loan_amount = GREATEST(loan_amount - $1, 0) WHERE encampment_id = $2", taxDeducted, s.ID)
		}

		rationsConsumed := float64(s.TroopCount) * 0.05

		newScrap := s.Scrap + scrapGenerated
		newRations := s.Rations + rationsGenerated - rationsConsumed
		newEnergy := s.Energy + energyGenerated

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

		_, err := tx.ExecContext(ctx, updateQuery, newScrap, newRations, newEnergy, s.ID)
		if err != nil {
			return fmt.Errorf("failed executing resource state write back: %w", err)
		}
	}

	return nil
}
