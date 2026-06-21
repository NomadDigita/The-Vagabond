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
		SELECT q.id, q.encampment_id, q.resource_type, q.miners_assigned, e.user_id, q.ready_at
		FROM active_mining_queues q
		JOIN encampments e ON e.id = q.encampment_id
		WHERE q.is_completed = FALSE`

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
		readyAt      time.Time
	}

	var completed []completedMine
	for rows.Next() {
		var m completedMine
		if err := rows.Scan(&m.id, &m.encampmentID, &m.resType, &m.miners, &m.userID, &m.readyAt); err == nil {
			completed = append(completed, m)
		}
	}
	rows.Close()

	for _, m := range completed {
		// Timezone Neutralization: Compare timestamps natively inside Go UTC boundaries
		if m.readyAt.UTC().After(time.Now().UTC()) {
			continue
		}

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
		SELECT s.id, s.spy_id, s.target_id, s.is_intercepted, ea.user_id as spy_user_id, s.resolve_time
		FROM spy_missions s
		JOIN encampments ea ON ea.id = s.spy_id
		WHERE s.resolved = FALSE`

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
		resolveTime   time.Time
	}

	var missions []espionageMission
	for rows.Next() {
		var m espionageMission
		if err := rows.Scan(&m.id, &m.spyID, &m.targetID, &m.isIntercepted, &m.spyUserID, &m.resolveTime); err == nil {
			missions = append(missions, m)
		}
	}
	rows.Close()

	for _, m := range missions {
		// Timezone Neutralization: Compare timestamps natively inside Go UTC boundaries
		if m.resolveTime.UTC().After(time.Now().UTC()) {
			continue
		}

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
	queryAllMarching := `
		SELECT r.id, r.resolve_time, ea.name, ed.user_id, ed.name, rf.route_type
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		JOIN encampments ed ON ed.id = r.defender_id
		JOIN raid_forces rf ON rf.raid_id = r.id
		WHERE r.state = 'marching'`

	rowsMarch, errMarch := tx.QueryContext(ctx, queryAllMarching)
	if errMarch == nil {
		defer rowsMarch.Close()
		for rowsMarch.Next() {
			var rID string
			var resTime time.Time
			var attName, defName, routeType string
			var defUserID int64
			if err := rowsMarch.Scan(&rID, &resTime, &attName, &defUserID, &defName, &routeType); err == nil {
				// Timezone Neutralization: Compute remaining steps in Go
				if routeType != "stealth" && resTime.UTC().After(time.Now().UTC()) {
					timeLeft := int(time.Until(resTime.UTC()).Seconds())
					if timeLeft > 0 {
						proximityAlert := fmt.Sprintf(
							"🛰️ RADAR WARNING: An offensive fleet is approaching your coordinate perimeter!\n"+
								"Hostile Force: Outpost [%s]\n"+
								"Threat Distance Status: %ds remaining until direct impact.",
							attName, timeLeft,
						)
						_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", defUserID, proximityAlert)
					}
				}
			}
		}
		rowsMarch.Close()
	}

	query := `
		SELECT r.id, r.attacker_id, r.defender_id, r.state, r.round_number,
		       ea.name as attacker_name, ea.user_id as attacker_user_id,
		       COALESCE(ed.name, 'Rogue Drone Nest') as defender_name, 
		       COALESCE(ed.user_id, 0) as defender_user_id, r.resolve_time, r.stolen_scrap
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		LEFT JOIN encampments ed ON ed.id = r.defender_id
		WHERE r.state = 'marching' OR r.state = 'engaged' OR r.state = 'returning'`

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed querying combat raids: %w", err)
	}
	defer rows.Close()

	type activeRaid struct {
		id             string
		attackerID     string
		defenderID     sql.NullString
		state          string
		roundNumber    int
		attackerName   string
		attackerUserID int64
		defenderName   string
		defenderUserID int64
		resolveTime    time.Time
		stolenScrap    float64
	}

	var raids []activeRaid
	for rows.Next() {
		var r activeRaid
		err := rows.Scan(
			&r.id, &r.attackerID, &r.defenderID, &r.state, &r.roundNumber,
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
		// Timezone Neutralization: Check resolve timestamps in Go
		if r.resolveTime.UTC().After(time.Now().UTC()) {
			continue
		}

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
			nextRoundResolve := time.Now().UTC().Add(e.TickInterval)
			updateMarch := `
				UPDATE raids 
				SET state = 'engaged', resolve_time = $1, round_number = 1 
				WHERE id = $2`
			_, _ = tx.ExecContext(ctx, updateMarch, nextRoundResolve, r.id)

			arrivalAlert := fmt.Sprintf(
				"⚔️ CAMPAIGN ENGAGEMENT ACTIVATED!\n\n"+
					"Your forces have arrived at Outpost [%s].\n"+
					"Deployments are actively engaged in battlefield skirmishes.\n"+
					"Decisive Resolution progress starting next tick cycle.",
				r.defenderName,
			)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.attackerUserID, arrivalAlert)
			continue
		}

		// PROCESS COMBAT ROUND (Tick-by-Tick Combat Rounds Engine)
		var primarySoldiers, primaryMechs, primaryBuggies int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers_mobilized, 0), COALESCE(mechs_mobilized, 0), COALESCE(buggies_mobilized, 0) FROM raid_forces WHERE raid_id = $1 FOR UPDATE", r.id).Scan(&primarySoldiers, &primaryMechs, &primaryBuggies)

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

		attackForce := totSoldiers + totMechs

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
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers, 0), COALESCE(drones, 0), COALESCE(jets, 0), COALESCE(mechs, 0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", r.defenderID.String).Scan(&soldiersDefender, &dronesDefender, &jetsDefender, &mechsDefender)

			_ = tx.QueryRowContext(ctx, "SELECT level FROM modules WHERE encampment_id = $1 AND type = 'tent'", r.defenderID.String).Scan(&defLevel)
			if defLevel == 0 {
				defLevel = 1
			}

			_ = tx.QueryRowContext(ctx, "SELECT COALESCE((SELECT nuclear_shields FROM workshop_inventory WHERE encampment_id = $1), 0)", r.defenderID.String).Scan(&defenderShields)
			_ = tx.QueryRowContext(ctx, "SELECT is_active FROM agent_tasks WHERE user_id = $1", r.defenderUserID).Scan(&defenderAgentActive)
			_ = tx.QueryRowContext(ctx, "SELECT c.biome FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", r.defenderID.String).Scan(&targetBiome)
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(bio_lvl, 1) FROM mutation_states WHERE encampment_id = $1", r.defenderID.String).Scan(&defenderBioLvl)
		}

		defenseForce := soldiersDefender + dronesDefender + jetsDefender + mechsDefender

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

		// Read and apply ratios safely
		var rations, ammo float64
		_ = tx.QueryRowContext(ctx, "SELECT attacker_rations, attacker_ammo FROM raids WHERE id = $1", r.id).Scan(&rations, &ammo)

		if rations <= 0 || ammo <= 0 {
			offenseRatingModifier *= 0.50
		}

		mechOffenseMultiplier := 1.50 * (1.0 + float64(attackerMilitaryTechLvl-1)*0.25)

		attRating := (float64(attackForce) * 15.0 * offenseRatingModifier) * (1.0 + (float64(attackerTanks) * 0.50) + (float64(totMechs) * mechOffenseMultiplier))
		defRating := float64(defenseForce) * 10.0 * defenseRatingModifier

		attCas := 0
		defCas := 0

		if attRating > defRating {
			defCas = int(float64(defenseForce) * 0.35)
			if defCas <= 0 && defenseForce > 0 {
				defCas = 1
			}
			attCas = int(float64(attackForce) * 0.12)
		} else {
			attCas = int(float64(attackForce) * 0.30)
			if attCas <= 0 && attackForce > 0 {
				attCas = 1
			}
			defCas = int(float64(defenseForce) * 0.10)
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
		if lostAttSols > primarySoldiers {
			lostAttSols = primarySoldiers
		}
		if lostAttMechs > primaryMechs {
			lostAttMechs = primaryMechs
		}

		newAttSols := primarySoldiers - lostAttSols
		newAttMechs := primaryMechs - lostAttMechs

		_, _ = tx.ExecContext(ctx, "UPDATE raid_forces SET soldiers_mobilized = $1, mechs_mobilized = $2 WHERE raid_id = $3", newAttSols, newAttMechs, r.id)

		attackerCasualties := (totSoldiers + totMechs) - (newAttSols + newAttMechs)

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

			_, _ = tx.ExecContext(ctx, "UPDATE raid_coop_members SET soldiers_contributed = $1, mechs_contributed = $2 WHERE raid_id = $3 AND encampment_id = $4", refundSoldiers, refundMechs, r.id, h.encampment_id)
		}

		if r.defenderID.Valid && defCas > 0 {
			lostDefSols := int(float64(defCas) * 0.60)
			lostDefMechs := int(float64(defCas) * 0.20)
			if lostDefSols > soldiersDefender {
				lostDefSols = soldiersDefender
			}
			if lostDefMechs > mechsDefender {
				lostDefMechs = mechsDefender
			}
			_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = GREATEST(soldiers - $1, 0), mechs = GREATEST(mechs - $2, 0) WHERE encampment_id = $3", lostDefSols, lostDefMechs, r.defenderID.String)
		}

		attackerStillStanding := (newAttSols + newAttMechs) > 0
		defenderStillStanding := (defenseForce - defCas) > 0

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

		roundLog := fmt.Sprintf(
			"⚔️ BATTLE REPORT: ROUND %d COMPLETE!\n\n"+
				"💥 Attacker Casualties: %d Soldiers, %d Mechs lost.\n"+
				"🛡️ Defender Casualties: %d units lost.\n"+
				"⏳ Next skirmish round starting on next clock tick.",
			r.roundNumber, lostAttSols, lostAttMechs, defCas,
		)

		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.attackerUserID, roundLog)
		if r.defenderID.Valid {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.defenderUserID, roundLog)
		}

		if !attackerStillStanding {
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'completed' WHERE id = $1", r.id)
			defeatAlert := fmt.Sprintf("❌ BATTLE RESOLUTION: DEFEAT!\n\nYour forces were entirely repelled at Outpost [%s]. All deployed raiders were lost.", r.defenderName)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.attackerUserID, defeatAlert)

			if r.defenderID.Valid {
				winAlert := fmt.Sprintf("🛡️ BATTLE RESOLUTION: BASE DEFENSED!\n\nHostile forces marching from [%s] were repelled.", r.attackerName)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.defenderUserID, winAlert)
			}
		} else if !defenderStillStanding {
			primRatio := float64(primarySoldiers+primaryMechs) / float64(totSoldiers+totMechs)
			if math.IsNaN(primRatio) {
				primRatio = 1.0
			}
			primaryShare := stolenScrap * primRatio

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

			if r.defenderID.Valid {
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = GREATEST(scrap - $1, 0) WHERE encampment_id = $2", stolenScrap, r.defenderID.String)
			}

			winAlert := fmt.Sprintf(
				"🏆 BATTLE RESOLUTION: VICTORY!\n\n"+
					"Your forces breached the coordinate perimeters of [%s]!\n"+
					"⚙️ Looted: +%.1f Scrap\n\n"+
					"🚀 RETURN MARCH ENGAGED: Your survivors are marching back home with the loot.",
				r.defenderName, primaryShare,
			)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.attackerUserID, winAlert)

			if r.defenderID.Valid {
				loseAlert := fmt.Sprintf(
					"🚨 BATTLE RESOLUTION: BASE BREACHED!\n\n"+
						"Our perimeters were breached by [%s]!\n"+
						"⚙️ Looted: -%.1f Scrap stolen from warehouses.",
					r.attackerName, stolenScrap,
				)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.defenderUserID, loseAlert)
			}

			// Concluding pass co-op helper refunds
			for _, h := range helpers {
				_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2 WHERE encampment_id = $3", h.soldiers, h.mechs, h.encampment_id)
			}
		} else if r.roundNumber >= 5 {
			returnMinutes := 15.0
			resolveTime := time.Now().UTC().Add(time.Duration(returnMinutes) * time.Minute)

			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'returning', stolen_scrap = 0, resolve_time = $1 WHERE id = $2", resolveTime, r.id)

			drawAlert := fmt.Sprintf("⚔️ BATTLE TIMEOUT: RETREAT ENGAGED!\n\nNo decisive victory was achieved after 5 rounds. Your remaining forces have retreated and are returning home.")
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.attackerUserID, drawAlert)

			if r.defenderID.Valid {
				defDrawAlert := fmt.Sprintf("🛡️ BATTLE TIMEOUT: SHIELD HELD!\n\nDefenses held for 5 rounds. Hostile raiders retreated.")
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.defenderUserID, defDrawAlert)
			}

			// Concluding pass co-op helper refunds on Timeout Draw
			for _, h := range helpers {
				_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2 WHERE encampment_id = $3", h.soldiers, h.mechs, h.encampment_id)
			}
		} else {
			nextRoundResolve := time.Now().UTC().Add(e.TickInterval)
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET round_number = round_number + 1, resolve_time = $1 WHERE id = $2", nextRoundResolve, r.id)
		}
	}

	return nil
}