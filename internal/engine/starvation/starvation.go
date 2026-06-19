package starvation

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
)

type Engine struct {
	DB *sql.DB
}

func NewEngine(db *sql.DB) *Engine {
	return &Engine{DB: db}
}

// RunStarvationPass runs starvation checks and Ghost Mode ruin conversions
func (e *Engine) RunStarvationPass(ctx context.Context, tx *sql.Tx) error {
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
		if err := rows.Scan(&c.id, &c.userID, &c.name); err == nil {
			camps = append(camps, c)
		}
	}
	rows.Close()

	for _, c := range camps {
		// Verify personnel counts directly from authoritative inventory slots
		var troopCount int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers, 0) FROM workshop_inventory WHERE encampment_id = $1", c.id).Scan(&troopCount)

		if troopCount <= 0 && c.name != "Ruined Outpost" {
			// Outpost has fully collapsed into ruins!
			_, _ = tx.ExecContext(ctx, "UPDATE encampments SET name = 'Ruined Outpost' WHERE id = $1", c.id)

			// Record sector news headline
			headline := fmt.Sprintf("☠️ GHOST MODE: Encampment [%s] has collapsed due to starvation. Location reduced to scavengable ruins.", c.name)
			_, _ = tx.ExecContext(ctx, "INSERT INTO world_news (headline) VALUES ($1)", headline)

			log.Printf("Ghost Mode: Encampment %s collapsed into ruins.", c.name)
			continue
		}

		// Proportional starvation desertion decay: 20% chance to lose 1 soldier on starvation tick
		if rand.Float64() < 0.20 {
			_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = GREATEST(soldiers - 1, 0) WHERE encampment_id = $1", c.id)

			alertMsg := fmt.Sprintf(
				"⚠️ STARVATION DESERTION\n\n"+
					"Outpost: %s\n"+
					"Your rations are fully depleted. "+
					"Due to starvation, one of your soldiers has deserted the encampment.",
				c.name,
			)

			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", c.userID, alertMsg)
		}
	}

	return nil
}