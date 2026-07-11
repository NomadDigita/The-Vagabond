package tick

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/engine/agent"
	"github.com/NomadDigita/The-Vagabond/internal/engine/resource"
	"github.com/NomadDigita/The-Vagabond/internal/engine/starvation"
	"github.com/NomadDigita/The-Vagabond/internal/engine/world"
	"github.com/NomadDigita/The-Vagabond/internal/game/battlereport"
	"github.com/NomadDigita/The-Vagabond/internal/game/content"
	"github.com/NomadDigita/The-Vagabond/internal/game/scoring"
)

type Engine struct {
	DB                *sql.DB
	TickInterval      time.Duration
	stopChan          chan struct{}
	resourceProcessor *resource.Processor
	starvationEngine  *starvation.Engine
	weatherEngine     *world.WeatherEngine
	agentProcessor    *agent.Processor
	lastIdleCheck     time.Time
	lastAutoScan      time.Time
}

func NewEngine(db *sql.DB, interval time.Duration) *Engine {
	return &Engine{
		DB:                db,
		TickInterval:      interval,
		stopChan:          make(chan struct{}),
		resourceProcessor: resource.NewProcessor(db),
		starvationEngine:  starvation.NewEngine(db),
		weatherEngine:     world.NewWeatherEngine(db),
		agentProcessor:    agent.NewProcessor(db),
		lastIdleCheck:     time.Now().UTC(),
		lastAutoScan:      time.Now().UTC(),
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

// tickPhase describes one isolated unit of tick work. Each phase gets its
// own transaction so that a failure in one subsystem (e.g. arena matchmaking)
// can never block or roll back unrelated subsystems (e.g. espionage,
// mining, combat resolution) in the same tick.
type tickPhase struct {
	name string
	run  func(ctx context.Context, tx *sql.Tx) error
}

// runPhase executes a single phase in its own transaction. It always
// returns (never aborts the caller's loop) so subsequent phases still run
// even if this one fails. Errors are logged with the phase name attached
// for easier diagnosis.
func (e *Engine) runPhase(ctx context.Context, p tickPhase) {
	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("Tick phase [%s] failed to open transaction: %v", p.name, err)
		return
	}
	defer tx.Rollback()

	if err := p.run(ctx, tx); err != nil {
		log.Printf("Tick phase [%s] failed: %v", p.name, err)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Tick phase [%s] failed to commit: %v", p.name, err)
	}
}

// collectDailyTax implements SpaceHunt's Daily Tax Law: once every 24
// hours, every player pays tax_rate_percent of their banked Dollars into a
// shared pool, which is then paid out evenly to the top 3 ranked
// survivors (by the canonical scoring.ScoreExpr).
func (e *Engine) collectDailyTax(ctx context.Context, tx *sql.Tx) error {
	var lastCollected time.Time
	var taxRate int
	err := tx.QueryRowContext(ctx, "SELECT tax_rate_percent, last_collected_at FROM tax_law WHERE id = 1 FOR UPDATE").Scan(&taxRate, &lastCollected)
	if err != nil {
		return err
	}

	if time.Now().UTC().Sub(lastCollected) < 24*time.Hour {
		return nil // Not due yet.
	}

	if taxRate > 0 {
		var totalCollected float64

		payerRows, err := tx.QueryContext(ctx, "SELECT encampment_id, dollars FROM resources WHERE dollars > 0")
		if err != nil {
			return err
		}
		type payer struct {
			campID  string
			dollars float64
		}
		var payers []payer
		for payerRows.Next() {
			var p payer
			if scanErr := payerRows.Scan(&p.campID, &p.dollars); scanErr == nil {
				payers = append(payers, p)
			}
		}
		payerRows.Close()

		for _, p := range payers {
			collected := p.dollars * float64(taxRate) / 100.0
			totalCollected += collected
			_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars - $1 WHERE encampment_id = $2", collected, p.campID)
		}

		if totalCollected > 0 {
			topQuery := fmt.Sprintf(`
				SELECT e.user_id, e.id, e.name
				FROM encampments e
				ORDER BY %s DESC
				LIMIT 3`, scoring.ScoreExpr)

			topRows, err := tx.QueryContext(ctx, topQuery)
			if err == nil {
				type winner struct {
					userID int64
					campID string
					name   string
				}
				var winners []winner
				for topRows.Next() {
					var w winner
					if scanErr := topRows.Scan(&w.userID, &w.campID, &w.name); scanErr == nil {
						winners = append(winners, w)
					}
				}
				topRows.Close()

				if len(winners) > 0 {
					share := totalCollected / float64(len(winners))
					for _, w := range winners {
						_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars + $1 WHERE encampment_id = $2", share, w.campID)
						alertMsg := fmt.Sprintf(
							"💰🏆 DAILY TAX PAYOUT! 🏆💰\n\nAs a Top-3 ranked survivor, you received 💵 $%.2f from the Wasteland Tax Law (%d%% rate) collected from all players!",
							share, taxRate,
						)
						_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", w.userID, alertMsg)
					}
				}
			}
		}
	}

	_, err = tx.ExecContext(ctx, "UPDATE tax_law SET last_collected_at = CURRENT_TIMESTAMP WHERE id = 1")
	return err
}

// autoScanSweep implements SpaceHunt's "Automatic Scan" job: for every
// encampment with auto_scan_enabled, pick one random rival outpost and
// send a lightweight scan report - automating what /scout does manually.
// Runs on its own ~30-minute cadence via the caller, same pattern as the
// idle miner sweep.
func (e *Engine) autoScanSweep(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT e.id, e.user_id
		FROM encampments e
		WHERE e.auto_scan_enabled = TRUE`)
	if err != nil {
		return err
	}

	type scanner struct {
		campID string
		userID int64
	}
	var scanners []scanner
	for rows.Next() {
		var s scanner
		if scanErr := rows.Scan(&s.campID, &s.userID); scanErr == nil {
			scanners = append(scanners, s)
		}
	}
	rows.Close()

	for _, s := range scanners {
		var targetName, targetOwnerName string
		var targetX, targetY int
		var targetScrap float64

		err := tx.QueryRowContext(ctx, `
			SELECT e.name, u.first_name, c.x, c.y, r.scrap
			FROM encampments e
			JOIN users u ON u.telegram_id = e.user_id
			JOIN coordinates c ON c.id = e.coordinate_id
			JOIN resources r ON r.encampment_id = e.id
			WHERE e.id != $1
			ORDER BY RANDOM()
			LIMIT 1`, s.campID).Scan(&targetName, &targetOwnerName, &targetX, &targetY, &targetScrap)
		if err != nil {
			continue // No other outposts to scan yet, or a transient error - skip silently.
		}

		reportMsg := fmt.Sprintf(
			"📡🔄 AUTOMATIC SCAN SWEEP COMPLETE\n\n"+
				"🎯 Rival Detected: %s (Commander: %s)\n"+
				"📍 Coordinates: [%d, %d]\n"+
				"♻️ Estimated Scrap: %.0f\n\n"+
				"💡 Use /scout %s for a full manual intel report.",
			targetName, targetOwnerName, targetX, targetY, targetScrap, targetOwnerName,
		)
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", s.userID, reportMsg)
	}

	return nil
}

func (e *Engine) ProcessTick() {
	start := time.Now()
	log.Println("⌛ Processing master game tick pass...")

	ctx := context.Background()

	phases := []tickPhase{
		{"weather", func(ctx context.Context, tx *sql.Tx) error { return e.weatherEngine.RunWeatherPass(ctx, tx) }},
		{"resources", func(ctx context.Context, tx *sql.Tx) error { return e.resourceProcessor.RunResourcePass(ctx, tx) }},
		{"agents", func(ctx context.Context, tx *sql.Tx) error { return e.agentProcessor.RunAgentPass(ctx, tx) }},
		{"logistics_consumption", e.applyActiveLogisticsConsumption},
		{"arena_matchmaking", e.processArenaMatchmaking},
		{"espionage", e.resolvePendingEspionageMissions},
		{"mining", e.resolveCompletedMiningQueues},
		{"starvation", func(ctx context.Context, tx *sql.Tx) error { return e.starvationEngine.RunStarvationPass(ctx, tx) }},
		{"construction", e.resolveCompletedUpgrades},
		{"staged_raid_departures", e.autoLaunchExpiredStagedRaids},
		{"coop_rallies", e.resolveCompletedCoopRallies},
		{"raid_combat", e.resolveRaidCombats},
		{"daily_tax", e.collectDailyTax},
		{"expired_world_events", func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "DELETE FROM world_events WHERE expires_at < CURRENT_TIMESTAMP")
			return err
		}},
	}

	for _, p := range phases {
		e.runPhase(ctx, p)
	}

	// 20-Minute Idle Miner Sweeper Check. Kept isolated from the main phase
	// list since it's on its own cadence rather than every tick.
	if time.Now().UTC().Sub(e.lastIdleCheck) >= 20*time.Minute {
		e.runPhase(ctx, tickPhase{"idle_miner_sweep", e.notifyIdleMiners})
		e.lastIdleCheck = time.Now().UTC()
	}

	if time.Now().UTC().Sub(e.lastAutoScan) >= 30*time.Minute {
		e.runPhase(ctx, tickPhase{"auto_scan_sweep", e.autoScanSweep})
		e.lastAutoScan = time.Now().UTC()
	}

	log.Printf("Tick pass complete. Duration: %s", time.Since(start))
}

// autoLaunchExpiredStagedRaids transitions staged co-op lobbies into active marching campaigns at timeout
func (e *Engine) autoLaunchExpiredStagedRaids(ctx context.Context, tx *sql.Tx) error {
	queryExpiredStaged := `
		SELECT r.id, r.attacker_id, r.defender_id, r.resolve_time
		FROM raids r
		WHERE r.state = 'staged' AND r.resolve_time <= CURRENT_TIMESTAMP`
	
	rows, err := tx.QueryContext(ctx, queryExpiredStaged)
	if err != nil {
		return err
	}
	defer rows.Close()

	type stagedRaid struct {
		id         string
		attackerID string
		defenderID sql.NullString
	}
	var expired []stagedRaid
	for rows.Next() {
		var sr stagedRaid
		if err := rows.Scan(&sr.id, &sr.attackerID, &sr.defenderID); err == nil {
			expired = append(expired, sr)
		}
	}
	rows.Close()

	for _, r := range expired {
		var totalHelpers int
		_ = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM raid_coop_members WHERE raid_id = $1", r.id).Scan(&totalHelpers)

		var creatorUserID int64
		_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", r.attackerID).Scan(&creatorUserID)

		if totalHelpers == 0 {
			var sols, mechs int
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers_mobilized, 0), COALESCE(mechs_mobilized, 0) FROM raid_forces WHERE raid_id = $1", r.id).Scan(&sols, &mechs)
			
			// Refund locked lobby creator forces
			_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2 WHERE encampment_id = $3", sols, mechs, r.attackerID)
			_, _ = tx.ExecContext(ctx, "DELETE FROM raids WHERE id = $1", r.id)

			cancelMsg := "🤝 CO-OP NOTICE: Your staged lobby was cancelled and initial forces refunded because no ally commanders joined before departure."
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", creatorUserID, cancelMsg)
			continue
		}

		// Force-complete any partial rallying transits into lead base
		_, _ = tx.ExecContext(ctx, "UPDATE raid_coop_members SET state = 'stationed' WHERE raid_id = $1", r.id)

		var myX, myY int
		_ = tx.QueryRowContext(ctx, "SELECT c.x, c.y FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", r.attackerID).Scan(&myX, &myY)

		var defX, defY int
		if r.defenderID.Valid {
			_ = tx.QueryRowContext(ctx, "SELECT c.x, c.y FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", r.defenderID.String).Scan(&defX, &defY)
		} else {
			defX, defY = 1, 1
		}

		steps := math.Abs(float64(defX-myX)) + math.Abs(float64(defY-myY))
		if steps == 0 {
			steps = 1
		}
		marchingMinutes := steps * 10.0
		arrival := time.Now().UTC().Add(time.Duration(marchingMinutes) * time.Minute)

		_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'marching', resolve_time = $1 WHERE id = $2", arrival, r.id)

		launchMsg := "🤝 CO-OP LOBBY DEPARTED: The staging window expired. Your joint military forces have successfully departed towards target."
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", creatorUserID, launchMsg)
	}
	return nil
}

// notifyIdleMiners sends a real-time warning notification to opted-in users who have idle workers
func (e *Engine) notifyIdleMiners(ctx context.Context, tx *sql.Tx) error {
	query := `
		SELECT u.telegram_id, e.name, COALESCE(w.miners, 1) as owned,
		       COALESCE((SELECT SUM(miners_assigned) FROM active_mining_queues WHERE encampment_id = e.id AND is_completed = FALSE), 0) as active
		FROM users u
		JOIN encampments e ON e.user_id = u.telegram_id
		JOIN workshop_inventory w ON w.encampment_id = e.id
		WHERE u.idle_miner_notifications = TRUE`
	
	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	type idleUser struct {
		userID   int64
		campName string
		idle     int
	}

	var targets []idleUser
	for rows.Next() {
		var uID int64
		var cName string
		var owned, active int
		if err := rows.Scan(&uID, &cName, &owned, &active); err == nil {
			if owned > active {
				targets = append(targets, idleUser{userID: uID, campName: cName, idle: owned - active})
			}
		}
	}
	rows.Close()

	for _, t := range targets {
		alert := fmt.Sprintf("⛏️ WORKSTATION ALERT: Outpost [%s] currently has %d idle miners stationed in the hangar. Open Active Mining on camp deck to deploy them!", t.campName, t.idle)
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", t.userID, alert)
	}

	return nil
}

func (e *Engine) resolveCompletedCoopRallies(ctx context.Context, tx *sql.Tx) error {
	queryRallies := `
		UPDATE raid_coop_members 
		SET state = 'stationed' 
		WHERE state = 'marching_to_ally' AND arrival_time <= CURRENT_TIMESTAMP`
	
	_, err := tx.ExecContext(ctx, queryRallies)
	if err != nil {
		return fmt.Errorf("failed executing co-op rally state transition: %w", err)
	}
	return nil
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
		if m.readyAt.UTC().After(time.Now().UTC()) {
			continue
		}

		var gain float64
		var column string

		switch m.resType {
		case "metal":
			gain = float64(m.miners * 20)
			column = "metal"
		case "crystal":
			gain = float64(m.miners * 5)
			column = "crystal"
		case "hydrogen":
			gain = float64(m.miners * 10)
			column = "hydrogen"
		}

		if column != "" {
			queryUpdateRes := fmt.Sprintf("UPDATE resources SET %s = %s + $1 WHERE encampment_id = $2", column, column)
			_, _ = tx.ExecContext(ctx, queryUpdateRes, gain, m.encampmentID)
		}

		_, _ = tx.ExecContext(ctx, "UPDATE active_mining_queues SET is_completed = TRUE WHERE id = $1", m.id)

		alertMsg := fmt.Sprintf("⛏️ EXTRACTION COMPLETE: Your miners successfully returned with +%.1f %s!", gain, m.resType)
		queryNotif := "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)"
		_, _ = tx.ExecContext(ctx, queryNotif, m.userID, alertMsg)
	}

	return nil
}

func (e *Engine) resolvePendingEspionageMissions(ctx context.Context, tx *sql.Tx) error {
	// Step 1: Process Outbound Spy Satellites reaching target (resolved = FALSE)
	queryOutbound := `
		SELECT s.id, s.spy_id, s.target_id, ea.name as spy_name, ed.name as target_name, ed.user_id as target_user_id, s.resolve_time
		FROM spy_missions s
		JOIN encampments ea ON ea.id = s.spy_id
		JOIN encampments ed ON ed.id = s.target_id
		WHERE s.resolved = FALSE AND s.is_intercepted = FALSE`
	
	rows, err := tx.QueryContext(ctx, queryOutbound)
	if err != nil {
		return fmt.Errorf("failed querying outbound espionage: %w", err)
	}
	defer rows.Close()

	type outboundSpy struct {
		id           string
		spyID        string
		targetID     string
		spyName      string
		targetName   string
		targetUserID int64
		resolveTime  time.Time
	}
	var outSpies []outboundSpy
	for rows.Next() {
		var s outboundSpy
		if err := rows.Scan(&s.id, &s.spyID, &s.targetID, &s.spyName, &s.targetName, &s.targetUserID, &s.resolveTime); err == nil {
			outSpies = append(outSpies, s)
		}
	}
	rows.Close()

	for _, s := range outSpies {
		if s.resolveTime.UTC().After(time.Now().UTC()) {
			continue
		}

		// Scanning complete. Transition into the return leg.
		returnDuration := 2 * time.Minute // Simulated return flight time
		returnResolveTime := time.Now().UTC().Add(returnDuration)

		_, _ = tx.ExecContext(ctx, "UPDATE spy_missions SET resolved = TRUE, resolve_time = $1 WHERE id = $2", returnResolveTime, s.id)

		defenderAlert := fmt.Sprintf(
			"🛰️ ESPIONAGE BREACH: Outpost [%s] has successfully scanned your warehouse telemetry!\n"+
				"The spy satellite is now returning to orbit. You have %d seconds left to launch an Interceptor Drone and vaporize the intel before it lands.",
			s.spyName, int(returnDuration.Seconds()),
		)
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", s.targetUserID, defenderAlert)
	}

	// Step 2: Process Returning Spy Satellites landing back at base (resolved = TRUE)
	queryReturning := `
		SELECT s.id, s.spy_id, s.target_id, s.is_intercepted, ea.user_id as spy_user_id, s.resolve_time
		FROM spy_missions s
		JOIN encampments ea ON ea.id = s.spy_id
		WHERE s.resolved = TRUE`

	rowsRet, err := tx.QueryContext(ctx, queryReturning)
	if err != nil {
		return fmt.Errorf("failed querying returning espionage: %w", err)
	}
	defer rowsRet.Close()

	type returningSpy struct {
		id            string
		spyID         string
		targetID      string
		isIntercepted bool
		spyUserID     int64
		resolveTime   time.Time
	}
	var retSpies []returningSpy
	for rowsRet.Next() {
		var s returningSpy
		if err := rowsRet.Scan(&s.id, &s.spyID, &s.targetID, &s.isIntercepted, &s.spyUserID, &s.resolveTime); err == nil {
			retSpies = append(retSpies, s)
		}
	}
	rowsRet.Close()

	for _, s := range retSpies {
		if s.resolveTime.UTC().After(time.Now().UTC()) {
			continue
		}

		if s.isIntercepted {
			_, _ = tx.ExecContext(ctx, "DELETE FROM spy_missions WHERE id = $1", s.id)
			continue
		}

		var targetName string
		_ = tx.QueryRowContext(ctx, "SELECT name FROM encampments WHERE id = $1", s.targetID).Scan(&targetName)

		var tentLvl, heapLvl, genLvl int = 1, 1, 1
		queryMod := "SELECT level FROM modules WHERE encampment_id = $1 AND type = $2"
		_ = tx.QueryRowContext(ctx, queryMod, s.targetID, "tent").Scan(&tentLvl)
		_ = tx.QueryRowContext(ctx, queryMod, s.targetID, "scrap_heap").Scan(&heapLvl)
		_ = tx.QueryRowContext(ctx, queryMod, s.targetID, "generator").Scan(&genLvl)

		var scrap, rations float64
		_ = tx.QueryRowContext(ctx, "SELECT scrap, rations FROM resources WHERE encampment_id = $1", s.targetID).Scan(&scrap, &rations)

		spyReport := fmt.Sprintf(
			"🛰️ ESPIONAGE DOWNLINK COMPLETED!\n\n"+
				"Your spy satellite has returned safely to hangar. Telemetry fully decrypted:\n\n"+
				"Target Outpost: %s\n\n"+
				"DECRYPTED RESOURCES:\n"+
				"⚙️ Scrap: %.1f\n"+
				"🥫 Rations: %.1f\n\n"+
				"MODULE STATUS GRID:\n"+
				"⛺ Tent: Level %d\n"+
				"⚙️ Scrap Heap: Level %d\n"+
				"⚡ Generator: Level %d",
			targetName, scrap, rations, tentLvl, heapLvl, genLvl,
		)

		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", s.spyUserID, spyReport)
		_, _ = tx.ExecContext(ctx, "DELETE FROM spy_missions WHERE id = $1", s.id)
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

	for _, ex := range exps {
		var rations, metalFuel float64
		_ = tx.QueryRowContext(ctx, "SELECT rations, metal FROM resources WHERE encampment_id = $1 FOR UPDATE", ex.attackerID).Scan(&rations, &metalFuel)

		var tankers int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(tankers, 0) FROM workshop_inventory WHERE encampment_id = $1", ex.attackerID).Scan(&tankers)

		deductRations := 3.0
		deductMetalFuel := 1.0

		if tankers > 0 {
			deductMetalFuel = 0.80
		}

		newRations := math.Max(rations-deductRations, 0.0)
		newMetalFuel := math.Max(metalFuel-deductMetalFuel, 0.0)

		_, _ = tx.ExecContext(ctx, "UPDATE resources SET rations = $1, metal = $2 WHERE encampment_id = $3", newRations, newMetalFuel, ex.attackerID)

		if newMetalFuel <= 0 {
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
				participants = append(participants, qu)
			}
		}
		rows.Close()

		if len(participants) >= e.requiredMatchCount(b) {
			requiredCount := e.requiredMatchCount(b)
			matched := participants[:requiredCount]

			winners := matched[:requiredCount/2]
			losers := matched[requiredCount/2:]

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

func (e *Engine) requiredMatchCount(bracket string) int {
	switch bracket {
	case "2v2":
		return 4
	case "3v3":
		return 6
	default:
		return 2
	}
}

func (e *Engine) resolveCompletedUpgrades(ctx context.Context, tx *sql.Tx) error {
	query := `
		SELECT m.id, e.user_id, e.name, m.type, m.level, m.upgrade_ready_at
		FROM modules m
		JOIN encampments e ON e.id = m.encampment_id
		WHERE m.is_upgrading = TRUE`

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed selecting completed upgrades: %w", err)
	}
	defer rows.Close()

	type completedUpgrade struct {
		id             string
		userID         int64
		campName       string
		modType        string
		oldLvl         int
		upgradeReadyAt sql.NullTime
	}

	var completed []completedUpgrade
	for rows.Next() {
		var c completedUpgrade
		if err := rows.Scan(&c.id, &c.userID, &c.campName, &c.modType, &c.oldLvl, &c.upgradeReadyAt); err == nil {
			completed = append(completed, c)
		}
	}

	for _, c := range completed {
		if c.upgradeReadyAt.Valid && c.upgradeReadyAt.Time.UTC().After(time.Now().UTC()) {
			continue
		}

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
		SELECT r.id, r.resolve_time, ea.name, COALESCE(ed.user_id, 0), COALESCE(ed.name, 'Rogue Drone Nest'), rf.route_type
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		LEFT JOIN encampments ed ON ed.id = r.defender_id
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
				if routeType != "stealth" && resTime.UTC().After(time.Now().UTC()) {
					timeLeft := int(time.Until(resTime.UTC()).Seconds())
					if timeLeft > 0 && defUserID != 0 {
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
		       COALESCE(ed.user_id, 0) as defender_user_id, r.resolve_time, 
		       COALESCE(r.stolen_scrap, 0.0) as stolen_scrap,
		       COALESCE(r.stolen_metal, 0.0) as stolen_metal,
		       COALESCE(r.stolen_crystal, 0.0) as stolen_crystal,
		       COALESCE(r.attacker_rations, 100.0) as attacker_rations,
		       COALESCE(r.attacker_ammo, 100.0) as attacker_ammo,
		       COALESCE(r.attacker_losses, 0) as attacker_losses,
		       COALESCE(r.defender_losses, 0) as defender_losses
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
		id              string
		attackerID      string
		defenderID      sql.NullString
		state           string
		roundNumber     int
		attackerName    string
		attackerUserID  int64
		defenderName    string
		defenderUserID  int64
		resolveTime     time.Time
		stolenScrap     float64
		stolenMetal     float64
		stolenCrystal   float64
		attackerRations float64
		attackerAmmo    float64
		attackerLosses  int
		defenderLosses  int
	}

	var raids []activeRaid
	for rows.Next() {
		var r activeRaid
		err := rows.Scan(
			&r.id, &r.attackerID, &r.defenderID, &r.state, &r.roundNumber,
			&r.attackerName, &r.attackerUserID,
			&r.defenderName, &r.defenderUserID, &r.resolveTime, &r.stolenScrap,
			&r.stolenMetal, &r.stolenCrystal,
			&r.attackerRations, &r.attackerAmmo, &r.attackerLosses, &r.defenderLosses,
		)
		if err == nil {
			raids = append(raids, r)
		}
	}
	rows.Close()

	for _, r := range raids {
		if r.resolveTime.UTC().After(time.Now().UTC()) {
			continue
		}

		if r.state == "returning" {
			var soldiersMob, mechsMob, destroyersMob, bombersMob, bcMob, dsMob int
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers_mobilized, 0), COALESCE(mechs_mobilized, 0), COALESCE(destroyers_mobilized, 0), COALESCE(bombers_mobilized, 0), COALESCE(battlecruisers_mobilized, 0), COALESCE(deathstars_mobilized, 0) FROM raid_forces WHERE raid_id = $1", r.id).Scan(&soldiersMob, &mechsMob, &destroyersMob, &bombersMob, &bcMob, &dsMob)

			_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2, destroyers = destroyers + $3, bombers = bombers + $4, battlecruisers = battlecruisers + $6, deathstars = deathstars + $7 WHERE encampment_id = $5", soldiersMob, mechsMob, destroyersMob, bombersMob, r.attackerID, bcMob, dsMob)
			_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap + $1, metal = metal + $2, crystal = crystal + $3 WHERE encampment_id = $4", r.stolenScrap, r.stolenMetal, r.stolenCrystal, r.attackerID)
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'completed' WHERE id = $1", r.id)

			alertMsg := fmt.Sprintf("🚀 RETURN MARCH COMPLETED: Your survivors returned to base carrying +%.1f Scrap, +%.1f Metal, +%.1f Crystal!", r.stolenScrap, r.stolenMetal, r.stolenCrystal)
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

		var primarySoldiers, primaryMechs, primaryBuggies, primaryDestroyers, primaryBombers, primaryBC, primaryDS int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers_mobilized, 0), COALESCE(mechs_mobilized, 0), COALESCE(buggies_mobilized, 0), COALESCE(destroyers_mobilized, 0), COALESCE(bombers_mobilized, 0), COALESCE(battlecruisers_mobilized, 0), COALESCE(deathstars_mobilized, 0) FROM raid_forces WHERE raid_id = $1 FOR UPDATE", r.id).Scan(&primarySoldiers, &primaryMechs, &primaryBuggies, &primaryDestroyers, &primaryBombers, &primaryBC, &primaryDS)

		type coopContributor struct {
			encampment_id string
			soldiers      int
			mechs         int
		}

		var helpers []coopContributor
		
		queryHelpers := "SELECT encampment_id, soldiers_contributed, mechs_contributed FROM raid_coop_members WHERE raid_id = $1 AND state = 'stationed'"
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

		// Destroyers and Bombers aren't yet draftable into co-op lobbies
		// (only the raid creator's primary force can bring them), so no
		// helper contribution to add here - just the primary force.
		totDestroyers := primaryDestroyers
		totBombers := primaryBombers
		totBC := primaryBC
		totDS := primaryDS

		attackForce := totSoldiers + totMechs + totDestroyers + totBombers + totBC + totDS

		var attackerTanks int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(fusion_tanks, 0) FROM workshop_inventory WHERE encampment_id = $1", r.attackerID).Scan(&attackerTanks)

		var attackerMilitaryTechLvl int = 1
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(military_tech_lvl, 1) FROM research_states WHERE encampment_id = $1", r.attackerID).Scan(&attackerMilitaryTechLvl)

		var attackerIntegrityTechLvl int = 1
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(integrity_tech_lvl, 1) FROM research_states WHERE encampment_id = $1", r.attackerID).Scan(&attackerIntegrityTechLvl)

		var attackerBioLvl int = 1
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(bio_lvl, 1) FROM mutation_states WHERE encampment_id = $1", r.attackerID).Scan(&attackerBioLvl)

		var attackerHeroSuperpower string
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(h.superpower, '') FROM raid_forces rf LEFT JOIN heroes h ON h.id = rf.hero_id WHERE rf.raid_id = $1", r.id).Scan(&attackerHeroSuperpower)

		var defLevel int = 1
		var defenderShields int = 0
		var defenderAgentActive bool = false
		var targetBiome string = "wasteland"
		var defenderBioLvl int = 1
		var defenderIntegrityTechLvl int = 1
		var defenderTurretLevels int = 0
		var defenderHeroSuperpower string
		var soldiersDefender, dronesDefender, jetsDefender, mechsDefender, scoutsDefender int

		if !r.defenderID.Valid {
			var attackerCoreLvl int
			_ = tx.QueryRowContext(ctx, "SELECT level FROM encampments WHERE id = $1", r.attackerID).Scan(&attackerCoreLvl)
			if attackerCoreLvl <= 0 {
				attackerCoreLvl = 1
			}
			soldiersDefender = attackerCoreLvl * 18
			defLevel = attackerCoreLvl
		} else {
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers, 0), COALESCE(drones, 0), COALESCE(jets, 0), COALESCE(mechs, 0), COALESCE(scouts, 0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", r.defenderID.String).Scan(&soldiersDefender, &dronesDefender, &jetsDefender, &mechsDefender, &scoutsDefender)
			_ = tx.QueryRowContext(ctx, "SELECT level FROM modules WHERE encampment_id = $1 AND type = 'tent'", r.defenderID.String).Scan(&defLevel)
			if defLevel == 0 {
				defLevel = 1
			}

			_ = tx.QueryRowContext(ctx, "SELECT COALESCE((SELECT nuclear_shields FROM workshop_inventory WHERE encampment_id = $1), 0)", r.defenderID.String).Scan(&defenderShields)
			_ = tx.QueryRowContext(ctx, "SELECT is_active FROM agent_tasks WHERE user_id = $1", r.defenderUserID).Scan(&defenderAgentActive)
			_ = tx.QueryRowContext(ctx, "SELECT c.biome FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", r.defenderID.String).Scan(&targetBiome)
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(bio_lvl, 1) FROM mutation_states WHERE encampment_id = $1", r.defenderID.String).Scan(&defenderBioLvl)
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(integrity_tech_lvl, 1) FROM research_states WHERE encampment_id = $1", r.defenderID.String).Scan(&defenderIntegrityTechLvl)

			// Defense Grid: sum all turret levels (Light/Heavy Laser, Gauss
			// Cannon, Ion Cannon, Plasma Turret) into a single bonus. Each
			// level of any turret adds a flat slice of defense rating.
			_ = tx.QueryRowContext(ctx,
				"SELECT COALESCE(SUM(level), 0) FROM modules WHERE encampment_id = $1 AND type IN ('light_laser', 'heavy_laser', 'gauss_cannon', 'ion_cannon', 'plasma_turret')",
				r.defenderID.String).Scan(&defenderTurretLevels)
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(superpower, '') FROM heroes WHERE encampment_id = $1", r.defenderID.String).Scan(&defenderHeroSuperpower)
		}

		defenseForce := soldiersDefender + dronesDefender + jetsDefender + mechsDefender

		var activeWeather string
		_ = tx.QueryRowContext(ctx, "SELECT active_weather FROM world_state WHERE id = 1").Scan(&activeWeather)

		offenseRatingModifier := 1.0
		defenseRatingModifier := 1.0 + (float64(defLevel) * 0.15) + (float64(defenderTurretLevels) * 0.08) + math.Min(float64(scoutsDefender)*0.01, 0.20)

		switch targetBiome {
		case "forest":
			offenseRatingModifier *= 0.85
			defenseRatingModifier *= 1.15
		case "ruins":
			offenseRatingModifier *= 0.75
			defenseRatingModifier *= 1.30
		}

		// Dynamic weather combat rating calculations
		var weatherNotice string = ""
		switch activeWeather {
		case "solar_flare":
			// Solar Flare introduces accuracy/electronic target variance (scrambles telemetry)
			offenseRatingModifier *= (0.80 + rand.Float64()*0.40)
			weatherNotice = "\n⚠️ SOLAR FLARE ACTIVE: Scrambling electronic communication and target locks! Offensive variance fluctuated."
		case "radiation_storm":
			offenseRatingModifier *= 0.75
			weatherNotice = "\n⚠️ RADIATION STORM ACTIVE: Fallout morale penalty applied! Base offense rating reduced by 25%."
		case "acid_rain":
			totMechs = totMechs / 2
			weatherNotice = "\n⚠️ ACID RAIN ACTIVE: Corrosive rain detected! Armored Mech defensive structures degraded by 50%."
		}

		if r.attackerRations <= 0 || r.attackerAmmo <= 0 {
			offenseRatingModifier *= 0.50
		}

		mechOffenseMultiplier := 1.50 * (1.0 + float64(attackerMilitaryTechLvl-1)*0.25)

		attRating := (float64(attackForce) * 15.0 * offenseRatingModifier) * (1.0 + (float64(attackerTanks) * 0.50) + (float64(totMechs) * mechOffenseMultiplier))

		// Destroyer: anti-drone/anti-air specialist. Rating scales up the
		// more drones/jets the defender is fielding - a hard counter unit.
		if totDestroyers > 0 {
			destroyerTargets := dronesDefender + jetsDefender
			destroyerBonus := 1.0 + math.Min(float64(destroyerTargets)*0.02, 1.0)
			attRating += float64(totDestroyers) * 20.0 * offenseRatingModifier * destroyerBonus
		}

		// Bomber: siege specialist. Rating scales up the more fortified the
		// defender's Defense Grid is - a hard counter to turtled bases.
		if totBombers > 0 {
			bomberBonus := 1.0 + math.Min(float64(defenderTurretLevels)*0.05, 1.5)
			attRating += float64(totBombers) * 18.0 * offenseRatingModifier * bomberBonus
		}

		// Battlecruiser: top-tier capital ship, flat massive attack rating
		// with no situational counter-bonus - the expensive, always-strong
		// "flex" unit. Attack value sourced from the canonical content
		// registry rather than hardcoded here.
		if totBC > 0 {
			bcUnit := content.MustFindUnit("battlecruiser")
			attRating += float64(totBC) * bcUnit.AttackRating * offenseRatingModifier
		}

		// Deathstar: the ultimate superweapon. Flat, enormous attack rating -
		// a single Doomsday Rig can single-handedly decide a battle.
		if totDS > 0 {
			dsUnit := content.MustFindUnit("deathstar")
			attRating += float64(totDS) * dsUnit.AttackRating * offenseRatingModifier
		}

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
			reduction := float64(attackerBioLvl-1)*0.10 + float64(attackerIntegrityTechLvl-1)*0.04
			reduction = math.Min(reduction, 0.90)
			attCas = int(float64(attCas) * (1.0 - reduction))
		}

		if defCas > 0 && r.defenderID.Valid {
			reduction := float64(defenderBioLvl-1)*0.10 + float64(defenderIntegrityTechLvl-1)*0.04
			if strings.Contains(defenderHeroSuperpower, "Kinetic Barrier") {
				reduction += 0.15
			}
			reduction = math.Min(reduction, 0.90)
			defCas = int(float64(defCas) * (1.0 - reduction))
		}

		// SpaceHunt-style "Exploding Units": each destroyed unit has a chance
		// to blow up and take a unit from the opposing side down with it.
		// This keeps lopsided wins from being entirely free for the winner.
		const explodeChance = 0.15
		bonusAttCasFromDefLosses := 0
		for i := 0; i < defCas; i++ {
			if rand.Float64() < explodeChance {
				bonusAttCasFromDefLosses++
			}
		}
		bonusDefCasFromAttLosses := 0
		for i := 0; i < attCas; i++ {
			if rand.Float64() < explodeChance {
				bonusDefCasFromAttLosses++
			}
		}
		attCas += bonusAttCasFromDefLosses
		defCas += bonusDefCasFromAttLosses

		// Casualty split: preserve the original 70/30 soldier/mech ratio for
		// that pool, and split whatever share of casualties falls on the
		// Destroyer/Bomber pool proportional to their relative counts. When
		// no destroyers/bombers are drafted this reduces to exactly the old
		// behavior (soldierMechShare = 1.0).
		soldierMechShare := 0.0
		if attackForce > 0 {
			soldierMechShare = float64(totSoldiers+totMechs) / float64(attackForce)
		}
		smCas := int(float64(attCas) * soldierMechShare)
		dbCas := attCas - smCas

		lostAttSols := int(float64(smCas) * 0.70)
		lostAttMechs := smCas - lostAttSols
		if lostAttSols > primarySoldiers {
			lostAttSols = primarySoldiers
		}
		if lostAttMechs > primaryMechs {
			lostAttMechs = primaryMechs
		}

		var lostAttDestroyers, lostAttBombers, lostAttBC, lostAttDS int
		specialistPool := totDestroyers + totBombers + totBC + totDS
		if specialistPool > 0 {
			lostAttDestroyers = int(float64(dbCas) * float64(totDestroyers) / float64(specialistPool))
			lostAttBombers = int(float64(dbCas) * float64(totBombers) / float64(specialistPool))
			lostAttBC = int(float64(dbCas) * float64(totBC) / float64(specialistPool))
			lostAttDS = dbCas - lostAttDestroyers - lostAttBombers - lostAttBC
			if lostAttDestroyers > primaryDestroyers {
				lostAttDestroyers = primaryDestroyers
			}
			if lostAttBombers > primaryBombers {
				lostAttBombers = primaryBombers
			}
			if lostAttBC > primaryBC {
				lostAttBC = primaryBC
			}
			if lostAttDS > primaryDS {
				lostAttDS = primaryDS
			}
		}

		newAttSols := primarySoldiers - lostAttSols
		newAttMechs := primaryMechs - lostAttMechs
		newAttDestroyers := primaryDestroyers - lostAttDestroyers
		newAttBombers := primaryBombers - lostAttBombers
		newAttBC := primaryBC - lostAttBC
		newAttDS := primaryDS - lostAttDS

		_, _ = tx.ExecContext(ctx, "UPDATE raid_forces SET soldiers_mobilized = $1, mechs_mobilized = $2, destroyers_mobilized = $4, bombers_mobilized = $5, battlecruisers_mobilized = $6, deathstars_mobilized = $7 WHERE raid_id = $3", newAttSols, newAttMechs, r.id, newAttDestroyers, newAttBombers, newAttBC, newAttDS)

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

		var rigs int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(rigs, 0) FROM workshop_inventory WHERE encampment_id = $1", r.attackerID).Scan(&rigs)

		if rigs > 0 {
			lostDefMechs := int(float64(lostAttMechs) * 0.15)
			newAttMechs = int(math.Min(float64(newAttMechs+lostDefMechs), float64(primaryMechs)))
			_, _ = tx.ExecContext(ctx, "UPDATE raid_forces SET mechs_mobilized = $1 WHERE raid_id = $2", newAttMechs, r.id)
		}

		var lostDefSols, lostDefMechs, lostDefDrones, lostDefJets int
		if r.defenderID.Valid && defCas > 0 {
			lostDefSols = int(float64(defCas) * 0.60)
			lostDefMechs = int(float64(defCas) * 0.20)
			if lostDefSols > soldiersDefender {
				lostDefSols = soldiersDefender
			}
			if lostDefMechs > mechsDefender {
				lostDefMechs = mechsDefender
			}

			remainingDefCas := defCas - lostDefSols - lostDefMechs
			if remainingDefCas > 0 && dronesDefender+jetsDefender > 0 {
				lostDefDrones = int(float64(remainingDefCas) * float64(dronesDefender) / float64(dronesDefender+jetsDefender))
				lostDefJets = remainingDefCas - lostDefDrones
				if lostDefDrones > dronesDefender {
					lostDefDrones = dronesDefender
				}
				if lostDefJets > jetsDefender {
					lostDefJets = jetsDefender
				}
			}

			_, _ = tx.ExecContext(ctx,
				"UPDATE workshop_inventory SET soldiers = GREATEST(soldiers - $1, 0), mechs = GREATEST(mechs - $2, 0), drones = GREATEST(drones - $3, 0), jets = GREATEST(jets - $4, 0) WHERE encampment_id = $5",
				lostDefSols, lostDefMechs, lostDefDrones, lostDefJets, r.defenderID.String)
		}

		attackerStillStanding := (newAttSols + newAttMechs + newAttDestroyers + newAttBombers + newAttBC + newAttDS) > 0
		defenderStillStanding := (defenseForce - defCas) > 0

		var defenderScrap, defenderMetal, defenderCrystal float64
		if r.defenderID.Valid {
			_ = tx.QueryRowContext(ctx, "SELECT scrap, metal, crystal FROM resources WHERE encampment_id = $1 FOR UPDATE", r.defenderID.String).Scan(&defenderScrap, &defenderMetal, &defenderCrystal)
		} else {
			defenderScrap = 125.0
			defenderMetal = 300.0
			defenderCrystal = 60.0
		}

		lootPercentage := 0.40
		if defenderShields > 0 {
			lootPercentage = 0.20
		}
		stolenScrap := defenderScrap * lootPercentage
		stolenMetal := defenderMetal * lootPercentage
		stolenCrystal := defenderCrystal * lootPercentage
		primaryShare := stolenScrap
		primaryMetalShare := stolenMetal
		primaryCrystalShare := stolenCrystal

		// Build the SpaceHunt-style battle report. Composition reflects
		// forces standing AT THE START of this round (before losses); loss
		// tallies are what fell THIS round specifically.
		report := battlereport.Round{
			Number:       r.roundNumber,
			AttackerName: r.attackerName,
			DefenderName: r.defenderName,
			AttackerComposition: []battlereport.UnitTally{
				{Emoji: "🪖", Label: "Soldiers", Count: totSoldiers},
				{Emoji: "🤖", Label: "Mechs", Count: totMechs},
				{Emoji: "💥", Label: "Destroyers", Count: totDestroyers},
				{Emoji: "🛩️", Label: "Bombers", Count: totBombers},
				{Emoji: "🚢👑", Label: "Battlecruisers", Count: totBC},
				{Emoji: "🌑💀", Label: "Doomsday Rigs", Count: totDS},
			},
			DefenderComposition: []battlereport.UnitTally{
				{Emoji: "🪖", Label: "Soldiers", Count: soldiersDefender},
				{Emoji: "🛰️", Label: "Drones", Count: dronesDefender},
				{Emoji: "✈️", Label: "Jets", Count: jetsDefender},
				{Emoji: "🤖", Label: "Mechs", Count: mechsDefender},
			},
			AttackerLosses: []battlereport.UnitTally{
				{Emoji: "🪖", Label: "Soldiers", Count: lostAttSols},
				{Emoji: "🤖", Label: "Mechs", Count: lostAttMechs},
				{Emoji: "💥", Label: "Destroyers", Count: lostAttDestroyers},
				{Emoji: "🛩️", Label: "Bombers", Count: lostAttBombers},
				{Emoji: "🚢👑", Label: "Battlecruisers", Count: lostAttBC},
				{Emoji: "🌑💀", Label: "Doomsday Rigs", Count: lostAttDS},
			},
			DefenderLosses: []battlereport.UnitTally{
				{Emoji: "🪖", Label: "Soldiers", Count: lostDefSols},
				{Emoji: "🛰️", Label: "Drones", Count: lostDefDrones},
				{Emoji: "✈️", Label: "Jets", Count: lostDefJets},
				{Emoji: "🤖", Label: "Mechs", Count: lostDefMechs},
			},
		}

		switch {
		case !attackerStillStanding:
			report.Outcome = battlereport.OutcomeDefenderWon
		case !defenderStillStanding:
			report.Outcome = battlereport.OutcomeAttackerWon
			report.LootLines = []string{
				fmt.Sprintf("🧱 %.0f Metal", primaryMetalShare),
				fmt.Sprintf("💎 %.0f Crystal", primaryCrystalShare),
				fmt.Sprintf("♻️ %.0f Scrap", primaryShare),
			}
			report.LootCollector = r.attackerName
		case r.roundNumber >= 5:
			report.Outcome = battlereport.OutcomeDraw
		default:
			report.Outcome = battlereport.OutcomeOngoing
		}

		reportText := battlereport.Render(report)
		if weatherNotice != "" {
			reportText += "\n\n" + strings.TrimSpace(weatherNotice)
		}

		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.attackerUserID, reportText)
		if r.defenderID.Valid && r.defenderUserID != 0 {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.defenderUserID, reportText)
		}

		if !attackerStillStanding {
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'completed' WHERE id = $1", r.id)
		} else if !defenderStillStanding {
			primRatio := float64(primarySoldiers+primaryMechs) / float64(totSoldiers+totMechs)
			if math.IsNaN(primRatio) {
				primRatio = 1.0
			}
			primaryShare = stolenScrap * primRatio
			primaryMetalShare = stolenMetal * primRatio
			primaryCrystalShare = stolenCrystal * primRatio

			if strings.Contains(attackerHeroSuperpower, "Scrap Recovery") {
				primaryShare *= 1.10
				primaryMetalShare *= 1.10
				primaryCrystalShare *= 1.10
			}

			var haulers int
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(haulers, 0) FROM workshop_inventory WHERE encampment_id = $1", r.attackerID).Scan(&haulers)

			weightFactor := (primaryShare + primaryMetalShare + primaryCrystalShare) / 5000.0
			if haulers > 0 {
				weightFactor *= 0.50
			}

			returnMinutes := 15.0 * (1.0 + weightFactor)
			returnDuration := time.Duration(returnMinutes) * time.Minute
			resolveTime := time.Now().UTC().Add(returnDuration)

			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'returning', stolen_scrap = $1, stolen_metal = $4, stolen_crystal = $5, resolve_time = $2 WHERE id = $3", primaryShare, resolveTime, r.id, primaryMetalShare, primaryCrystalShare)

			if r.defenderID.Valid {
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = GREATEST(scrap - $1, 0), metal = GREATEST(metal - $2, 0), crystal = GREATEST(crystal - $3, 0) WHERE encampment_id = $4", stolenScrap, stolenMetal, stolenCrystal, r.defenderID.String)
			}

			for _, h := range helpers {
				_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2 WHERE encampment_id = $3", h.soldiers, h.mechs, h.encampment_id)
			}
		} else if r.roundNumber >= 5 {
			returnMinutes := 15.0
			resolveTime := time.Now().UTC().Add(time.Duration(returnMinutes) * time.Minute)

			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'returning', stolen_scrap = 0, resolve_time = $1 WHERE id = $2", resolveTime, r.id)

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