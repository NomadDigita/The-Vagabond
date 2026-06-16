package agent

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

type Processor struct {
	DB *sql.DB
}

func NewProcessor(db *sql.DB) *Processor {
	return &Processor{DB: db}
}

type ActiveAgent struct {
	UserID       int64
	Mode         string
	CampID       string
	CampName     string
	ScrapBalance float64
	Energy       float64
}

// RunAgentPass executes automation logic for all active agents inside the transaction
func (p *Processor) RunAgentPass(ctx context.Context, tx *sql.Tx) error {
	// Query all active agents alongside their current resources
	query := `
		SELECT t.user_id, t.mode, e.id, e.name, r.scrap, r.energy
		FROM agent_tasks t
		JOIN encampments e ON e.user_id = t.user_id
		JOIN resources r ON r.encampment_id = e.id
		WHERE t.is_active = TRUE`

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed querying active automation tasks: %w", err)
	}
	defer rows.Close()

	var agents []ActiveAgent
	for rows.Next() {
		var a ActiveAgent
		if err := rows.Scan(&a.UserID, &a.Mode, &a.CampID, &a.CampName, &a.ScrapBalance, &a.Energy); err == nil {
			agents = append(agents, a)
		}
	}
	rows.Close()

	for _, a := range agents {
		// 1. Consume 2.0 energy cells as operation fuel
		if a.Energy < 2.0 {
			// Energy depleted: Force shutdown agent task
			_, _ = tx.ExecContext(ctx, "UPDATE agent_tasks SET is_active = FALSE WHERE user_id = $1", a.UserID)

			// Queue alert notification for player
			alertMsg := fmt.Sprintf(
				"🔌 AGENT DEACTIVATED\n\n"+
					"Outpost: %s\n"+
					"Your automation agent has shut down due to complete depletion of Energy Cells.",
				a.CampName,
			)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", a.UserID, alertMsg)
			log.Printf("Agent auto-shut down for user %d: lack of energy.", a.UserID)
			continue
		}

		// Deduct energy
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET energy = energy - 2.0 WHERE encampment_id = $1", a.CampID)

		// 2. Process action modes
		switch a.Mode {
		case "collector":
			// Generate +2.0 Scrap and +1.0 Rations
			updateCollector := `
				UPDATE resources 
				SET scrap = scrap + 2.00, rations = rations + 1.00 
				WHERE encampment_id = $1`
			_, err = tx.ExecContext(ctx, updateCollector, a.CampID)
			if err != nil {
				log.Printf("Agent failed executing collector pass: %v", err)
			}
			log.Printf("Agent [Collector] executed action for outpost: %s (+2.0 Scrap, +1.0 Rations)", a.CampName)

		case "builder":
			// Check if any module is currently upgrading in this camp
			var isUpgrading bool
			_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM modules WHERE encampment_id = $1 AND is_upgrading = TRUE)", a.CampID).Scan(&isUpgrading)
			if isUpgrading {
				// Queue is busy, builder agent waits
				continue
			}

			// Find modules eligible for upgrade (Tent, Scrap Heap, or Generator)
			// Choose the lowest level module that we can afford (Cost: Level * 150 Scrap)
			queryEligible := `
				SELECT type, level 
				FROM modules 
				WHERE encampment_id = $1 
				ORDER BY level ASC 
				LIMIT 1`

			var modType string
			var lvl int
			err = tx.QueryRowContext(ctx, queryEligible, a.CampID).Scan(&modType, &lvl)
			if err != nil {
				// Initialize modules if missing
				_, _ = tx.ExecContext(ctx, "INSERT INTO modules (encampment_id, type, level) VALUES ($1, 'tent', 1) ON CONFLICT DO NOTHING", a.CampID)
				continue
			}

			cost := lvl * 150
			if a.ScrapBalance >= float64(cost) {
				// Deduct Scrap and trigger the construction upgrade
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", cost, a.CampID)

				readyAt := time.Now().Add(20 * time.Second)
				upsertModule := `
					UPDATE modules 
					SET is_upgrading = TRUE, upgrade_ready_at = $1 
					WHERE encampment_id = $2 AND type = $3`
				_, err = tx.ExecContext(ctx, upsertModule, readyAt, a.CampID, modType)
				if err != nil {
					log.Printf("Agent failed writing auto-upgrade: %v", err)
					continue
				}

				// Notify player of agent construction start
				alertMsg := fmt.Sprintf(
					"🤖 AGENT AUTOMATED CONSTRUCTION\n\n"+
						"Outpost: %s\n"+
						"Your Agent has initiated an upgrade on your [%s] to Level %d.\n"+
						"⚙️ Construction Cost: %d Scrap deducted.",
					a.CampName, modType, lvl+1, cost,
				)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", a.UserID, alertMsg)
				log.Printf("Agent [Builder] auto-triggered upgrade for module %s level %d on camp %s", modType, lvl+1, a.CampName)
			}
		}
	}

	return nil
}
