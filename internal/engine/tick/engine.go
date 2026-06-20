package tick

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"strings"
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

	if err := e.weatherEngine.RunWeatherPass(ctx, tx); err != nil {
		log.Printf("Error during Tick Weather Pass: %v", err)
		return
	}

	if err := e.resourceProcessor.RunResourcePass(ctx, tx); err != nil {
		log.Printf("Error during Tick Resource Pass execution: %v", err)
		return
	}

	if err := e.applyActiveLogisticsConsumption(ctx, tx); err != nil {
		log.Printf("Error during Active Logistics Consumption: %v", err)
		return
	}

	if err := e.processArenaMatchmaking(ctx, tx); err != nil {
		log.Printf("Error during Arena Matchmaker sweep: %v", err)
		return
	}

	if err := e.resolvePendingEspionageMissions(ctx, tx); err != nil {
		log.Printf("Error during Espionage resolution: %v", err)
		return
	}

	if err := e.resolveCompletedMiningQueues(ctx, tx); err != nil {
		log.Printf("Error during Active Mining Resolution: %v", err)
		return
	}

	if err := e.starvationEngine.RunStarvationPass(ctx, tx); err != nil {
		log.Printf("Error during Tick Starvation Pass execution: %v", err)
		return
	}

	if err := e.resolveCompletedUpgrades(ctx, tx); err != nil {
		log.Printf("Error during Construction Upgrade Pass execution: %v", err)
		return
	}

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
		var targetLvl int = 1
		_ = tx.QueryRowContext(ctx, "SELECT name, level FROM encampments WHERE id = $1", m.targetID).Scan(&targetName, &targetLvl)

		var tentLvl, heapLvl, genLvl int = 1, 1, 1
		queryMod := "SELECT level FROM modules WHERE encampment_id = $1 AND type = $2"
		if err := tx.QueryRowContext(ctx, queryMod, m.targetID, "tent").Scan(&tentLvl); err != nil {
			tentLvl = 1
		}
		if err := tx.QueryRowContext(ctx, queryMod, m.targetID, "scrap_heap").Scan(&heapLvl); err != nil {
			heapLvl = 1
		}
		if err := tx.QueryRowContext(ctx, queryMod, m.targetID, "generator").Scan(&genLvl); err != nil {
			genLvl = 1
		}

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
			delayedResolve := ex.resolveTime.UTC().Add(3 * time.Minute)
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET resolve_time = $1 WHERE id = $2", delayedResolve, ex.id)
		}

		if newRations <= 0 {
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET attacker_rations = 0.0 WHERE id = $1", ex.id)
		}
	}
	return nil
}

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

			winners := matched[:requiredMatchCount/2]
			losers := matched[requiredMatchCount/2:]

			lootWon := 100.0
			switch b {
			case "2v2":
				lootWon = 200.0
			case "3v3":
				lootWon = 400.0
			}

			for _, w := range winners {
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars + $1 WHERE encampment_id = (SELECT id FROM encampments WHERE user_id = $2)", lootWon, w.userID)

				winAlert := fmt.Sprintf("🏟️ ARENA REPORT: TEAM VICTORY!\n\nYou won the %s team clash!\n🏆 Reward: +$%.0f Cash credited.", b, lootWon)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", w.userID, winAlert)
			}

			for _, l := range losers {
				loseAlert := fmt.Sprintf("🏟️ ARENA REPORT: TEAM DEFEAT\n\nYou lost the %s team clash. Keep training commander!", b)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", l.userID, loseAlert)
			}

			var winnerNames []string
			for _, w := range winners {
				winnerNames = append(winnerNames, "@"+w.username)
			}
			var loserNames []string
			for _, l := range losers {
				loserNames = append(loserNames, "@"+l.username)
			}

			queryOutcome := `
				INSERT INTO arena_battles (bracket, winner_username, loser_username, winner_loot)
				VALUES ($1, $2, $3, $4)`
			_, _ = tx.ExecContext(ctx, queryOutcome, b, strings.Join(winnerNames, ", "), strings.Join(loserNames, ", "), lootWon)

			for _, user := range matched {
				_, _ = tx.ExecContext(ctx, "DELETE FROM arena_queue WHERE user_id = $1", user.userID)
			}
		}

		for _, q := range participants {
			if time.Since(q.enteredAt.UTC()) >= 5*time.Minute {
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
		if r.state == "returning" {
			var soldiersMob, mechsMob int
			_ = tx.QueryRowContext(ctx, "SELECT soldiers_mobilized, mechs_mobilized FROM raid_forces WHERE raid_id = $1", r.id).Scan(&soldiersMob, &mechsMob)

			_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2 WHERE encampment_id = $3", soldiersMob, mechsMob, r.attackerID)

			_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap + $1 WHERE encampment_id = $2", r.stolenScrap, r.attackerID)

			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'completed' WHERE id = $1", r.id)

			alertMsg := fmt.Sprintf("🚀 RETURN MARCH COMPLETED: Your expedition survivors returned to base safely carrying +%.1f Scrap!", r.stolenScrap)
			queryNotif := "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)"
			_, _ = tx.ExecContext(ctx, queryNotif, r.attackerUserID, alertMsg)
			continue
		}

		if r.state == "marching" {
			var routeType string
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(route_type, 'direct') FROM raid_forces WHERE raid_id = $1", r.id).Scan(&routeType)

			if routeType != "stealth" && r.defenderID.Valid {
				timeLeft := int(time.Until(r.resolveTime.UTC()).Seconds())
				if timeLeft > 0 {
					proximityAlert := fmt.Sprintf(
						"🛰️ RADAR WARNING: incoming offensive force is approaching coordinate perimeter!\n"+
							"Target: Your base [%s]\n"+
							"Threat Distance Status: In Transit (%ds remaining until boundary breach).",
						r.defenderName, timeLeft,
					)
					_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.defenderUserID, proximityAlert)
				}
			}

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

		var primarySoldiers, primaryMechs, primaryBuggies int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers_mobilized, 0), COALESCE(mechs_mobilized, 0), COALESCE(buggies_mobilized, 0) FROM raid_forces WHERE raid_id = $1", r.id).Scan(&primarySoldiers, &primaryMechs, &primaryBuggies)

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

		totSoldiers := primarySoldiers
		totMechs := primaryMechs
		for _, h := range helpers {
			totSoldiers += h.soldiers
			totMechs += h.mechs
		}

		var attackerTanks int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(fusion_tanks, 0) FROM workshop_inventory WHERE encampment_id = $1", r.attackerID).Scan(&attackerTanks)

		var attackerMilitaryTechLvl int = 1
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(military_tech_lvl, 1) FROM research_states WHERE encampment_id = $1", r.attackerID).Scan(&attackerMilitaryTechLvl)

		var attackerBioLvl int = 1
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(bio_lvl, 1) FROM mutation_states WHERE encampment_id = $1", r.attackerID).Scan(&attackerBioLvl)

		var defLevel int = 1
		var defenderShields int = 0
		var defenderAgentActive bool = false
		var targetBiome string = "wasteland"
		var defenderBioLvl int = 1
		var soldiersDefender, dronesDefender, jetsDefender, mechsDefender int

		if !r.defenderID.Valid {
			var attackerCoreLvl int
			_ = tx.QueryRowContext(ctx, "SELECT level FROM encampments WHERE id = $1", r.attackerID).Scan(&attackerCoreLvl)
			if attackerCoreLvl <= 0 {
				attackerCoreLvl = 1
			}
			soldiersDefender = attackerCoreLvl * 18
			defLevel = attackerCoreLvl
		} else {
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers, 0), COALESCE(drones, 0), COALESCE(jets, 0), COALESCE(mechs, 0) FROM workshop_inventory WHERE encampment_id = $1", r.defenderID.String).Scan(&soldiersDefender, &dronesDefender, &jetsDefender, &mechsDefender)

			_ = tx.QueryRowContext(ctx, "SELECT level FROM modules WHERE encampment_id = $1 AND type = 'tent'", r.defenderID.String).Scan(&defLevel)
			if defLevel == 0 {
				defLevel = 1
			}

			_ = tx.QueryRowContext(ctx, "SELECT COALESCE((SELECT nuclear_shields FROM workshop_inventory WHERE encampment_id = $1), 0)", r.defenderID.String).Scan(&defenderShields)
			_ = tx.QueryRowContext(ctx, "SELECT is_active FROM agent_tasks WHERE user_id = $1", r.defenderUserID).Scan(&defenderAgentActive)
			_ = tx.QueryRowContext(ctx, "SELECT c.biome FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", r.defenderID.String).Scan(&targetBiome)
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(bio_lvl, 1) FROM mutation_states WHERE encampment_id = $1", r.defenderID.String).Scan(&defenderBioLvl)
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

		mechOffenseMultiplier := 1.50 * (1.0 + float64(attackerMilitaryTechLvl-1)*0.25)

		round := 1
		attSols := totSoldiers
		attMechs := totMechs
		defSols := soldiersDefender
		defMechs := mechsDefender
		defDrones := dronesDefender
		defJets := jetsDefender

		var narrationLogs []string

		for round <= 5 && (attSols > 0 || attMechs > 0) && (defSols > 0 || defMechs > 0 || defDrones > 0 || defJets > 0) {
			attRating := (float64(attSols+attMechs) * 15.0 * offenseRatingModifier) * (1.0 + (float64(attackerTanks) * 0.50) + (float64(attMechs) * mechOffenseMultiplier))
			defRating := float64(defSols+defMechs+defDrones+defJets) * 10.0 * defenseRatingModifier

			attCas := 0
			defCas := 0
			if attRating > defRating {
				defCas = int(float64(defSols+defMechs+defDrones+defJets) * 0.40)
				if defCas <= 0 && (defSols+defMechs+defDrones+defJets) > 0 {
					defCas = 1
				}
				attCas = int(float64(attSols+attMechs) * 0.15)
			} else {
				attCas = int(float64(attSols+attMechs) * 0.35)
				if attCas <= 0 && (attSols+attMechs) > 0 {
					attCas = 1
				}
				defCas = int(float64(defSols+defMechs+defDrones+defJets) * 0.10)
			}

			if attCas > 0 {
				reduction := float64(attackerBioLvl-1) * 0.10
				reduction = math.Min(reduction, 0.90)
				attCas = int(float64(attCas) * (1.0 - reduction))
			}
			if defCas > 0 && r.defenderID.Valid {
				reduction := float64(defenderBioLvl-1) * 0.10
				reduction = math.Min(reduction, 0.90)
				defCas = int(float64(defCas) * (1.0 - reduction))
			}

			lostAttSols := int(float64(attCas) * 0.70)
			lostAttMechs := attCas - lostAttSols
			if lostAttSols > attSols {
				lostAttSols = attSols
			}
			if lostAttMechs > attMechs {
				lostAttMechs = attMechs
			}
			attSols -= lostAttSols
			attMechs -= lostAttMechs

			lostDefSols := int(float64(defCas) * 0.60)
			lostDefMechs := int(float64(defCas) * 0.20)
			lostDefDrones := defCas - lostDefSols - lostDefMechs
			if lostDefSols > defSols {
				lostDefSols = defSols
			}
			if lostDefMechs > defMechs {
				lostDefMechs = defMechs
			}
			if lostDefDrones > (defDrones + defJets) {
				lostDefDrones = defDrones + defJets
			}

			defSols -= lostDefSols
			defMechs -= lostDefMechs
			if lostDefDrones > defDrones {
				rem := lostDefDrones - defDrones
				defDrones = 0
				defJets -= rem
				if defJets < 0 {
					defJets = 0
				}
			} else {
				defDrones -= lostDefDrones
			}

			roundLog := fmt.Sprintf(
				"Round %d Summary:\n"+
					"💥 Attacker Casualties: %d Soldiers, %d Mechs lost.\n"+
					"🛡️ Defender Casualties: %d Soldiers, %d Mechs/Drones lost.\n"+
					"⚔️ Remaining Forces -> Attacker: %d units | Defender: %d units.\n",
				round, lostAttSols, lostAttMechs, lostDefSols, lostDefMechs,
				attSols+attMechs, defSols+defMechs+defDrones+defJets,
			)
			narrationLogs = append(narrationLogs, roundLog)
			round++
		}

		isVictory := (defSols+defMechs+defDrones+defJets) <= 0 && (attSols+attMechs) > 0
		attackerCasualties := (totSoldiers + totMechs) - (attSols + attMechs)
		defenderCasualties := (soldiersDefender + dronesDefender + jetsDefender + mechsDefender) - (defSols + defMechs + defDrones + defJets)

		var primSurvSoldiers, primSurvMechs int

		if attackerCasualties > 0 {
			primRatio := float64(primarySoldiers+primaryMechs) / float64(totSoldiers+totMechs)
			if math.IsNaN(primRatio) {
				primRatio = 1.0
			}
			primCas := int(float64(attackerCasualties) * primRatio)

			casSoldiers := primCas / 2
			casMechs := primCas / 2

			primSurvSoldiers = primarySoldiers - casSoldiers
			primSurvMechs = primaryMechs - casMechs
			if primSurvSoldiers < 0 {
				primSurvSoldiers = 0
			}
			if primSurvMechs < 0 {
				primSurvMechs = 0
			}

			for _, h := range helpers {
				hForce := h.soldiers + h.mechs
				hRatio := float64(hForce) / float64(totSoldiers+totMechs)
				hCas := int(float64(attackerCasualties) * hRatio)

				casSoldiersH := hCas / 2
				casMechsH := hCas / 2

				refundSoldiers := h.soldiers - casSoldiersH
				refundMechs := h.mechs - casMechsH
				if refundSoldiers < 0 {
					refundSoldiers = 0
				}
				if refundMechs < 0 {
					refundMechs = 0
				}

				_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2 WHERE encampment_id = $3", refundSoldiers, refundMechs, h.encampment_id)
			}
		} else {
			primSurvSoldiers = primarySoldiers
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

		if isVictory {
			primRatio := float64(primarySoldiers+primaryMechs) / float64(totSoldiers+totMechs)
			if math.IsNaN(primRatio) {
				primRatio = 1.0
			}
			primaryShare := stolenScrap * primRatio

			_, _ = tx.ExecContext(ctx, "INSERT INTO raid_forces (raid_id, soldiers_mobilized, mechs_mobilized, route_type) VALUES ($1, $2, $3, 'direct') ON CONFLICT (raid_id) DO UPDATE SET soldiers_mobilized = $2, mechs_mobilized = $3", r.id, primSurvSoldiers, primSurvMechs)

			var haulers int
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(haulers, 0) FROM workshop_inventory WHERE encampment_id = $1", r.attackerID).Scan(&haulers)

			weightFactor := primaryShare / 5000.0
			if haulers > 0 {
				weightFactor *= 0.50
			}

			returnMinutes := 15.0 * (1.0 + weightFactor)
			returnDuration := time.Duration(returnMinutes) * time.Minute
			resolveTime := time.Now().UTC().Add(returnDuration)

			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'returning', stolen_scrap = $1, resolve_time = $2 WHERE id = $3", primaryShare, resolveTime, r.id)

			for _, h := range helpers {
				hForce := h.soldiers + h.mechs
				hRatio := float64(hForce) / float64(totSoldiers+totMechs)
				helperShare := stolenScrap * hRatio

				_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap + $1 WHERE encampment_id = $2", helperShare, h.encampment_id)

				var helperUserID int64
				_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", h.encampment_id).Scan(&helperUserID)
				helperAlert := fmt.Sprintf("🤝 CO-OP RAID RESOLUTION: VICTORY!\n\nYour forces successfully assisted in breaching the defenses of [%s]!\nProportional Loot Share Earned: +%.1f Scrap.", r.defenderName, helperShare)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", helperUserID, helperAlert)
			}
		} else {
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'completed' WHERE id = $1", r.id)
		}

		if r.defenderID.Valid {
			_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = GREATEST(scrap - $1, 0) WHERE encampment_id = $2", stolenScrap, r.defenderID.String)
		}

		battleReportHeader := fmt.Sprintf(
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
				"⚔️ DETAILED BATTLE REPORT (%s vs %s)\n"+
				"━━━━━━━━━━━━━━━━━━━━━━\n\n",
			r.attackerName, r.defenderName,
		)
		battleReportNarrative := strings.Join(narrationLogs, "\n")

		attackerAlert := battleReportHeader + battleReportNarrative + fmt.Sprintf(
			"\n🏆 RESOLUTION: ATTACKER VICTORY!\n"+
				"⚙️ Proportional Loot Earned: %.1f Scrap\n"+
				"💀 Total Casualties Sustained: %d units\n\n"+
				"🚀 RETURN MARCH ENGAGED: Your survivors are marching back home with the loot. Check Expedition Radar for travel progress.",
			stolenScrap, attackerCasualties,
		)
		if !isVictory {
			attackerAlert = battleReportHeader + battleReportNarrative + fmt.Sprintf(
				"\n❌ RESOLUTION: ATTACKER DEFEATED!\n"+
					"💀 Casualties Sustained: All %d units lost.",
				attackerCasualties,
			)
		}

		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.attackerUserID, attackerAlert)

		if r.defenderID.Valid {
			defenderAlert := battleReportHeader + battleReportNarrative + fmt.Sprintf(
				"\n🚨 RESOLUTION: BASE DEFENSES BREACHED!\n"+
					"⚙️ Scrap Stolen from Warehouse: %.1f Scrap\n"+
					"💀 Defense Casualties: %d units lost.",
				stolenScrap, defenderCasualties,
			)
			if !isVictory {
				defenderAlert = battleReportHeader + battleReportNarrative + fmt.Sprintf(
					"\n🛡️ RESOLUTION: BASE SHIELDED AND SECURE!\n"+
						"💀 Enemy repelled. Defense Casualties: %d units lost.",
					defenderCasualties,
				)
			}
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.defenderUserID, defenderAlert)
		}
	}

	return nil
}
