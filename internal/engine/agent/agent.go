package agent

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
)

type Processor struct {
	DB *sql.DB
}

func NewProcessor(db *sql.DB) *Processor {
	return &Processor{DB: db}
}

type ActiveAgent struct {
	UserID      int64
	Mode        string
	CampID      string
	CampName    string
	Scrap       float64
	Rations     float64
	Electricity float64
	Metal       float64
	Crystal     float64
	Hydrogen    float64

	Dollars      float64
	NeuroCores   float64
	TentLvl      int
	CampLvl      int
	WarehouseLvl int
	ExtensionLvl int
	EconTechLvl  int
	SynapticLvl  int
}

// RunAgentPass executes automation logic for all active agents inside the transaction
func (p *Processor) RunAgentPass(ctx context.Context, tx *sql.Tx) error {
	query := `
		SELECT t.user_id, t.mode, e.id, e.name, 
		       r.scrap, r.rations, r.electricity, r.metal, r.crystal, r.hydrogen, r.dollars, r.neuro_cores,
		       COALESCE((SELECT m.level FROM modules m WHERE m.encampment_id = e.id AND m.type = 'tent'), 1) as tent_lvl,
		       e.level as camp_lvl,
		       COALESCE((SELECT m.level FROM modules m WHERE m.encampment_id = e.id AND m.type = 'warehouse'), 0) as warehouse_lvl,
		       COALESCE(e.extension_lvl, 0) as extension_lvl,
		       COALESCE((SELECT res.econ_tech_lvl FROM research_states res WHERE res.encampment_id = e.id), 1) as econ_tech_lvl,
		       COALESCE((SELECT mut.synaptic_lvl FROM mutation_states mut WHERE mut.encampment_id = e.id), 1) as synaptic_lvl
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
			&a.Scrap, &a.Rations, &a.Electricity, &a.Metal, &a.Crystal, &a.Hydrogen, &a.Dollars, &a.NeuroCores,
			&a.TentLvl, &a.CampLvl, &a.WarehouseLvl, &a.ExtensionLvl, &a.EconTechLvl, &a.SynapticLvl,
		)
		if err == nil {
			agents = append(agents, a)
		} else {
			log.Printf("Error scanning agent task row: %v", err)
		}
	}
	rows.Close()

	for _, a := range agents {
		// Calculate fuel deductions incorporating Science Tech and Biological Mutations
		upkeepReduction := (float64(a.EconTechLvl-1) * 0.15) + (float64(a.SynapticLvl-1) * 0.10)
		upkeepMultiplier := math.Max(1.0-upkeepReduction, 0.10) // Cap minimum electricity upkeep at 10%
		upkeepEnergy := 2.0 * upkeepMultiplier

		if a.Electricity < upkeepEnergy {
			_, _ = tx.ExecContext(ctx, "UPDATE agent_tasks SET is_active = FALSE WHERE user_id = $1", a.UserID)

			alertMsg := fmt.Sprintf(
				"🔌 AGENT DEACTIVATED\n\n"+
					"Outpost: %s\n"+
					"Your automation agent has shut down due to complete depletion of Electricity Cells.",
				a.CampName,
			)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", a.UserID, alertMsg)
			log.Printf("Agent auto-shut down for user %d: lack of electricity.", a.UserID)
			continue
		}

		newEnergy := math.Max(a.Electricity-upkeepEnergy, 0.0)
		storageCap := storagecap.Cap(a.TentLvl, a.WarehouseLvl, a.ExtensionLvl)

		switch a.Mode {
		case "collector":
			newScrap, _ := storagecap.Clamp(a.Scrap, 5.00, storageCap)
			newRations, _ := storagecap.Clamp(a.Rations, 2.00, storageCap)

			_, err = tx.ExecContext(ctx, `
				UPDATE resources 
				SET scrap = $1, rations = $2, electricity = $3 
				WHERE encampment_id = $4`,
				newScrap, newRations, newEnergy, a.CampID,
			)
			if err != nil {
				log.Printf("Agent failed executing collector pass: %v", err)
			}
			log.Printf("Agent [Collector] executed action for outpost: %s (+5.0 Scrap, +2.0 Rations capped at %.0f)", a.CampName, storageCap)

		case "collector_omega":
			// Autopilot Industrial mode (Metal, Hydrogen). Former separate
			// Iron/Oil gains are now folded directly into Metal.
			// Same Surplus Preservation cap rule as `collector`: no
			// further passive gain once a resource is at/above cap.
			newMetal, _ := storagecap.Clamp(a.Metal, 33.00, storageCap)
			newHydrogen, _ := storagecap.Clamp(a.Hydrogen, 5.00, storageCap)

			_, err = tx.ExecContext(ctx, `
				UPDATE resources 
				SET electricity = $1, metal = $2, hydrogen = $3 
				WHERE encampment_id = $4`,
				newEnergy, newMetal, newHydrogen, a.CampID,
			)
			if err != nil {
				log.Printf("Agent failed executing collector_omega pass: %v", err)
			}
			log.Printf("Agent [Collector Ω] executed resource extraction pass for outpost: %s (capped at %.0f)", a.CampName, storageCap)

		case "collector_precious":
			// Autopilot Precious mode (Crystal, Dollars, Neuro Cores).
			// Former separate Silver/Gold/Diamond gains are now folded
			// directly into Crystal. Same cap rule as the other
			// collector modes.
			newCrystal, _ := storagecap.Clamp(a.Crystal, 8.10, storageCap)
			newDollars, _ := storagecap.Clamp(a.Dollars, 2.00, storageCap)
			newNeuro, _ := storagecap.Clamp(a.NeuroCores, 1.00, storageCap)

			_, err = tx.ExecContext(ctx, `
				UPDATE resources 
				SET electricity = $1, crystal = $2, dollars = $3, neuro_cores = $4 
				WHERE encampment_id = $5`,
				newEnergy, newCrystal, newDollars, newNeuro, a.CampID,
			)
			if err != nil {
				log.Printf("Agent failed executing collector_precious pass: %v", err)
			}
			log.Printf("Agent [Collector Precious] executed resource extraction pass for outpost: %s (capped at %.0f)", a.CampName, storageCap)

		case "builder":
			var isUpgrading bool
			_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM modules WHERE encampment_id = $1 AND is_upgrading = TRUE)", a.CampID).Scan(&isUpgrading)
			if isUpgrading {
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = $1 WHERE encampment_id = $2", newEnergy, a.CampID)
				continue
			}

			// Same "Module levels cannot exceed your Outpost Core level"
			// rule enforced on the manual upgrade path (camp.go
			// HandleUpgradeCallback: `currentLvl >= campLvl` is blocked).
			// Only consider modules still below that cap so the agent
			// can't auto-build past what a manual upgrade would allow.
			queryEligible := `
				SELECT type, level 
				FROM modules 
				WHERE encampment_id = $1 
				  AND level < $2
				ORDER BY level ASC 
				LIMIT 1`

			var modType string
			var lvl int
			err = tx.QueryRowContext(ctx, queryEligible, a.CampID, a.CampLvl).Scan(&modType, &lvl)
			if err != nil {
				if err == sql.ErrNoRows {
					// Either no modules exist yet, or every module is
					// already capped at the Outpost Core level. Seed a
					// starter tent only if nothing exists at all.
					var anyModule bool
					_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM modules WHERE encampment_id = $1)", a.CampID).Scan(&anyModule)
					if !anyModule {
						_, _ = tx.ExecContext(ctx, "INSERT INTO modules (encampment_id, type, level) VALUES ($1, 'tent', 1) ON CONFLICT DO NOTHING", a.CampID)
					}
				} else {
					log.Printf("Agent [Builder] failed querying eligible module: %v", err)
				}
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = $1 WHERE encampment_id = $2", newEnergy, a.CampID)
				continue
			}

			cost := lvl * 150
			if a.Scrap >= float64(cost) {
				newScrap := a.Scrap - float64(cost)
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = $1, electricity = $2 WHERE encampment_id = $3", newScrap, newEnergy, a.CampID)

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
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = $1 WHERE encampment_id = $2", newEnergy, a.CampID)
			}

		case "military":
			var rations, metal float64
			_ = tx.QueryRowContext(ctx, "SELECT rations, metal FROM resources WHERE encampment_id = $1", a.CampID).Scan(&rations, &metal)

			// Same Hangar capacity rule enforced on the manual Recruit
			// Soldier path (factory.go HandleCraftItemCallback):
			// maxCapacity = 50 + hangarLvl*20, blocked once totalUnits
			// hits that cap. The agent must not be able to auto-recruit
			// past what a manual recruit would allow.
			var hangarLvl int
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'hangar'", a.CampID).Scan(&hangarLvl)
			maxCapacity := 50 + hangarLvl*20

			var totalUnits int
			_ = tx.QueryRowContext(ctx, `
				SELECT COALESCE(soldiers,0)+COALESCE(drones,0)+COALESCE(mechs,0)+COALESCE(nukes,0)+COALESCE(buggies,0)+COALESCE(ships,0)+COALESCE(jets,0)+
				       COALESCE(haulers,0)+COALESCE(tankers,0)+COALESCE(rigs,0)+COALESCE(destroyers,0)+COALESCE(bombers,0)+COALESCE(scouts,0)+
				       COALESCE(battlecruisers,0)+COALESCE(deathstars,0)+COALESCE(liberators,0)+COALESCE(wraiths,0)+COALESCE(observers,0)+
				       COALESCE(guardians,0)+COALESCE(piercing_missiles,0)+COALESCE(cargo_mk1,0)+COALESCE(cargo_mk2,0)+COALESCE(cargo_mk3,0)
				FROM workshop_inventory WHERE encampment_id = $1`, a.CampID).Scan(&totalUnits)

			if rations >= 50.0 && metal >= 10.0 && totalUnits < maxCapacity {
				newRations := rations - 50.0
				newMetal := metal - 10.0
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET rations = $1, metal = $2, electricity = $3 WHERE encampment_id = $4", newRations, newMetal, newEnergy, a.CampID)
				_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + 1 WHERE encampment_id = $1", a.CampID)

				log.Printf("Agent [Military] auto-recruited 1 Soldier for outpost: %s", a.CampName)
			} else {
				if rations >= 50.0 && metal >= 10.0 && totalUnits >= maxCapacity {
					log.Printf("Agent [Military] skipped recruit for outpost %s: Hangar full (%d/%d)", a.CampName, totalUnits, maxCapacity)
				}
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = $1 WHERE encampment_id = $2", newEnergy, a.CampID)
			}
		}
	}

	return nil
}
