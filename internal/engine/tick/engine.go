package tick

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/engine/agent"
	"github.com/NomadDigita/The-Vagabond/internal/engine/resource"
	"github.com/NomadDigita/The-Vagabond/internal/engine/starvation"
)

type Engine struct {
	DB                *sql.DB
	TickInterval      time.Duration
	stopChan          chan struct{}
	resourceProcessor *resource.Processor
	starvationEngine  *starvation.Engine
	agentProcessor    *agent.Processor
}

func NewEngine(db *sql.DB, interval time.Duration) *Engine {
	return &Engine{
		DB:                db,
		TickInterval:      interval,
		stopChan:          make(chan struct{}),
		resourceProcessor: resource.NewProcessor(db),
		starvationEngine:  starvation.NewEngine(db),
		agentProcessor:    agent.NewProcessor(db),
	}
}

func (e *Engine) Start() {
	log.Printf("Tick Engine initialized. Run interval: %v", e.TickInterval)
	ticker := time.NewTicker(e.TickInterval)

	go func() {
		for {
			select {
			case <-ticker.C:
				e.ProcessTick()
			case <-e.stopChan:
				ticker.Stop()
				log.Println("Tick Engine background goroutine stopped.")
				return
			}
		}
	}()
}

func (e *Engine) Stop() {
	close(e.stopChan)
}

func (e *Engine) ProcessTick() {
	start := time.Now()
	log.Println("⌛ Processing master game tick pass...")

	ctx := context.Background()

	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("Tick Engine failed to initiate database transaction: %v", err)
		return
	}
	defer tx.Rollback()

	// Pass 1: Resource production/consumption
	if err := e.resourceProcessor.RunResourcePass(ctx, tx); err != nil {
		log.Printf("Error during Tick Resource Pass execution: %v", err)
		return
	}

	// Pass 2: Starvation processing
	if err := e.starvationEngine.RunStarvationPass(ctx, tx); err != nil {
		log.Printf("Error during Tick Starvation Pass execution: %v", err)
		return
	}

	// Pass 3: Construction timers checks
	if err := e.resolveCompletedUpgrades(ctx, tx); err != nil {
		log.Printf("Error during Construction Upgrade Pass execution: %v", err)
		return
	}

	// Pass 4: AGENT AUTOMATION PASS
	if err := e.agentProcessor.RunAgentPass(ctx, tx); err != nil {
		log.Printf("Error during Agent Automation Pass execution: %v", err)
		return
	}

	// Pass 5: Tactical combat marches checks
	if err := e.resolveRaidCombats(ctx, tx); err != nil {
		log.Printf("Error during Combat Resolution Pass: %v", err)
		return
	}

	// Pass 6: Clear expired world events
	deleteExpiredEvents := `
		DELETE FROM world_events 
		WHERE expires_at < CURRENT_TIMESTAMP`
	_, err = tx.ExecContext(ctx, deleteExpiredEvents)
	if err != nil {
		log.Printf("Tick Engine failed cleaning expired world events: %v", err)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Tick Engine failed to commit transaction updates: %v", err)
		return
	}

	log.Printf("Tick pass successfully calculated and committed. Duration: %s", time.Since(start))
}

func (e *Engine) resolveCompletedUpgrades(ctx context.Context, tx *sql.Tx) error {
	query := `
		SELECT m.id, e.user_id, e.name, m.type, m.level
		FROM modules m
		JOIN encampments e ON e.id = m.encampment_id
		WHERE m.is_upgrading = TRUE AND m.upgrade_ready_at <= CURRENT_TIMESTAMP`

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed selecting completed upgrades: %w", err)
	}
	defer rows.Close()

	type completedUpgrade struct {
		id       string
		userID   int64
		campName string
		modType  string
		oldLvl   int
	}

	var completed []completedUpgrade
	for rows.Next() {
		var c completedUpgrade
		if err := rows.Scan(&c.id, &c.userID, &c.campName, &c.modType, &c.oldLvl); err == nil {
			completed = append(completed, c)
		}
	}
	rows.Close()

	for _, c := range completed {
		newLvl := c.oldLvl + 1
		updateQuery := `
			UPDATE modules 
			SET level = $1, is_upgrading = FALSE, upgrade_ready_at = NULL 
			WHERE id = $2`
		_, err := tx.ExecContext(ctx, updateQuery, newLvl, c.id)
		if err != nil {
			log.Printf("Failed executing module upgrade writeback: %v", err)
			continue
		}

		alertMsg := fmt.Sprintf(
			"🏗️ CONSTRUCTION COMPLETE\n\n"+
				"Outpost: %s\n"+
				"Upgrade completed successfully!\n"+
				"Your [%s] has successfully reached Level %d.",
			c.campName, c.modType, newLvl,
		)

		insertNotification := `
			INSERT INTO notifications (user_id, message, is_sent) 
			VALUES ($1, $2, FALSE)`
		_, err = tx.ExecContext(ctx, insertNotification, c.userID, alertMsg)
		if err != nil {
			log.Printf("Failed queuing upgrade completed push alert: %v", err)
		}
	}

	return nil
}

func (e *Engine) resolveRaidCombats(ctx context.Context, tx *sql.Tx) error {
	query := `
		SELECT r.id, r.attacker_id, r.defender_id,
		       ea.name as attacker_name, ea.user_id as attacker_user_id,
		       ed.name as defender_name, ed.user_id as defender_user_id
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		JOIN encampments ed ON ed.id = r.defender_id
		WHERE r.state = 'marching' AND r.resolve_time <= CURRENT_TIMESTAMP`

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed querying matching combat raids: %w", err)
	}
	defer rows.Close()

	type activeRaid struct {
		id             string
		attackerID     string
		defenderID     string
		attackerName   string
		attackerUserID int64
		defenderName   string
		defenderUserID int64
	}

	var raids []activeRaid
	for rows.Next() {
		var r activeRaid
		if err := rows.Scan(&r.id, &r.attackerID, &r.defenderID, &r.attackerName, &r.attackerUserID, &r.defenderName, &r.defenderUserID); err == nil {
			raids = append(raids, r)
		}
	}
	rows.Close()

	for _, r := range raids {
		var attackForce int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(quantity), 0) FROM units WHERE encampment_id = $1", r.attackerID).Scan(&attackForce)

		var defenseForce int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(SUM(quantity), 0) FROM units WHERE encampment_id = $1", r.defenderID).Scan(&defenseForce)

		if attackForce == 0 {
			attackForce = 1
		}

		var defLevel int
		_ = tx.QueryRowContext(ctx, "SELECT level FROM modules WHERE encampment_id = $1 AND type = 'tent'", r.defenderID).Scan(&defLevel)
		if defLevel == 0 {
			defLevel = 1
		}
		defenseShieldMultiplier := 1.0 + (float64(defLevel) * 0.15)

		attackerOffenseRating := float64(attackForce) * 15.0
		defenderDefenseRating := float64(defenseForce) * 10.0 * defenseShieldMultiplier

		attackerCasualties := 0
		defenderCasualties := 0

		if attackerOffenseRating > defenderDefenseRating {
			defenderCasualties = defenseForce
			attackerCasualties = attackForce / 2
		} else {
			attackerCasualties = attackForce
			defenderCasualties = defenseForce / 3
		}

		if attackerCasualties > 0 {
			_, _ = tx.ExecContext(ctx, "UPDATE units SET quantity = GREATEST(quantity - $1, 0) WHERE encampment_id = $2", attackerCasualties, r.attackerID)
		}
		if defenderCasualties > 0 {
			_, _ = tx.ExecContext(ctx, "UPDATE units SET quantity = GREATEST(quantity - $1, 0) WHERE encampment_id = $2", defenderCasualties, r.defenderID)
		}
		_, _ = tx.ExecContext(ctx, "DELETE FROM units WHERE quantity <= 0")

		var defenderScrap float64
		_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", r.defenderID).Scan(&defenderScrap)

		stolenScrap := defenderScrap * 0.40
		if stolenScrap < 0 {
			stolenScrap = 0
		}

		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap + $1 WHERE encampment_id = $2", stolenScrap, r.attackerID)
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = GREATEST(scrap - $1, 0) WHERE encampment_id = $2", stolenScrap, r.defenderID)

		_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'completed' WHERE id = $1", r.id)

		attackerAlert := fmt.Sprintf(
			"⚔️ RAID REPORT: VICTORY!\n\n"+
				"Target: %s\n"+
				"Your raiders breached the base defense grid.\n"+
				"⚙️ Looted: %.1f Scrap\n"+
				"💀 Casualties Sustained: %d units",
			r.defenderName, stolenScrap, attackerCasualties,
		)
		if attackerOffenseRating <= defenderDefenseRating {
			attackerAlert = fmt.Sprintf(
				"❌ RAID REPORT: DEFEAT!\n\n"+
					"Target: %s\n"+
					"Your forces were repelled. March failed.\n"+
					"💀 Casualties Sustained: All %d units lost.",
				r.defenderName, attackerCasualties,
			)
		}

		defenderAlert := fmt.Sprintf(
			"🚨 OUTPOST UNDER ATTACK!\n\n"+
				"Attacker: %s\n"+
				"Intruders breached your gates.\n"+
				"⚙️ Scrap Looted: %.1f Scrap\n"+
				"💀 Defense Casualties: %d units lost.",
			r.attackerName, stolenScrap, defenderCasualties,
		)

		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.attackerUserID, attackerAlert)
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.defenderUserID, defenderAlert)

		log.Printf("Combat Raid Resolved: %s raided %s. Result Attacked Casualties: %d, Defender Casualties: %d, Stolen Scrap: %.1f", r.attackerName, r.defenderName, attackerCasualties, defenderCasualties, stolenScrap)
	}

	return nil
}
