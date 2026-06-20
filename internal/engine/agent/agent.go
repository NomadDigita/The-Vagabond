package agent

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"time"
)

type Processor struct {
	DB *sql.DB
}

func NewProcessor(db *sql.DB) *Processor {
	return &Processor{DB: db}
}

type ActiveAgent struct {
	UserID   int64
	Mode     string
	CampID   string
	CampName string
	Scrap    float64
	Rations  float64
	Energy   float64
	Iron     float64
	Gold     float64
	Silver   float64
	Diamond  float64
	Dollars  float64
	TentLvl  int
}

// RunAgentPass executes automation logic for all active agents inside the transaction
func (p *Processor) RunAgentPass(ctx context.Context, tx *sql.Tx) error {
	query := `
		SELECT t.user_id, t.mode, e.id, e.name, 
		       r.scrap, r.rations, r.energy, r.iron, r.gold, r.silver, r.diamond, r.dollars,
		       COALESCE((SELECT m.level FROM modules m WHERE m.encampment_id = e.id AND m.type = 'tent'), 1) as tent_lvl
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
		err := rows.Scan(
			&a.UserID, &a.Mode, &a.CampID, &a.CampName, 
			&a.Scrap, &a.Rations, &a.Energy, &a.Iron, &a.Gold, &a.Silver, &a.Diamond, &a.Dollars,
			&a.TentLvl,
		)
		if err == nil {
			agents = append(agents, a)
		} else {
			log.Printf("Error scanning agent task row: %v", err)
		}
	}
	rows.Close()

	for _, a := range agents {
		// Consume 0.2 energy cells as operation fuel (Balanced Energy Upkeep)
		if a.Energy < 0.2 {
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

		newEnergy := math.Max(a.Energy-0.2, 0.0)
		storageCap := float64(a.TentLvl) * 500.0

		switch a.Mode {
		case "collector":
			// Generate +2.0 Scrap and +1.0 Rations up to the Tent storage capacity cap
			newScrap := math.Min(a.Scrap+2.00, storageCap)
			newRations := math.Min(a.Rations+1.00, storageCap)

			_, err = tx.ExecContext(ctx, `
				UPDATE resources 
				SET scrap = $1, rations = $2, energy = $3 
				WHERE encampment_id = $4`,
				newScrap, newRations, newEnergy, a.CampID,
			)
			if err != nil {
				log.Printf("Agent failed executing collector pass: %v", err)
			}
			log.Printf("Agent [Collector] executed action for outpost: %s (+2.0 Scrap, +1.0 Rations capped at %.0f)", a.CampName, storageCap)

		case "collector_omega":
			// Generate +20.0 Scrap and heavy war resources respecting warehouse storage cap limits
			newScrap := math.Min(a.Scrap+20.00, storageCap)
			newIron := a.Iron + 5.00
			newGold := a.Gold + 1.00
			newSilver := a.Silver + 1.00
			newDiamond := a.Diamond + 0.2
			newDollars := a.Dollars + 2.00

			_, err = tx.ExecContext(ctx, `
				UPDATE resources 
				SET scrap = $1, energy = $2, iron = $3, gold = $4, silver = $5, diamond = $6, dollars = $7 
				WHERE encampment_id = $8`,
				newScrap, newEnergy, newIron, newGold, newSilver, newDiamond, newDollars, a.CampID,
			)
			if err != nil {
				log.Printf("Agent failed executing collector_omega pass: %v", err)
			}
			log.Printf("Agent [Collector Ω] executed resource extraction pass for outpost: %s", a.CampName)

		case "builder":
			// Check if any module is currently upgrading in this camp
			var isUpgrading bool
			_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM modules WHERE encampment_id = $1 AND is_upgrading = TRUE)", a.CampID).Scan(&isUpgrading)
			if isUpgrading {
				// Queue is busy, builder agent waits but still consumes fuel
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET energy = $1 WHERE encampment_id = $2", newEnergy, a.CampID)
				continue
			}

			// Find modules eligible for upgrade (Tent, Scrap Heap, or Generator)
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
				_, _ = tx.ExecContext(ctx, "INSERT INTO modules (encampment_id, type, level) VALUES ($1, 'tent', 1) ON CONFLICT DO NOTHING", a.CampID)
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET energy = $1 WHERE encampment_id = $2", newEnergy, a.CampID)
				continue
			}

			cost := lvl * 150
			if a.Scrap >= float64(cost) {
				newScrap := a.Scrap - float64(cost)
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = $1, energy = $2 WHERE encampment_id = $3", newScrap, newEnergy, a.CampID)

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

				alertMsg := fmt.Sprintf(
					"🤖 AGENT AUTOMATED CONSTRUCTION\n\n"+
						"Outpost: %s\n"+
						"Your Agent has initiated an upgrade on your [%s] to Level %d.\n"+
						"⚙️ Construction Cost: %d Scrap deducted.",
					a.CampName, modType, lvl+1, cost,
				)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", a.UserID, alertMsg)
				log.Printf("Agent [Builder] auto-triggered upgrade for module %s level %d on camp %s", modType, lvl+1, a.CampName)
			} else {
				// Scrap is insufficient, builder agent waits but still consumes fuel
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET energy = $1 WHERE encampment_id = $2", newEnergy, a.CampID)
			}

		case "military":
			var rations, iron float64
			_ = tx.QueryRowContext(ctx, "SELECT rations, iron FROM resources WHERE encampment_id = $1", a.CampID).Scan(&rations, &iron)

			if rations >= 50.0 && iron >= 10.0 {
				newRations := rations - 50.0
				newIron := iron - 10.0
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET rations = $1, iron = $2, energy = $3 WHERE encampment_id = $4", newRations, newIron, newEnergy, a.CampID)
				_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + 1 WHERE encampment_id = $1", a.CampID)
				
				log.Printf("Agent [Military] auto-recruited 1 Soldier for outpost: %s", a.CampName)
			} else {
				// Rations or materials are insufficient, military agent waits but still consumes fuel
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET energy = $1 WHERE encampment_id = $2", newEnergy, a.CampID)
			}
		}
	}

	return nil
}