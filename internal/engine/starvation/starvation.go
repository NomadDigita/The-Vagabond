package starvation

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
)

// Engine handles penalties, unit desertion, and alert notifications.
type Engine struct {
	DB *sql.DB
}

// NewEngine builds a configured starvation systems module.
func NewEngine(db *sql.DB) *Engine {
	return &Engine{DB: db}
}

// RunStarvationPass runs starvation checks inside the active transaction.
func (e *Engine) RunStarvationPass(ctx context.Context, tx *sql.Tx) error {
	// Find all outposts that have run out of rations
	query := `
		SELECT e.id, e.user_id, e.name
		FROM encampments e
		JOIN resources r ON r.encampment_id = e.id
		WHERE r.rations = 0`

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed scanning starvation records: %w", err)
	}
	defer rows.Close()

	type starvingCamp struct {
		id     string
		userID int64
		name   string
	}

	var camps []starvingCamp
	for rows.Next() {
		var c starvingCamp
		if err := rows.Scan(&c.id, &c.userID, &c.name); err != nil {
			log.Printf("Error scanning starving camp row: %v", err)
			continue
		}
		camps = append(camps, c)
	}

	for _, c := range camps {
		// 1. Decay morale of all units in this encampment by 5 points
		decayMoraleQuery := `
			UPDATE units 
			SET morale = GREATEST(morale - 5, 0) 
			WHERE encampment_id = $1`
		_, err := tx.ExecContext(ctx, decayMoraleQuery, c.id)
		if err != nil {
			return fmt.Errorf("failed applying starvation morale decay: %w", err)
		}

		// 2. Fetch units with low morale (< 30) to calculate desertion chance
		queryLowMorale := `
			SELECT id, type, quantity, morale 
			FROM units 
			WHERE encampment_id = $1 AND morale < 30 AND quantity > 0`

		unitRows, err := tx.QueryContext(ctx, queryLowMorale, c.id)
		if err != nil {
			log.Printf("Failed scanning unit rows for desertion checks: %v", err)
			continue
		}

		type unitDesertion struct {
			id       string
			unitType string
			quantity int
		}

		var candidates []unitDesertion
		for unitRows.Next() {
			var d unitDesertion
			var m int
			if err := unitRows.Scan(&d.id, &d.unitType, &d.quantity, &m); err == nil {
				candidates = append(candidates, d)
			}
		}
		unitRows.Close()

		// 3. Process 20% random chance of unit desertion
		for _, u := range candidates {
			if rand.Float64() < 0.20 {
				// Reduce unit quantity count by 1
				desertionQuery := `
					UPDATE units 
					SET quantity = quantity - 1 
					WHERE id = $1`
				_, err := tx.ExecContext(ctx, desertionQuery, u.id)
				if err != nil {
					log.Printf("Failed writing unit desertion update: %v", err)
					continue
				}

				// Clean up empty units
				deleteEmptyUnits := `DELETE FROM units WHERE id = $1 AND quantity <= 0`
				_, _ = tx.ExecContext(ctx, deleteEmptyUnits, u.id)

				// Queue alert notification for Telegram
				alertMsg := fmt.Sprintf(
					"⚠️ STARVATION DESERTION\n\n"+
						"Outpost: %s\n"+
						"Your rations are fully depleted. "+
						"Due to starvation and critically low morale, "+
						"one of your [%s] units has deserted the encampment.",
					c.name, u.unitType,
				)

				insertNotification := `
					INSERT INTO notifications (user_id, message, is_sent) 
					VALUES ($1, $2, FALSE)`
				_, err = tx.ExecContext(ctx, insertNotification, c.userID, alertMsg)
				if err != nil {
					log.Printf("Failed queuing starvation alert notification: %v", err)
				}
				log.Printf("Starvation desertion triggered for camp %s. One %s deserted.", c.name, u.unitType)
			}
		}
	}

	return nil
}
