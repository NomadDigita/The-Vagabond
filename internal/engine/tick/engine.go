package tick

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/engine/resource"
	"github.com/NomadDigita/The-Vagabond/internal/engine/starvation"
	"github.com/NomadDigita/The-Vagabond/internal/engine/world"
)

type Engine struct {
	DB                *sql.DB
	TickInterval      time.Duration
	stopChan          chan struct{}
	resourceProcessor *resource.Processor
	starvationEngine  *starvation.Engine
	weatherEngine     *world.WeatherEngine
}

func NewEngine(db *sql.DB, interval time.Duration) *Engine {
	return &Engine{
		DB:                db,
		TickInterval:      interval,
		stopChan:          make(chan struct{}),
		resourceProcessor: resource.NewProcessor(db),
		starvationEngine:  starvation.NewEngine(db),
		weatherEngine:     world.NewWeatherEngine(db),
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

	// Pass 1: Global Weather progression check
	if err := e.weatherEngine.RunWeatherPass(ctx, tx); err != nil {
		log.Printf("Error during Tick Weather Pass: %v", err)
		return
	}

	// Pass 2: Resource production/consumption
	if err := e.resourceProcessor.RunResourcePass(ctx, tx); err != nil {
		log.Printf("Error during Tick Resource Pass execution: %v", err)
		return
	}

	// Pass 3: Starvation check
	if err := e.starvationEngine.RunStarvationPass(ctx, tx); err != nil {
		log.Printf("Error during Tick Starvation Pass execution: %v", err)
		return
	}

	// Pass 4: Construction upgrades
	if err := e.resolveCompletedUpgrades(ctx, tx); err != nil {
		log.Printf("Error during Construction Upgrade Pass execution: %v", err)
		return
	}

	// Pass 5: PvP target and AI Skirmish combat marches
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
	// Rebuilt to scan both 'marching' and 'engaged' multi-layered operational phases
	query := `
		SELECT r.id, r.attacker_id, r.defender_id, r.state, r.round_number,
		       r.attacker_rations, r.attacker_ammo, r.attacker_losses, r.defender_losses,
		       ea.name as attacker_name, ea.user_id as attacker_user_id,
		       COALESCE(ed.name, 'Rogue Drone Nest') as defender_name, 
		       COALESCE(ed.user_id, 0) as defender_user_id, r.resolve_time
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		LEFT JOIN encampments ed ON ed.id = r.defender_id
		WHERE (r.state = 'marching' OR r.state = 'engaged') AND r.resolve_time <= CURRENT_TIMESTAMP`

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed querying combat raids: %w", err)
	}
	defer rows.Close()

	type activeRaid struct {
		id              string
		attackerID      string
		defenderID      string
		state           string
		roundNumber     int
		attackerRations float64
		attackerAmmo    float64
		attackerLosses  int
		defenderLosses  int
		attackerName    string
		attackerUserID  int64
		defenderName    string
		defenderUserID  int64
		resolveTime     time.Time
	}

	var raids []activeRaid
	for rows.Next() {
		var r activeRaid
		err := rows.Scan(
			&r.id, &r.attackerID, &r.defenderID,
			&r.attackerName, &r.attackerUserID,
			&r.defenderName, &r.defenderUserID,
		)
		if err == nil {
			raids = append(raids, r)
		}
	}
	rows.Close()

	for _, r := range raids {
		// --- PHASE 4: ACTIVE MULTI-STAGE ENGAGEMENT TRANSITION ---
		if r.state == "marching" {
			// Outpost troops arrived at the battlefield coordinate grid. Shift to 'engaged' battle!
			// Small battles take 15m, Large scale battles take up to 90m (1h 30m)
			battleDuration := 20 * time.Minute
			if r.defenderID == "00000000-0000-0000-0000-000000000000" {
				battleDuration = 15 * time.Minute // Rogue Drone Skirmish standard duration
			}

			newResolve := time.Now().Add(battleDuration)
			updateMarch := `
				UPDATE raids 
				SET state = 'engaged', resolve_time = $1, round_number = 1 
				WHERE id = $2`
			_, _ = tx.ExecContext(ctx, updateMarch, newResolve, r.id)

			arrivalAlert := fmt.Sprintf(
				"⚔️ CAMPAIGN ENGAGEMENT ACTIVATED!\n\n"+
					"Your forces have arrived at Sector coordinates for Outpost [%s].\n"+
					"Deployments are actively engaged in battlefield skirmishes.\n"+
					"Estimated Battle Resolution: %s.",
				r.defenderName, newResolve.UTC().Format("15:04:05"),
			)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.attackerUserID, arrivalAlert)
			continue
		}

		// --- PROCESSING ACTIVE COMBAT ROUNDS (state = 'engaged') ---
		var soldiersAttacker, dronesAttacker, jetsAttacker, mechsAttacker int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers, 0), COALESCE(drones, 0), COALESCE(jets, 0), COALESCE(mechs, 0) FROM workshop_inventory WHERE encampment_id = $1", r.attackerID).Scan(&soldiersAttacker, &dronesAttacker, &jetsAttacker, &mechsAttacker)

		attackForce := soldiersAttacker + dronesAttacker + jetsAttacker + mechsAttacker

		var attackerTanks, attackerMechs int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(fusion_tanks, 0), COALESCE(mechs, 0) FROM workshop_inventory WHERE encampment_id = $1", r.attackerID).Scan(&attackerTanks, &attackerMechs)

		var defenseForce int
		var defLevel int = 1
		var defenderShields int = 0
		var defenderAgentActive bool = false
		var targetBiome string = "wasteland"

		if r.defenderID == "00000000-0000-0000-0000-000000000000" {
			var attackerCoreLvl int
			_ = tx.QueryRowContext(ctx, "SELECT level FROM encampments WHERE id = $1", r.attackerID).Scan(&attackerCoreLvl)
			if attackerCoreLvl <= 0 {
				attackerCoreLvl = 1
			}
			defenseForce = attackerCoreLvl * 18
			defLevel = attackerCoreLvl
		} else {
			var soldiersDefender, dronesDefender, jetsDefender, mechsDefender int
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers, 0), COALESCE(drones, 0), COALESCE(jets, 0), COALESCE(mechs, 0) FROM workshop_inventory WHERE encampment_id = $1", r.defenderID).Scan(&soldiersDefender, &dronesDefender, &jetsDefender, &mechsDefender)
			defenseForce = soldiersDefender + dronesDefender + jetsDefender + mechsDefender

			_ = tx.QueryRowContext(ctx, "SELECT level FROM modules WHERE encampment_id = $1 AND type = 'tent'", r.defenderID).Scan(&defLevel)
			if defLevel == 0 {
				defLevel = 1
			}

			_ = tx.QueryRowContext(ctx, "SELECT COALESCE((SELECT nuclear_shields FROM workshop_inventory WHERE encampment_id = $1), 0)", r.defenderID).Scan(&defenderShields)
			_ = tx.QueryRowContext(ctx, "SELECT is_active FROM agent_tasks WHERE user_id = $1", r.defenderUserID).Scan(&defenderAgentActive)
			_ = tx.QueryRowContext(ctx, "SELECT c.biome FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", r.defenderID).Scan(&targetBiome)
		}

		var activeWeather string
		_ = tx.QueryRowContext(ctx, "SELECT active_weather FROM world_state WHERE id = 1").Scan(&activeWeather)

		offenseRatingModifier := 1.0
		defenseRatingModifier := 1.0 + (float64(defLevel) * 0.15)

		// Apply Terrain Modifications
		switch targetBiome {
		case "forest":
			offenseRatingModifier *= 0.85
			defenseRatingModifier *= 1.15
		case "ruins":
			offenseRatingModifier *= 0.75
			defenseRatingModifier *= 1.30
		}

		// Apply Weather Modifications
		switch activeWeather {
		case "radiation_storm":
			offenseRatingModifier *= 0.75
		case "acid_rain":
			attackerMechs = int(float64(attackerMechs) * 0.50)
		}

		// --- SUPPLY & LOGISTICS DEBUFFS (Phase 4) ---
		if r.attackerRations <= 0 || r.attackerAmmo <= 0 {
			offenseRatingModifier *= 0.50 // Performance drop due to hunger or ammunition depletion
		}

		attackerOffenseRating := (float64(attackForce) * 15.0 * offenseRatingModifier) * (1.0 + (float64(attackerTanks) * 0.50) + (float64(attackerMechs) * 1.50))
		defenderDefenseRating := float64(defenseForce) * 10.0 * defenseRatingModifier

		attackerCasualties := 0
		defenderCasualties := 0

		if attackerOffenseRating > defenderDefenseRating {
			defenderCasualties = defenseForce
			attackerCasualties = attackForce / 2
		} else {
			attackerCasualties = attackForce
			defenderCasualties = defenseForce / 3
		}

		// Write final casualties and finalize
		if attackerCasualties > 0 {
			_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = GREATEST(soldiers - $1, 0) WHERE encampment_id = $2", attackerCasualties, r.attackerID)
		}
		if r.defenderID != "00000000-0000-0000-0000-000000000000" && defenderCasualties > 0 {
			_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = GREATEST(soldiers - $1, 0) WHERE encampment_id = $2", defenderCasualties, r.defenderID)
		}

		var defenderScrap float64
		if r.defenderID == "00000000-0000-0000-0000-000000000000" {
			defenderScrap = 125.0
		} else {
			_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", r.defenderID).Scan(&defenderScrap)
		}

		lootPercentage := 0.40
		if defenderShields > 0 {
			lootPercentage = 0.20
		}

		stolenScrap := defenderScrap * lootPercentage

		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap + $1 WHERE encampment_id = $2", stolenScrap, r.attackerID)
		if r.defenderID != "00000000-0000-0000-0000-000000000000" {
			_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = GREATEST(scrap - $1, 0) WHERE encampment_id = $2", stolenScrap, r.defenderID)
		}

		// Complete the raid
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

		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.attackerUserID, attackerAlert)

		if r.defenderID != "00000000-0000-0000-0000-000000000000" {
			defenderAlert := fmt.Sprintf(
				"🚨 OUTPOST UNDER ATTACK!\n\n"+
					"Attacker: %s\n"+
					"Intruders breached your gates.\n"+
					"⚙️ Scrap Looted: %.1f Scrap\n"+
					"💀 Defense Casualties: %d units lost.",
				r.attackerName, stolenScrap, defenderCasualties,
			)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.defenderUserID, defenderAlert)
		}

		log.Printf("Combat Raid Resolved: %s raided %s. Result Attacker Casualties: %d, Defender Casualties: %d", r.attackerName, r.defenderName, attackerCasualties, defenderCasualties)
	}

	return nil
}
