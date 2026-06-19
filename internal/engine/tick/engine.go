package tick

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
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

	// Pass 1: Global Weather check
	if err := e.weatherEngine.RunWeatherPass(ctx, tx); err != nil {
		log.Printf("Error during Tick Weather Pass: %v", err)
		return
	}

	// Pass 2: Resource calculations
	if err := e.resourceProcessor.RunResourcePass(ctx, tx); err != nil {
		log.Printf("Error during Tick Resource Pass execution: %v", err)
		return
	}

	// Pass 3: Active logistics depletion
	if err := e.applyActiveLogisticsConsumption(ctx, tx); err != nil {
		log.Printf("Error during Active Logistics Consumption: %v", err)
		return
	}

	// Pass 4: Arena Matchmaker Queue Resolution
	if err := e.processArenaMatchmaking(ctx, tx); err != nil {
		log.Printf("Error during Arena Matchmaker sweep: %v", err)
		return
	}

	// Pass 5: Resolve pending espionage missions
	if err := e.resolvePendingEspionageMissions(ctx, tx); err != nil {
		log.Printf("Error during Espionage resolution: %v", err)
		return
	}

	// Pass 6: Resolve pending time-based mining queues (Phase 2 Addition)
	if err := e.resolveCompletedMiningQueues(ctx, tx); err != nil {
		log.Printf("Error during Active Mining Resolution: %v", err)
		return
	}

	// Pass 7: Starvation decay calculations
	if err := e.starvationEngine.RunStarvationPass(ctx, tx); err != nil {
		log.Printf("Error during Tick Starvation Pass execution: %v", err)
		return
	}

	// Pass 8: Construction upgrades
	if err := e.resolveCompletedUpgrades(ctx, tx); err != nil {
		log.Printf("Error during Construction Upgrade Pass execution: %v", err)
		return
	}

	// Pass 9: PvP target and AI Skirmish combat resolutions (Supports return march transition)
	if err := e.resolveRaidCombats(ctx, tx); err != nil {
		log.Printf("Error during Combat Resolution Pass: %v", err)
		return
	}

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

// resolveCompletedMiningQueues finalizes active extraction tasks
func (e *Engine) resolveCompletedMiningQueues(ctx context.Context, tx *sql.Tx) error {
	queryCompleted := `
		SELECT q.id, q.encampment_id, q.resource_type, q.miners_assigned, e.user_id
		FROM active_mining_queues q
		JOIN encampments e ON e.id = q.encampment_id
		WHERE q.is_completed = FALSE AND q.ready_at <= CURRENT_TIMESTAMP`

	rows, err := tx.QueryContext(ctx, queryCompleted)
	if err != nil {
		return fmt.Errorf("failed scanning finished mining queues: %w", err)
	}
	defer rows.Close()

	type completedMine struct {
		id           string
		encampmentID string
		resType      string
		miners       int
		userID       int64
	}

	var completed []completedMine
	for rows.Next() {
		var m completedMine
		if err := rows.Scan(&m.id, &m.encampmentID, &m.resType, &m.miners, &m.userID); err == nil {
			completed = append(completed, m)
		}
	}
	rows.Close()

	for _, m := range completed {
		var gain float64
		var column string

		switch m.resType {
		case "iron":
			gain = float64(m.miners * 20)
			column = "iron"
		case "oil":
			gain = float64(m.miners * 10)
			column = "oil"
		case "gold":
			gain = float64(m.miners * 5)
			column = "gold"
		case "silver":
			gain = float64(m.miners * 10)
			column = "silver"
		case "diamond":
			gain = float64(m.miners * 1)
			column = "diamond"
		case "uranium":
			gain = float64(m.miners * 5)
			column = "uranium"
		case "hydrogen":
			gain = float64(m.miners * 10)
			column = "hydrogen"
		case "steel":
			gain = float64(m.miners * 20)
			column = "steel"
		}

		queryUpdateRes := fmt.Sprintf("UPDATE resources SET %s = %s + $1 WHERE encampment_id = $2", column, column)
		_, _ = tx.ExecContext(ctx, queryUpdateRes, gain, m.encampmentID)

		_, _ = tx.ExecContext(ctx, "UPDATE active_mining_queues SET is_completed = TRUE WHERE id = $1", m.id)

		alertMsg := fmt.Sprintf("⛏️ EXTRACTION COMPLETE: Your miners successfully returned with +%.1f %s!", gain, m.resType)
		queryNotif := "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)"
		_, _ = tx.ExecContext(ctx, queryNotif, m.userID, alertMsg)
	}

	return nil
}

// resolvePendingEspionageMissions checks and finalizes satellites after the 30-second window expires
func (e *Engine) resolvePendingEspionageMissions(ctx context.Context, tx *sql.Tx) error {
	queryPendingSpies := `
		SELECT s.id, s.spy_id, s.target_id, s.is_intercepted, ea.user_id as spy_user_id
		FROM spy_missions s
		JOIN encampments ea ON ea.id = s.spy_id
		WHERE s.resolved = FALSE AND s.created_at <= CURRENT_TIMESTAMP - INTERVAL '30 seconds'`

	rows, err := tx.QueryContext(ctx, queryPendingSpies)
	if err != nil {
		return fmt.Errorf("failed querying pending espionage missions: %w", err)
	}
	defer rows.Close()

	type espionageMission struct {
		id            string
		spyID         string
		targetID      string
		isIntercepted bool
		spyUserID     int64
	}

	var missions []espionageMission
	for rows.Next() {
		var m espionageMission
		if err := rows.Scan(&m.id, &m.spyID, &m.targetID, &m.isIntercepted, &m.spyUserID); err == nil {
			missions = append(missions, m)
		}
	}
	rows.Close()

	for _, m := range missions {
		if m.isIntercepted {
			_, _ = tx.ExecContext(ctx, "UPDATE spy_missions SET resolved = TRUE WHERE id = $1", m.id)
			continue
		}

		var targetName string
		var targetLvl int
		_ = tx.QueryRowContext(ctx, "SELECT name, level FROM encampments WHERE id = $1", m.targetID).Scan(&targetName, &targetLvl)

		var tentLvl, heapLvl, genLvl int
		_ = tx.QueryRowContext(ctx, "SELECT level FROM modules WHERE encampment_id = $1 AND type = 'tent'", m.targetID).Scan(&tentLvl)
		_ = tx.QueryRowContext(ctx, "SELECT level FROM modules WHERE encampment_id = $1 AND type = 'scrap_heap'", m.targetID).Scan(&heapLvl)
		_ = tx.QueryRowContext(ctx, "SELECT level FROM modules WHERE encampment_id = $1 AND type = 'generator'", m.targetID).Scan(&genLvl)

		var upgradingModule string
		_ = tx.QueryRowContext(ctx, "SELECT type FROM modules WHERE encampment_id = $1 AND is_upgrading = TRUE LIMIT 1", m.targetID).Scan(&upgradingModule)
		if upgradingModule == "" {
			upgradingModule = "None"
		}

		var scrap, rations float64
		_ = tx.QueryRowContext(ctx, "SELECT scrap, rations FROM resources WHERE encampment_id = $1", m.targetID).Scan(&scrap, &rations)

		spyReport := fmt.Sprintf(
			"🛰️ SPY SATELLITE DECRYPTOR INDICES\n\n"+
				"Target Outpost: %s\n\n"+
				"DECRYPTED RESOURCES:\n"+
				"⚙️ Scrap: %.1f\n"+
				"🥫 Rations: %.1f\n\n"+
				"MODULE STATUS GRID:\n"+
				"⛺ Tent: Level %d\n"+
				"⚙️ Scrap Heap: Level %d\n"+
				"⚡ Generator: Level %d\n\n"+
				"🔧 Active Upgrades Queue: %s",
			targetName, scrap, rations, tentLvl, heapLvl, genLvl, upgradingModule,
		)

		queryNotif := `
			INSERT INTO notifications (user_id, message, is_sent) 
			VALUES ($1, $2, FALSE)`
		_, _ = tx.ExecContext(ctx, queryNotif, m.spyUserID, spyReport)

		_, _ = tx.ExecContext(ctx, "UPDATE spy_missions SET resolved = TRUE WHERE id = $1", m.id)
	}
	return nil
}

// applyActiveLogisticsConsumption depletes food and fuel during active expeditions
func (e *Engine) applyActiveLogisticsConsumption(ctx context.Context, tx *sql.Tx) error {
	queryExpeditions := `
		SELECT id, attacker_id, state, resolve_time 
		FROM raids 
		WHERE state = 'marching' OR state = 'engaged'`

	rows, err := tx.QueryContext(ctx, queryExpeditions)
	if err != nil {
		return fmt.Errorf("failed fetching active expeditions: %w", err)
	}
	defer rows.Close()

	type activeExp struct {
		id          string
		attackerID  string
		state       string
		resolveTime time.Time
	}

	var exps []activeExp
	for rows.Next() {
		var ex activeExp
		if err := rows.Scan(&ex.id, &ex.attackerID, &ex.state, &ex.resolveTime); err == nil {
			exps = append(exps, ex)
		}
	}
	rows.Close()

	for _, ex := range exps {
		var rations, oil float64
		_ = tx.QueryRowContext(ctx, "SELECT rations, oil FROM resources WHERE encampment_id = $1 FOR UPDATE", ex.attackerID).Scan(&rations, &oil)

		deductRations := 3.0
		deductOil := 1.0

		newRations := math.Max(rations-deductRations, 0.0)
		newOil := math.Max(oil-deductOil, 0.0)

		_, _ = tx.ExecContext(ctx, "UPDATE resources SET rations = $1, oil = $2 WHERE encampment_id = $3", newRations, newOil, ex.attackerID)

		if newOil <= 0 {
			delayedResolve := ex.resolveTime.Add(3 * time.Minute)
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET resolve_time = $1 WHERE id = $2", delayedResolve, ex.id)
		}

		if newRations <= 0 {
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET attacker_rations = 0.0 WHERE id = $1", ex.id)
		}
	}
	return nil
}

// processArenaMatchmaking implements ELO/Power-based pairings and timeout management
func (e *Engine) processArenaMatchmaking(ctx context.Context, tx *sql.Tx) error {
	brackets := []string{"1v1", "2v2", "3v3"}

	for _, b := range brackets {
		queryQueue := `
			SELECT q.user_id, q.entered_at, u.username,
			       COALESCE((SELECT soldiers FROM workshop_inventory WHERE encampment_id = e.id), 0) as soldiers,
			       COALESCE((SELECT mechs FROM workshop_inventory WHERE encampment_id = e.id), 0) as mechs
			FROM arena_queue q
			JOIN users u ON u.telegram_id = q.user_id
			JOIN encampments e ON e.user_id = q.user_id
			WHERE q.bracket = $1
			ORDER BY q.entered_at ASC`

		rows, err := tx.QueryContext(ctx, queryQueue, b)
		if err != nil {
			return err
		}

		type queuedUser struct {
			userID      int64
			enteredAt   time.Time
			username    string
			powerRating int
		}

		var participants []queuedUser
		for rows.Next() {
			var qu queuedUser
			var soldiers, mechs int
			if err := rows.Scan(&qu.userID, &qu.enteredAt, &qu.username, &soldiers, &mechs); err == nil {
				qu.powerRating = (soldiers * 10) + (mechs * 150)
				participants = append(participants, qu)
			}
		}
		rows.Close()

		requiredMatchCount := 2
		switch b {
		case "2v2":
			requiredMatchCount = 4
		case "3v3":
			requiredMatchCount = 6
		}

		if len(participants) >= requiredMatchCount {
			matched := participants[:requiredMatchCount]

			winner := matched[0]
			loser := matched[1]

			lootWon := 100.0
			switch b {
			case "2v2":
				lootWon = 200.0
			case "3v3":
				lootWon = 400.0
			}

			_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars + $1 WHERE encampment_id = (SELECT id FROM encampments WHERE user_id = $2)", lootWon, winner.userID)

			queryOutcome := `
				INSERT INTO arena_battles (bracket, winner_username, loser_username, winner_loot)
				VALUES ($1, $2, $3, $4)`
			_, _ = tx.ExecContext(ctx, queryOutcome, b, winner.username, loser.username, lootWon)

			for _, user := range matched {
				_, _ = tx.ExecContext(ctx, "DELETE FROM arena_queue WHERE user_id = $1", user.userID)
			}

			winAlert := fmt.Sprintf("🏟️ ARENA REPORT: VICTORY!\n\nYou won the %s duel against @%s!\n🏆 Reward: +$%.0f Cash credited.", b, loser.username, lootWon)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", winner.userID, winAlert)

			loseAlert := fmt.Sprintf("🏟️ ARENA REPORT: DEFEAT\n\nYou lost the %s duel against @%s. Keep training commander!", b, winner.username)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", loser.userID, loseAlert)
		}

		for _, q := range participants {
			if time.Since(q.enteredAt) >= 5*time.Minute {
				_, _ = tx.ExecContext(ctx, "DELETE FROM arena_queue WHERE user_id = $1", q.userID)

				refundDollars := 50.0
				switch b {
				case "2v2":
					refundDollars = 100.0
				case "3v3":
					refundDollars = 200.0
				}

				_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars + $1 WHERE encampment_id = (SELECT id FROM encampments WHERE user_id = $2)", refundDollars, q.userID)

				alert := fmt.Sprintf("⏳ ARENA QUEUE TIMEOUT\n\nNo equivalent opponents were found for the %s queue. Your entry fee of $%.0f has been refunded.", b, refundDollars)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", q.userID, alert)
			}
		}
	}
	return nil
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
		SELECT r.id, r.attacker_id, r.defender_id, r.state, r.round_number,
		       r.attacker_rations, r.attacker_ammo, r.attacker_losses, r.defender_losses,
		       ea.name as attacker_name, ea.user_id as attacker_user_id,
		       COALESCE(ed.name, 'Rogue Drone Nest') as defender_name, 
		       COALESCE(ed.user_id, 0) as defender_user_id, r.resolve_time, r.stolen_scrap
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		LEFT JOIN encampments ed ON ed.id = r.defender_id
		WHERE (r.state = 'marching' OR r.state = 'engaged' OR r.state = 'returning') AND r.resolve_time <= CURRENT_TIMESTAMP`

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed querying combat raids: %w", err)
	}
	defer rows.Close()

	type activeRaid struct {
		id              string
		attackerID      string
		defenderID      sql.NullString
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
		stolenScrap     float64
	}

	var raids []activeRaid
	for rows.Next() {
		var r activeRaid
		err := rows.Scan(
			&r.id, &r.attackerID, &r.defenderID, &r.state, &r.roundNumber,
			&r.attackerRations, &r.attackerAmmo, &r.attackerLosses, &r.defenderLosses,
			&r.attackerName, &r.attackerUserID,
			&r.defenderName, &r.defenderUserID, &r.resolveTime, &r.stolenScrap,
		)
		if err == nil {
			raids = append(raids, r)
		} else {
			log.Printf("Error scanning combat raid: %v", err)
		}
	}
	rows.Close()

	for _, r := range raids {
		// --- RETURN MARCH INTRANSIT LOOT & TROOP SETTLEMENT (Phase 4 Addition) ---
		if r.state == "returning" {
			// Query survivors from raid_forces to return safely to active hangar inventories
			var soldiersMob, mechsMob int
			_ = tx.QueryRowContext(ctx, "SELECT soldiers_mobilized, mechs_mobilized FROM raid_forces WHERE raid_id = $1", r.id).Scan(&soldiersMob, &mechsMob)

			// Credit primary attacker survivors back to hangar inventory
			_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2 WHERE encampment_id = $3", soldiersMob, mechsMob, r.attackerID)

			// Credit primary attacker the in-transit looted scrap assets
			_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap + $1 WHERE encampment_id = $2", r.stolenScrap, r.attackerID)

			// Set campaign to completed
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'completed' WHERE id = $1", r.id)

			alertMsg := fmt.Sprintf("🚀 RETURN MARCH COMPLETED: Your expedition survivors returned to base safely carrying +%.1f Scrap!", r.stolenScrap)
			queryNotif := "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)"
			_, _ = tx.ExecContext(ctx, queryNotif, r.attackerUserID, alertMsg)
			continue
		}

		if r.state == "marching" {
			battleDuration := 20 * time.Minute
			if !r.defenderID.Valid {
				battleDuration = 15 * time.Minute
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

		var primarySoldians, primaryDrones, primaryJets, primaryMechs int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers, 0), COALESCE(drones, 0), COALESCE(jets, 0), COALESCE(mechs, 0) FROM workshop_inventory WHERE encampment_id = $1", r.attackerID).Scan(&primarySoldians, &primaryDrones, &primaryJets, &primaryMechs)

		type coopContributor struct {
			encampment_id string
			soldiers      int
			mechs         int
		}

		var helpers []coopContributor
		queryHelpers := "SELECT encampment_id, soldiers_contributed, mechs_contributed FROM raid_coop_members WHERE raid_id = $1"
		rowH, errH := tx.QueryContext(ctx, queryHelpers, r.id)
		if errH == nil {
			for rowH.Next() {
				var h coopContributor
				if err := rowH.Scan(&h.encampment_id, &h.soldiers, &h.mechs); err == nil {
					helpers = append(helpers, h)
				}
			}
			rowH.Close()
		}

		totSoldiers := primarySoldians
		totMechs := primaryMechs
		for _, h := range helpers {
			totSoldiers += h.soldiers
			totMechs += h.mechs
		}

		attackForce := totSoldiers + primaryDrones + primaryJets + totMechs

		var attackerTanks int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(fusion_tanks, 0) FROM workshop_inventory WHERE encampment_id = $1", r.attackerID).Scan(&attackerTanks)

		var defenseForce int
		var defLevel int = 1
		var defenderShields int = 0
		var defenderAgentActive bool = false
		var targetBiome string = "wasteland"

		if !r.defenderID.Valid {
			var attackerCoreLvl int
			_ = tx.QueryRowContext(ctx, "SELECT level FROM encampments WHERE id = $1", r.attackerID).Scan(&attackerCoreLvl)
			if attackerCoreLvl <= 0 {
				attackerCoreLvl = 1
			}
			defenseForce = attackerCoreLvl * 18
			defLevel = attackerCoreLvl
		} else {
			var soldiersDefender, dronesDefender, jetsDefender, mechsDefender int
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers, 0), COALESCE(drones, 0), COALESCE(jets, 0), COALESCE(mechs, 0) FROM workshop_inventory WHERE encampment_id = $1", r.defenderID.String).Scan(&soldiersDefender, &dronesDefender, &jetsDefender, &mechsDefender)
			defenseForce = soldiersDefender + dronesDefender + jetsDefender + mechsDefender

			_ = tx.QueryRowContext(ctx, "SELECT level FROM modules WHERE encampment_id = $1 AND type = 'tent'", r.defenderID.String).Scan(&defLevel)
			if defLevel == 0 {
				defLevel = 1
			}

			_ = tx.QueryRowContext(ctx, "SELECT COALESCE((SELECT nuclear_shields FROM workshop_inventory WHERE encampment_id = $1), 0)", r.defenderID.String).Scan(&defenderShields)
			_ = tx.QueryRowContext(ctx, "SELECT is_active FROM agent_tasks WHERE user_id = $1", r.defenderUserID).Scan(&defenderAgentActive)
			_ = tx.QueryRowContext(ctx, "SELECT c.biome FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", r.defenderID.String).Scan(&targetBiome)
		}

		var activeWeather string
		_ = tx.QueryRowContext(ctx, "SELECT active_weather FROM world_state WHERE id = 1").Scan(&activeWeather)

		offenseRatingModifier := 1.0
		defenseRatingModifier := 1.0 + (float64(defLevel) * 0.15)

		switch targetBiome {
		case "forest":
			offenseRatingModifier *= 0.85
			defenseRatingModifier *= 1.15
		case "ruins":
			offenseRatingModifier *= 0.75
			defenseRatingModifier *= 1.30
		}

		switch activeWeather {
		case "radiation_storm":
			offenseRatingModifier *= 0.75
		case "acid_rain":
			totMechs = int(float64(totMechs) * 0.50)
		}

		if r.attackerRations <= 0 || r.attackerAmmo <= 0 {
			offenseRatingModifier *= 0.50
		}

		attackerOffenseRating := (float64(attackForce) * 15.0 * offenseRatingModifier) * (1.0 + (float64(attackerTanks) * 0.50) + (float64(totMechs) * 1.50))
		defenderDefenseRating := float64(defenseForce) * 10.0 * defenseRatingModifier

		attackerCasualties := 0
		defenderCasualties := 0

		isVictory := attackerOffenseRating > defenderDefenseRating

		if isVictory {
			defenderCasualties = defenseForce
			attackerCasualties = attackForce / 2
		} else {
			attackerCasualties = attackForce
			defenderCasualties = defenseForce / 3
		}

		var primSurvSoldiers, primSurvMechs int

		if attackerCasualties > 0 {
			primRatio := float64(primarySoldians+primaryMechs) / float64(attackForce)
			if math.IsNaN(primRatio) {
				primRatio = 1.0
			}
			primCas := int(float64(attackerCasualties) * primRatio)

			// Calculate surviving forces for primary attacker (will return during the returning state resolve pass)
			primSurvSoldiers = primarySoldians - (primCas / 2)
			primSurvMechs = primaryMechs - (primCas / 2)
			if primSurvSoldiers < 0 {
				primSurvSoldiers = 0
			}
			if primSurvMechs < 0 {
				primSurvMechs = 0
			}

			// Immediately deduct casualties from active workshop inventories of primary attacker
			_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = GREATEST(soldiers - $1, 0), mechs = GREATEST(mechs - $2, 0) WHERE encampment_id = $3", primCas/2, primCas/2, r.attackerID)

			for _, h := range helpers {
				hForce := h.soldiers + h.mechs
				hRatio := float64(hForce) / float64(attackForce)
				hCas := int(float64(attackerCasualties) * hRatio)

				casSoldiers := hCas / 2
				casMechs := hCas / 2

				refundSoldiers := h.soldiers - casSoldiers
				refundMechs := h.mechs - casMechs
				if refundSoldiers < 0 {
					refundSoldiers = 0
				}
				if refundMechs < 0 {
					refundMechs = 0
				}

				// Survivors from co-op helpers immediately march back home safely
				_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2 WHERE encampment_id = $3", refundSoldiers, refundMechs, h.encampment_id)
			}
		} else {
			primSurvSoldiers = primarySoldians
			primSurvMechs = primaryMechs
		}

		if r.defenderID.Valid && defenderCasualties > 0 {
			_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = GREATEST(soldiers - $1, 0) WHERE encampment_id = $2", defenderCasualties, r.defenderID.String)
		}

		var defenderScrap float64
		if r.defenderID.Valid {
			_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", r.defenderID.String).Scan(&defenderScrap)
		} else {
			defenderScrap = 125.0
		}

		lootPercentage := 0.40
		if defenderShields > 0 {
			lootPercentage = 0.20
		}

		stolenScrap := defenderScrap * lootPercentage

		// If victory, save primary loot and transition state to returning march (Phase 4 Return Journeys)
		if isVictory {
			primRatio := float64(primarySoldians+primaryMechs) / float64(attackForce)
			if math.IsNaN(primRatio) {
				primRatio = 1.0
			}
			primaryShare := stolenScrap * primRatio

			// Save primary attacker survivors to the active campaigns raid_forces to return safely
			_, _ = tx.ExecContext(ctx, "INSERT INTO raid_forces (raid_id, soldiers_mobilized, mechs_mobilized, route_type) VALUES ($1, $2, $3, 'direct') ON CONFLICT (raid_id) DO UPDATE SET soldiers_mobilized = $2, mechs_mobilized = $3", r.id, primSurvSoldiers, primSurvMechs)

			// Store the primary attacker's in-transit looted scrap inside the raids table (Fixes Instant Teleporting)
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'returning', stolen_scrap = $1, resolve_time = CURRENT_TIMESTAMP + INTERVAL '15 minutes' WHERE id = $2", primaryShare, r.id)

			for _, h := range helpers {
				hForce := h.soldiers + h.mechs
				hRatio := float64(hForce) / float64(attackForce)
				helperShare := stolenScrap * hRatio

				// Helper loot returns back to helper warehouses instantly (saves network payload size)
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap + $1 WHERE encampment_id = $2", helperShare, h.encampment_id)

				var helperUserID int64
				_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", h.encampment_id).Scan(&helperUserID)
				helperAlert := fmt.Sprintf("🤝 CO-OP RAID RESOLUTION: VICTORY!\n\nYour forces successfully assisted in breaching the defenses of [%s]!\nProportional Loot Share Earned: +%.1f Scrap.", r.defenderName, helperShare)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", helperUserID, helperAlert)
			}
		} else {
			// Defeat: No returns, resolve immediately
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'completed' WHERE id = $1", r.id)
		}

		if r.defenderID.Valid {
			_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = GREATEST(scrap - $1, 0) WHERE encampment_id = $2", stolenScrap, r.defenderID.String)
		}

		attackerAlert := fmt.Sprintf(
			"⚔️ RAID REPORT: VICTORY!\n\n"+
				"Target: %s\n"+
				"Your raiders breached the base defense grid.\n"+
				"⚙️ Looted: %.1f Scrap\n"+
				"💀 Casualties Sustained: %d units\n\n"+
				"🚀 RETURN MARCH ENGAGED: Your survivors are marching back home with the loot (15m travel time). Check Expedition Radar for statuses.",
			r.defenderName, stolenScrap, attackerCasualties,
		)
		if !isVictory {
			attackerAlert = fmt.Sprintf(
				"❌ RAID REPORT: DEFEAT!\n\n"+
					"Target: %s\n"+
					"Your forces were repelled. March failed.\n"+
					"💀 Casualties Sustained: All %d units lost.",
				r.defenderName, attackerCasualties,
			)
		}

		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.attackerUserID, attackerAlert)

		if r.defenderID.Valid {
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
	}

	return nil
}