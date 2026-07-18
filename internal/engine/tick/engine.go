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
	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
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
						var curDollars float64
						_ = tx.QueryRowContext(ctx, "SELECT dollars FROM resources WHERE encampment_id = $1", w.campID).Scan(&curDollars)
						wCap := storagecap.CapFor(ctx, tx, w.campID)
						newDollars, _ := storagecap.Clamp(curDollars, share, wCap)
						_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = $1 WHERE encampment_id = $2", newDollars, w.campID)
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

// explorationSiteTemplate is one entry in the pool spawnExplorationSites
// rolls from - a flavor name/type paired with a reward currency and
// amount range.
type explorationSiteTemplate struct {
	siteType   string
	namePrefix string
	rewardType string
	minAmount  float64
	maxAmount  float64
}

var explorationTemplates = []explorationSiteTemplate{
	{"Ruins", "Ancient Ruins", "ether", 15, 40},
	{"Cache", "Supply Cache", "metal", 200, 500},
	{"Cache", "Supply Cache", "crystal", 80, 200},
	{"Artifact", "Tech Artifact", "ether", 30, 70},
	{"Beacon", "Signal Beacon", "dollars", 500, 1500},
}

const explorationSiteRollChance = 0.15
const explorationSiteDuration = 3 * time.Hour

// spawnExplorationSites is Phase 7 item 10's world-exploration engine.
// Same shape as the weather engine's per-continent roll: each continent
// gets a 15% chance per tick to spawn a new undiscovered site, but only
// if it doesn't already have one waiting to be claimed (keeps the map
// from flooding with sites nobody has time to reach).
func (e *Engine) spawnExplorationSites(ctx context.Context, tx *sql.Tx) error {
	for _, continent := range world.Continents {
		var hasOpenSite bool
		err := tx.QueryRowContext(ctx,
			"SELECT EXISTS(SELECT 1 FROM exploration_sites WHERE continent = $1 AND claimed_by IS NULL AND expires_at > CURRENT_TIMESTAMP)",
			continent).Scan(&hasOpenSite)
		if err != nil {
			continue
		}
		if hasOpenSite {
			continue
		}
		if rand.Float64() >= explorationSiteRollChance {
			continue
		}

		tmpl := explorationTemplates[rand.Intn(len(explorationTemplates))]
		sector := rand.Intn(99) + 1
		siteName := fmt.Sprintf("%s (Sector %d)", tmpl.namePrefix, sector)
		rewardAmount := tmpl.minAmount + rand.Float64()*(tmpl.maxAmount-tmpl.minAmount)
		expiresAt := time.Now().UTC().Add(explorationSiteDuration)

		_, err = tx.ExecContext(ctx,
			"INSERT INTO exploration_sites (continent, site_name, site_type, reward_type, reward_amount, expires_at) VALUES ($1, $2, $3, $4, $5, $6)",
			continent, siteName, tmpl.siteType, tmpl.rewardType, rewardAmount, expiresAt)
		if err != nil {
			log.Printf("Failed inserting exploration site for %s: %v", continent, err)
			continue
		}

		headline := fmt.Sprintf("🧭 UNCHARTED SIGNAL: A %s has been detected over %s. First outpost to dispatch an expedition claims it - check /explore.", siteName, continent)
		if _, err := tx.ExecContext(ctx, "INSERT INTO world_news (headline) VALUES ($1)", headline); err != nil {
			log.Printf("Failed writing exploration-site news headline: %v", err)
		}
	}
	return nil
}

// resolveExplorationDispatches credits the reward and marks the site
// claimed for every expedition whose resolve_time has passed.
func (e *Engine) resolveExplorationDispatches(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT d.id, d.site_id, d.encampment_id, d.user_id, s.site_name, s.reward_type, s.reward_amount
		FROM exploration_dispatches d
		JOIN exploration_sites s ON s.id = d.site_id
		WHERE d.resolve_time <= CURRENT_TIMESTAMP`)
	if err != nil {
		return fmt.Errorf("failed fetching resolved exploration dispatches: %w", err)
	}

	type resolvedDispatch struct {
		id           string
		siteID       string
		encampmentID string
		userID       int64
		siteName     string
		rewardType   string
		rewardAmount float64
	}
	var dispatches []resolvedDispatch
	for rows.Next() {
		var d resolvedDispatch
		if err := rows.Scan(&d.id, &d.siteID, &d.encampmentID, &d.userID, &d.siteName, &d.rewardType, &d.rewardAmount); err == nil {
			dispatches = append(dispatches, d)
		}
	}
	rows.Close()

	for _, d := range dispatches {
		storageCap := storagecap.CapFor(ctx, tx, d.encampmentID)

		var current float64
		column := d.rewardType // "metal", "crystal", "ether", or "dollars" - all valid column names on resources.
		_ = tx.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM resources WHERE encampment_id = $1 FOR UPDATE", column), d.encampmentID).Scan(&current)
		newAmount, _ := storagecap.Clamp(current, d.rewardAmount, storageCap)
		_, err := tx.ExecContext(ctx, fmt.Sprintf("UPDATE resources SET %s = $1 WHERE encampment_id = $2", column), newAmount, d.encampmentID)
		if err != nil {
			log.Printf("Failed crediting exploration reward for dispatch %s: %v", d.id, err)
			continue
		}

		_, _ = tx.ExecContext(ctx, "UPDATE exploration_sites SET claimed_by = $1, claimed_at = CURRENT_TIMESTAMP WHERE id = $2", d.encampmentID, d.siteID)
		_, _ = tx.ExecContext(ctx, "DELETE FROM exploration_dispatches WHERE id = $1", d.id)

		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", d.userID,
			fmt.Sprintf("🧭✅ EXPEDITION SUCCESSFUL: Your outpost has claimed %s! +%.0f %s credited to your reserves.", d.siteName, d.rewardAmount, d.rewardType))
	}
	return nil
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

// resolveWorldBossAttacks handles both stages of a real World Boss
// engagement: marching parties arriving deal damage AND take real
// retaliation casualties (the boss fights back), then survivors begin an
// actual return march instead of the old instant-resolve model. On a
// killing blow, loot is split by cumulative damage share exactly as
// before, just deferred until the engagement actually happens.
func (e *Engine) resolveWorldBossAttacks(ctx context.Context, tx *sql.Tx) error {
	// --- Stage 1: marching -> engage (retaliation happens here) ---
	marchingRows, err := tx.QueryContext(ctx, `
		SELECT a.id, a.boss_id, a.user_id, a.encampment_id, a.soldiers_committed, a.mechs_committed, a.march_minutes,
		       b.name, b.emoji, b.current_hp, b.max_hp, b.loot_pool_dollars, b.retaliation_rating
		FROM world_boss_attacks a
		JOIN world_bosses b ON b.id = a.boss_id
		WHERE a.state = 'marching' AND a.resolve_time <= CURRENT_TIMESTAMP`)
	if err != nil {
		return err
	}

	type marchingAttack struct {
		id, bossID, encampmentID       string
		userID                         int64
		soldiers, mechs                int
		marchMinutes                   float64
		bossName, bossEmoji            string
		bossCurHP, bossMaxHP, lootPool float64
		retaliation                    float64
	}
	var marching []marchingAttack
	for marchingRows.Next() {
		var m marchingAttack
		if scanErr := marchingRows.Scan(&m.id, &m.bossID, &m.userID, &m.encampmentID, &m.soldiers, &m.mechs, &m.marchMinutes,
			&m.bossName, &m.bossEmoji, &m.bossCurHP, &m.bossMaxHP, &m.lootPool, &m.retaliation); scanErr == nil {
			marching = append(marching, m)
		}
	}
	marchingRows.Close()

	for _, m := range marching {
		if m.bossCurHP <= 0 {
			// Someone else already killed it between launch and arrival -
			// no fight happens, survivors just turn back immediately.
			_, _ = tx.ExecContext(ctx, "UPDATE world_boss_attacks SET state = 'returning', resolve_time = CURRENT_TIMESTAMP + ($1 * INTERVAL '1 minute') WHERE id = $2", m.marchMinutes, m.id)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", m.userID,
				fmt.Sprintf("🏳️ %s was already defeated by another survivor before your forces arrived. Turning back - no fight, no losses.", m.bossName))
			continue
		}

		damage := float64(m.soldiers)*10.0 + float64(m.mechs)*400.0
		newHP := m.bossCurHP - damage
		killingBlow := newHP <= 0
		if newHP < 0 {
			newHP = 0
		}
		_, _ = tx.ExecContext(ctx, "UPDATE world_bosses SET current_hp = $1 WHERE id = $2", newHP, m.bossID)
		_, _ = tx.ExecContext(ctx, `
			INSERT INTO world_boss_contributions (boss_id, user_id, encampment_id, damage_dealt)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (boss_id, user_id) DO UPDATE SET damage_dealt = world_boss_contributions.damage_dealt + $4`,
			m.bossID, m.userID, m.encampmentID, damage)

		// Real retaliation: the boss hits back. Danger scales per boss
		// (retaliation_rating), and mechs are tankier than soldiers in the
		// losses split (mirroring the standard 70/30 raid pattern).
		totalCommitted := m.soldiers + m.mechs
		casualties := int(float64(totalCommitted) * m.retaliation / 100.0)
		if casualties > totalCommitted {
			casualties = totalCommitted
		}
		lostSoldiers := int(float64(casualties) * 0.75)
		lostMechs := casualties - lostSoldiers
		if lostSoldiers > m.soldiers {
			lostSoldiers = m.soldiers
		}
		if lostMechs > m.mechs {
			lostMechs = m.mechs
		}
		survivingSoldiers := m.soldiers - lostSoldiers
		survivingMechs := m.mechs - lostMechs

		report := battlereport.Round{
			Number:       1,
			AttackerName: "Your Strike Force",
			DefenderName: m.bossName,
			AttackerComposition: []battlereport.UnitTally{
				{Emoji: "🪖", Label: "Soldiers", Count: m.soldiers},
				{Emoji: "🤖", Label: "Mechs", Count: m.mechs},
			},
			DefenderComposition: []battlereport.UnitTally{
				{Emoji: m.bossEmoji, Label: m.bossName, Count: 1},
			},
			AttackerLosses: []battlereport.UnitTally{
				{Emoji: "🪖", Label: "Soldiers", Count: lostSoldiers},
				{Emoji: "🤖", Label: "Mechs", Count: lostMechs},
			},
			DefenderLosses: []battlereport.UnitTally{
				{Emoji: "💥", Label: fmt.Sprintf("Damage (%.0f/%.0f HP left)", newHP, m.bossMaxHP), Count: 1},
			},
		}

		if killingBlow {
			report.Outcome = battlereport.OutcomeAttackerWon
			report.LootLines = []string{fmt.Sprintf("💵 $%.0f (Boss Loot Pool - split by damage contribution)", m.lootPool)}
			report.LootCollector = "All contributors"
		} else {
			report.Outcome = battlereport.OutcomeDraw
		}
		reportText := battlereport.Render(report)

		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", m.userID, reportText)

		if killingBlow {
			if err := e.payoutWorldBossLoot(ctx, tx, m.bossID, m.bossName, m.lootPool); err != nil {
				log.Printf("World Boss loot payout failed: %v", err)
			}
		}

		// Survivors begin the real return march - same duration as the
		// trip out, exactly like a real raid, not an instant reset.
		_, _ = tx.ExecContext(ctx, `
			UPDATE world_boss_attacks 
			SET state = 'returning', soldiers_committed = $1, mechs_committed = $2, 
			    resolve_time = CURRENT_TIMESTAMP + ($3 * INTERVAL '1 minute')
			WHERE id = $4`,
			survivingSoldiers, survivingMechs, m.marchMinutes, m.id)
	}

	// --- Stage 2: returning -> home (restock survivors) ---
	returningRows, err := tx.QueryContext(ctx, `
		SELECT id, user_id, encampment_id, soldiers_committed, mechs_committed 
		FROM world_boss_attacks 
		WHERE state = 'returning' AND resolve_time <= CURRENT_TIMESTAMP`)
	if err != nil {
		return err
	}

	type returningAttack struct {
		id, encampmentID string
		userID           int64
		soldiers, mechs  int
	}
	var returning []returningAttack
	for returningRows.Next() {
		var r returningAttack
		if scanErr := returningRows.Scan(&r.id, &r.userID, &r.encampmentID, &r.soldiers, &r.mechs); scanErr == nil {
			returning = append(returning, r)
		}
	}
	returningRows.Close()

	for _, r := range returning {
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2 WHERE encampment_id = $3", r.soldiers, r.mechs, r.encampmentID)
		_, _ = tx.ExecContext(ctx, "DELETE FROM world_boss_attacks WHERE id = $1", r.id)

		if r.soldiers+r.mechs > 0 {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.userID,
				fmt.Sprintf("🚚 SURVIVORS RETURNED HOME: 🪖 %d Soldiers, 🤖 %d Mechs made it back from the World Boss engagement.", r.soldiers, r.mechs))
		} else {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.userID,
				"💀 No survivors returned from that World Boss engagement.")
		}
	}

	return nil
}

// payoutWorldBossLoot splits a defeated boss's loot pool proportional to
// each contributor's total damage dealt, then resets the boss to full HP
// with a 10% larger pool for long-term grinding incentive.
func (e *Engine) payoutWorldBossLoot(ctx context.Context, tx *sql.Tx, bossID, bossName string, lootPool float64) error {
	rows, err := tx.QueryContext(ctx, "SELECT user_id, encampment_id, damage_dealt FROM world_boss_contributions WHERE boss_id = $1", bossID)
	if err != nil {
		return err
	}

	type contributor struct {
		userID int64
		campID string
		damage float64
	}
	var contributors []contributor
	var totalDamage float64
	for rows.Next() {
		var ct contributor
		if scanErr := rows.Scan(&ct.userID, &ct.campID, &ct.damage); scanErr == nil {
			contributors = append(contributors, ct)
			totalDamage += ct.damage
		}
	}
	rows.Close()

	if totalDamage > 0 {
		for _, ct := range contributors {
			share := lootPool * (ct.damage / totalDamage)
			var curDollars float64
			_ = tx.QueryRowContext(ctx, "SELECT dollars FROM resources WHERE encampment_id = $1", ct.campID).Scan(&curDollars)
			ctCap := storagecap.CapFor(ctx, tx, ct.campID)
			newDollars, _ := storagecap.Clamp(curDollars, share, ctCap)
			_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = $1 WHERE encampment_id = $2", newDollars, ct.campID)
			alertMsg := fmt.Sprintf("☠️🎉 BOSS SLAIN: %s\n\nYour cumulative %.0f damage earned you 💵 $%.2f from the loot pool.", bossName, ct.damage, share)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ct.userID, alertMsg)
		}
	}

	_, _ = tx.ExecContext(ctx, "DELETE FROM world_boss_contributions WHERE boss_id = $1", bossID)
	_, _ = tx.ExecContext(ctx, `
		UPDATE world_bosses 
		SET current_hp = max_hp, loot_pool_dollars = loot_pool_dollars * 1.10, last_defeated_at = CURRENT_TIMESTAMP 
		WHERE id = $1`, bossID)

	return nil
}

// applyClanWarScore checks whether the attacker and defender encampments
// belong to two clans currently at war with each other, and if so, adds
// the raid's loot value to the attacker's clan war score. This is what
// makes Clan Wars a real, live-scored mechanic instead of a one-time
// notification with no lasting effect.
func (e *Engine) applyClanWarScore(ctx context.Context, tx *sql.Tx, attackerCampID, defenderCampID string, lootValue float64) {
	var attackerClanID, defenderClanID sql.NullString
	_ = tx.QueryRowContext(ctx, "SELECT uc.clan_id FROM user_clans uc JOIN encampments e ON e.user_id = uc.user_id WHERE e.id = $1", attackerCampID).Scan(&attackerClanID)
	_ = tx.QueryRowContext(ctx, "SELECT uc.clan_id FROM user_clans uc JOIN encampments e ON e.user_id = uc.user_id WHERE e.id = $1", defenderCampID).Scan(&defenderClanID)

	if !attackerClanID.Valid || !defenderClanID.Valid || attackerClanID.String == defenderClanID.String {
		return
	}

	var warID string
	var isClanA bool
	err := tx.QueryRowContext(ctx, `
		SELECT id, clan_a_id = $1
		FROM clan_wars
		WHERE status = 'active'
		AND ((clan_a_id = $1 AND clan_b_id = $2) OR (clan_a_id = $2 AND clan_b_id = $1))`,
		attackerClanID.String, defenderClanID.String).Scan(&warID, &isClanA)
	if err != nil {
		return // Not at war with each other - no war score change.
	}

	if isClanA {
		_, _ = tx.ExecContext(ctx, "UPDATE clan_wars SET score_a = score_a + $1 WHERE id = $2", lootValue, warID)
	} else {
		_, _ = tx.ExecContext(ctx, "UPDATE clan_wars SET score_b = score_b + $1 WHERE id = $2", lootValue, warID)
	}
}

// resolveClanWars closes out wars whose 48h window has expired, declares
// the higher-scoring clan the winner, and pays a shared Dollars spoils
// reward split evenly among the winning clan's members.
func (e *Engine) resolveClanWars(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, clan_a_id, clan_b_id, score_a, score_b
		FROM clan_wars
		WHERE status = 'active' AND ends_at <= CURRENT_TIMESTAMP`)
	if err != nil {
		return err
	}

	type expiredWar struct {
		id, clanA, clanB string
		scoreA, scoreB   float64
	}
	var wars []expiredWar
	for rows.Next() {
		var w expiredWar
		if scanErr := rows.Scan(&w.id, &w.clanA, &w.clanB, &w.scoreA, &w.scoreB); scanErr == nil {
			wars = append(wars, w)
		}
	}
	rows.Close()

	const spoilsPool = 10000.0 // Dollars, split evenly among the winning clan's members

	for _, w := range wars {
		_, _ = tx.ExecContext(ctx, "UPDATE clan_wars SET status = 'completed' WHERE id = $1", w.id)

		var winnerClanID, loserClanID string
		var winnerScore, loserScore float64
		isDraw := w.scoreA == w.scoreB

		if w.scoreA >= w.scoreB {
			winnerClanID, loserClanID = w.clanA, w.clanB
			winnerScore, loserScore = w.scoreA, w.scoreB
		} else {
			winnerClanID, loserClanID = w.clanB, w.clanA
			winnerScore, loserScore = w.scoreB, w.scoreA
		}

		var winnerName, loserName string
		_ = tx.QueryRowContext(ctx, "SELECT name FROM clans WHERE id = $1", winnerClanID).Scan(&winnerName)
		_ = tx.QueryRowContext(ctx, "SELECT name FROM clans WHERE id = $1", loserClanID).Scan(&loserName)

		if isDraw {
			drawMsg := fmt.Sprintf("🤝 CLAN WAR ENDED IN A DRAW!\n\n%s vs %s: %.0f - %.0f. Neither side prevailed.", winnerName, loserName, winnerScore, loserScore)
			e.notifyClanMembers(ctx, tx, w.clanA, drawMsg)
			e.notifyClanMembers(ctx, tx, w.clanB, drawMsg)
			continue
		}

		var winnerMembers []int64
		var winnerCamps []string
		memberRows, _ := tx.QueryContext(ctx, "SELECT uc.user_id, e.id FROM user_clans uc JOIN encampments e ON e.user_id = uc.user_id WHERE uc.clan_id = $1", winnerClanID)
		if memberRows != nil {
			for memberRows.Next() {
				var uid int64
				var campID string
				if memberRows.Scan(&uid, &campID) == nil {
					winnerMembers = append(winnerMembers, uid)
					winnerCamps = append(winnerCamps, campID)
				}
			}
			memberRows.Close()
		}

		if len(winnerCamps) > 0 {
			share := spoilsPool / float64(len(winnerCamps))
			for _, campID := range winnerCamps {
				var curDollars float64
				_ = tx.QueryRowContext(ctx, "SELECT dollars FROM resources WHERE encampment_id = $1", campID).Scan(&curDollars)
				campCap := storagecap.CapFor(ctx, tx, campID)
				newDollars, _ := storagecap.Clamp(curDollars, share, campCap)
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = $1 WHERE encampment_id = $2", newDollars, campID)
			}

			winMsg := fmt.Sprintf("🏆⚔️ CLAN WAR VICTORY!\n\nYour Clan defeated %s! Final score: %.0f - %.0f.\n💵 Your share of the spoils: $%.2f", loserName, winnerScore, loserScore, share)
			for _, uid := range winnerMembers {
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", uid, winMsg)
			}
		}

		loseMsg := fmt.Sprintf("💀⚔️ CLAN WAR DEFEAT\n\nYour Clan lost to %s. Final score: %.0f - %.0f.", winnerName, winnerScore, loserScore)
		e.notifyClanMembers(ctx, tx, loserClanID, loseMsg)
	}

	return nil
}

func (e *Engine) notifyClanMembers(ctx context.Context, tx *sql.Tx, clanID, message string) {
	rows, err := tx.QueryContext(ctx, "SELECT user_id FROM user_clans WHERE clan_id = $1", clanID)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var uid int64
		if rows.Scan(&uid) == nil {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", uid, message)
		}
	}
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
		{"world_boss_attacks", e.resolveWorldBossAttacks},
		{"clan_wars", e.resolveClanWars},
		{"daily_tax", e.collectDailyTax},
		{"exploration_spawn", e.spawnExplorationSites},
		{"exploration_resolve", e.resolveExplorationDispatches},
		{"expired_world_events", func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "DELETE FROM world_events WHERE expires_at < CURRENT_TIMESTAMP")
			return err
		}},
		{"expired_exploration_sites", func(ctx context.Context, tx *sql.Tx) error {
			_, err := tx.ExecContext(ctx, "DELETE FROM exploration_sites WHERE claimed_by IS NULL AND expires_at < CURRENT_TIMESTAMP")
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
		SELECT r.id, r.attacker_id, r.state, r.resolve_time, ea.user_id,
		       COALESCE(r.attacker_rations, 100.0), COALESCE(r.attacker_ammo, 100.0),
		       COALESCE(r.base_march_minutes, 15.0), COALESCE(ed.name, 'Rogue Drone Nest')
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		LEFT JOIN encampments ed ON ed.id = r.defender_id
		WHERE r.state = 'marching' OR r.state = 'engaged'`

	rows, err := tx.QueryContext(ctx, queryExpeditions)
	if err != nil {
		return fmt.Errorf("failed fetching active expeditions: %w", err)
	}
	defer rows.Close()

	type activeExp struct {
		id               string
		attackerID       string
		state            string
		resolveTime      time.Time
		userID           int64
		rations          float64
		ammo             float64
		baseMarchMinutes float64
		defenderName     string
	}

	var exps []activeExp
	for rows.Next() {
		var ex activeExp
		if err := rows.Scan(&ex.id, &ex.attackerID, &ex.state, &ex.resolveTime, &ex.userID,
			&ex.rations, &ex.ammo, &ex.baseMarchMinutes, &ex.defenderName); err == nil {
			exps = append(exps, ex)
		}
	}

	for _, ex := range exps {
		var homeRations, metalFuel float64
		_ = tx.QueryRowContext(ctx, "SELECT rations, metal FROM resources WHERE encampment_id = $1 FOR UPDATE", ex.attackerID).Scan(&homeRations, &metalFuel)

		var tankers int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(tankers, 0) FROM workshop_inventory WHERE encampment_id = $1", ex.attackerID).Scan(&tankers)

		deductRations := 3.0
		deductMetalFuel := 1.0
		if tankers > 0 {
			deductMetalFuel = 0.80
		}

		newHomeRations := math.Max(homeRations-deductRations, 0.0)
		newMetalFuel := math.Max(metalFuel-deductMetalFuel, 0.0)
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET rations = $1, metal = $2 WHERE encampment_id = $3", newHomeRations, newMetalFuel, ex.attackerID)

		if newMetalFuel <= 0 {
			delayedResolve := ex.resolveTime.UTC().Add(3 * time.Minute)
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET resolve_time = $1 WHERE id = $2", delayedResolve, ex.id)
		}

		// Raid-scoped supply pools (attacker_rations/attacker_ammo on the
		// raid itself, distinct from the home base's stockpile above) -
		// this is what actually reflects "the army in the field is running
		// out of what it packed for the march", and previously ammo was a
		// dead stub that never moved off its 100.0 default.
		oldRations := ex.rations
		oldAmmo := ex.ammo
		newRations := math.Max(oldRations-4.0, 0.0)
		newAmmo := math.Max(oldAmmo-4.0, 0.0)

		_, _ = tx.ExecContext(ctx, "UPDATE raids SET attacker_rations = $1, attacker_ammo = $2 WHERE id = $3", newRations, newAmmo, ex.id)

		// Threshold-crossing notifications only (not every tick), so the
		// player gets one heads-up at 25% and one at empty per resource,
		// instead of spam every 3 seconds.
		if oldRations > 25.0 && newRations <= 25.0 {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ex.userID,
				fmt.Sprintf("🍖 RATIONS RUNNING LOW: Your marching force toward [%s] is down to %.0f%% rations.", ex.defenderName, newRations))
		} else if oldRations > 0 && newRations <= 0 {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ex.userID,
				fmt.Sprintf("🍖❌ OUT OF RATIONS: Your force marching on [%s] has run out of rations! Combat strength will suffer until they return.", ex.defenderName))
		}
		if oldAmmo > 25.0 && newAmmo <= 25.0 {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ex.userID,
				fmt.Sprintf("🎖️ AMMUNITION RUNNING LOW: Your marching force toward [%s] is down to %.0f%% ammunition.", ex.defenderName, newAmmo))
		} else if oldAmmo > 0 && newAmmo <= 0 {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ex.userID,
				fmt.Sprintf("🎖️❌ OUT OF AMMUNITION: Your force marching on [%s] has run dry! Combat strength will suffer until they return.", ex.defenderName))
		}

		// If a force runs completely dry (rations AND ammo) before it has
		// even reached the target, it can't fight - order a forced
		// retreat rather than let it arrive and immediately eat a -50%
		// offense penalty in a battle it was never supplied to win.
		if ex.state == "marching" && newRations <= 0 && newAmmo <= 0 {
			returnResolve := time.Now().UTC().Add(time.Duration(ex.baseMarchMinutes) * time.Minute)
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'returning', resolve_time = $1 WHERE id = $2", returnResolve, ex.id)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ex.userID,
				fmt.Sprintf("↩️ FORCED RETREAT: Your force marching on [%s] ran completely out of supplies before arrival and has turned back.", ex.defenderName))
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
				var wCampID string
				_ = tx.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", w.userID).Scan(&wCampID)
				var curDollars float64
				_ = tx.QueryRowContext(ctx, "SELECT dollars FROM resources WHERE encampment_id = $1", wCampID).Scan(&curDollars)
				wCap := storagecap.CapFor(ctx, tx, wCampID)
				newDollars, _ := storagecap.Clamp(curDollars, lootWon, wCap)
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = $1 WHERE encampment_id = $2", newDollars, wCampID)

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

				var qCampID string
				_ = tx.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", q.userID).Scan(&qCampID)
				var curDollars float64
				_ = tx.QueryRowContext(ctx, "SELECT dollars FROM resources WHERE encampment_id = $1", qCampID).Scan(&curDollars)
				qCap := storagecap.CapFor(ctx, tx, qCampID)
				newDollars, _ := storagecap.Clamp(curDollars, refundDollars, qCap)
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = $1 WHERE encampment_id = $2", newDollars, qCampID)

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
		       COALESCE(r.base_march_minutes, 15.0) as base_march_minutes,
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
		id               string
		attackerID       string
		defenderID       sql.NullString
		state            string
		roundNumber      int
		attackerName     string
		attackerUserID   int64
		defenderName     string
		defenderUserID   int64
		resolveTime      time.Time
		stolenScrap      float64
		stolenMetal      float64
		stolenCrystal    float64
		baseMarchMinutes float64
		attackerRations  float64
		attackerAmmo     float64
		attackerLosses   int
		defenderLosses   int
	}

	var raids []activeRaid
	for rows.Next() {
		var r activeRaid
		err := rows.Scan(
			&r.id, &r.attackerID, &r.defenderID, &r.state, &r.roundNumber,
			&r.attackerName, &r.attackerUserID,
			&r.defenderName, &r.defenderUserID, &r.resolveTime, &r.stolenScrap,
			&r.stolenMetal, &r.stolenCrystal, &r.baseMarchMinutes,
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
			var soldiersMob, mechsMob, destroyersMob, bombersMob, bcMob, dsMob, liberatorsMob, wraithsMob int
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers_mobilized, 0), COALESCE(mechs_mobilized, 0), COALESCE(destroyers_mobilized, 0), COALESCE(bombers_mobilized, 0), COALESCE(battlecruisers_mobilized, 0), COALESCE(deathstars_mobilized, 0), COALESCE(liberators_mobilized, 0), COALESCE(wraiths_mobilized, 0) FROM raid_forces WHERE raid_id = $1", r.id).Scan(&soldiersMob, &mechsMob, &destroyersMob, &bombersMob, &bcMob, &dsMob, &liberatorsMob, &wraithsMob)

			_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2, destroyers = destroyers + $3, bombers = bombers + $4, battlecruisers = battlecruisers + $6, deathstars = deathstars + $7, liberators = liberators + $8, wraiths = wraiths + $9 WHERE encampment_id = $5", soldiersMob, mechsMob, destroyersMob, bombersMob, r.attackerID, bcMob, dsMob, liberatorsMob, wraithsMob)
			var curScrap, curMetal, curCrystal float64
			_ = tx.QueryRowContext(ctx, "SELECT scrap, metal, crystal FROM resources WHERE encampment_id = $1", r.attackerID).Scan(&curScrap, &curMetal, &curCrystal)
			attackerCap := storagecap.CapFor(ctx, tx, r.attackerID)
			newScrap, _ := storagecap.Clamp(curScrap, r.stolenScrap, attackerCap)
			newMetal, _ := storagecap.Clamp(curMetal, r.stolenMetal, attackerCap)
			newCrystal, _ := storagecap.Clamp(curCrystal, r.stolenCrystal, attackerCap)
			_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = $1, metal = $2, crystal = $3 WHERE encampment_id = $4", newScrap, newMetal, newCrystal, r.attackerID)
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

			if r.defenderID.Valid && r.defenderUserID != 0 {
				defenderEngagedAlert := fmt.Sprintf(
					"⚔️ UNDER ATTACK: ENGAGEMENT ACTIVATED!\n\n"+
						"Hostile forces from Outpost [%s] have arrived and are actively engaging your defenses!\n"+
						"Decisive Resolution progress starting next tick cycle.",
					r.attackerName,
				)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.defenderUserID, defenderEngagedAlert)
			}
			continue
		}

		var primarySoldiers, primaryMechs, primaryBuggies, primaryDestroyers, primaryBombers, primaryBC, primaryDS, primaryLiberators, primaryWraiths int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers_mobilized, 0), COALESCE(mechs_mobilized, 0), COALESCE(buggies_mobilized, 0), COALESCE(destroyers_mobilized, 0), COALESCE(bombers_mobilized, 0), COALESCE(battlecruisers_mobilized, 0), COALESCE(deathstars_mobilized, 0), COALESCE(liberators_mobilized, 0), COALESCE(wraiths_mobilized, 0) FROM raid_forces WHERE raid_id = $1 FOR UPDATE", r.id).Scan(&primarySoldiers, &primaryMechs, &primaryBuggies, &primaryDestroyers, &primaryBombers, &primaryBC, &primaryDS, &primaryLiberators, &primaryWraiths)

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

		// Destroyers, Bombers, Liberators and Wraiths aren't yet draftable
		// into co-op lobbies (only the raid creator's primary force can
		// bring them), so no helper contribution to add here - just the
		// primary force.
		totDestroyers := primaryDestroyers
		totBombers := primaryBombers
		totBC := primaryBC
		totDS := primaryDS
		totLiberators := primaryLiberators
		totWraiths := primaryWraiths

		attackForce := totSoldiers + totMechs + totDestroyers + totBombers + totBC + totDS + totLiberators + totWraiths

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
		var defenderLightLaserLvl, defenderHeavyLaserLvl, defenderGaussCannonLvl, defenderIonCannonLvl, defenderPlasmaTurretLvl int
		var defenderGuardians, defenderObservers int
		var defenderHeroSuperpower string
		var soldiersDefender, dronesDefender, jetsDefender, mechsDefender, scoutsDefender int
		var defenderOrbitalBuffActive bool

		if !r.defenderID.Valid {
			var attackerCoreLvl int
			_ = tx.QueryRowContext(ctx, "SELECT level FROM encampments WHERE id = $1", r.attackerID).Scan(&attackerCoreLvl)
			if attackerCoreLvl <= 0 {
				attackerCoreLvl = 1
			}
			nest := content.RogueNestComposition(attackerCoreLvl)
			soldiersDefender = nest.Soldiers
			mechsDefender = nest.Mechs
			dronesDefender = nest.Drones
			jetsDefender = nest.Jets
			defLevel = attackerCoreLvl

			// Phase 7 AI scaling: the Nest now feeds the exact same
			// per-subsystem fields a real defender does (Defense Grid,
			// Guardians/Observers, research, shields, hero superpower)
			// instead of a single flat fallback bonus, so it resolves
			// through the identical combat code path below as any
			// player base.
			defenderTurretLevels = int(nest.TurretBonus / 0.08)
			defenderLightLaserLvl = nest.LightLaserLvl
			defenderHeavyLaserLvl = nest.HeavyLaserLvl
			defenderGaussCannonLvl = nest.GaussCannonLvl
			defenderIonCannonLvl = nest.IonCannonLvl
			defenderPlasmaTurretLvl = nest.PlasmaTurretLvl
			defenderGuardians = nest.Guardians
			defenderObservers = nest.Observers
			defenderIntegrityTechLvl = nest.IntegrityTechLvl
			defenderShields = nest.Shields
			defenderHeroSuperpower = nest.HeroSuperpower
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

			// Phase 6 "engage-weapon" mechanics: pull each turret type's
			// level individually so each one can apply its own situational
			// bonus below, instead of contributing to just one flat sum.
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'light_laser'", r.defenderID.String).Scan(&defenderLightLaserLvl)
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'heavy_laser'", r.defenderID.String).Scan(&defenderHeavyLaserLvl)
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'gauss_cannon'", r.defenderID.String).Scan(&defenderGaussCannonLvl)
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'ion_cannon'", r.defenderID.String).Scan(&defenderIonCannonLvl)
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'plasma_turret'", r.defenderID.String).Scan(&defenderPlasmaTurretLvl)

			// Guardian/Observer: garrison-only units that never leave the
			// base, so they're read straight from workshop_inventory rather
			// than mobilized like Liberators/Wraiths.
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(guardians, 0), COALESCE(observers, 0) FROM workshop_inventory WHERE encampment_id = $1", r.defenderID.String).Scan(&defenderGuardians, &defenderObservers)

			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(superpower, '') FROM heroes WHERE encampment_id = $1", r.defenderID.String).Scan(&defenderHeroSuperpower)
			_ = tx.QueryRowContext(ctx, "SELECT (orbital_buff_until IS NOT NULL AND orbital_buff_until > CURRENT_TIMESTAMP) FROM encampments WHERE id = $1", r.defenderID.String).Scan(&defenderOrbitalBuffActive)
		}

		defenseForce := soldiersDefender + dronesDefender + jetsDefender + mechsDefender

		// Phase 7 (item 12): world events are now per-continent. Scope
		// this to the attacker's own region - always present (unlike
		// defenderID, which is null for AI/rogue-nest raids) and it's
		// the attacking fleet's own systems (targeting, mech corrosion)
		// executing the operation.
		var attackerRegion string
		_ = tx.QueryRowContext(ctx, "SELECT c.region FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", r.attackerID).Scan(&attackerRegion)
		activeWeather := world.ActiveEventFor(ctx, tx, attackerRegion)

		orbitalBonus := 0.0
		if defenderOrbitalBuffActive {
			orbitalBonus = 0.30 // Orbital Maneuver: +30% defense rating while active
		}

		offenseRatingModifier := 1.0

		// Phase 6 "engage-weapon" mechanics: each of the 6 turret types now
		// contributes its own differentiated slice of defense rating
		// instead of one flat sum, and each situationally scales up
		// against the attacker composition it's meant to counter. Rogue
		// AI nests (no real defenderID) don't have individually-typed
		// turrets, so they fall back to the original flat aggregate.
		// Phase 7: the Rogue Nest now carries individually-typed turret
		// levels of its own (content.RogueNestComposition), so it runs
		// through the exact same per-turret-type formula as a real
		// player defender - no more flat-sum fallback branch.
		lightLaserBonus := float64(defenderLightLaserLvl) * 0.02 // cheap, flat anti-infantry baseline
		heavyLaserBonus := float64(defenderHeavyLaserLvl) * 0.03 * (1.0 + math.Min(float64(totSoldiers)*0.005, 0.5))
		gaussCannonBonus := float64(defenderGaussCannonLvl) * 0.05 * (1.0 + math.Min(float64(totMechs+totLiberators)*0.02, 1.0))
		ionCannonBonus := float64(defenderIonCannonLvl) * 0.05 * (1.0 + math.Min(float64(totDestroyers+totWraiths)*0.03, 1.0))
		plasmaTurretBonus := float64(defenderPlasmaTurretLvl) * 0.08 // top-tier, no counter needed
		turretDefenseBonus := lightLaserBonus + heavyLaserBonus + gaussCannonBonus + ionCannonBonus + plasmaTurretBonus

		// Wraith: stealth strike fighter. Its cloaking field partially
		// blinds the target's Defense Grid before the engagement,
		// shaving a chunk off the combined turret bonus.
		if totWraiths > 0 {
			turretDefenseBonus *= 1.0 - math.Min(float64(totWraiths)*0.03, 0.40)
		}

		// Guardian: garrison-only heavy defense walker, adds directly to
		// defense rating. Observer: garrison-only recon satellite, a
		// smaller early-warning-flavored defense bonus (mirrors Scouts).
		guardianBonus := math.Min(float64(defenderGuardians)*0.03, 1.0)
		observerBonus := math.Min(float64(defenderObservers)*0.01, 0.15)

		defenseRatingModifier := 1.0 + (float64(defLevel) * 0.15) + turretDefenseBonus + guardianBonus + observerBonus + math.Min(float64(scoutsDefender)*0.01, 0.20) + orbitalBonus

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
		case "emp":
			// EMP hits electronics-dependent systems hard - a flat, non-random offense penalty.
			offenseRatingModifier *= 0.60
			weatherNotice = "\n⚠️ EMP ACTIVE: Electromagnetic pulse degraded fleet targeting systems! Offense rating reduced by 40%."
		case "sandstorm":
			// Sandstorm degrades visibility/targeting - similar shape to Solar Flare's variance, but a straight reduction rather than a swing.
			offenseRatingModifier *= 0.85
			weatherNotice = "\n⚠️ SANDSTORM ACTIVE: Reduced visibility hampered target acquisition! Offense rating reduced by 15%."
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
		// Guardian is Bomber's own hard counter: the more Guardians a
		// defender fields, the less a Bomber-heavy raid gains from this.
		if totBombers > 0 {
			bomberBonus := 1.0 + math.Min(float64(defenderTurretLevels)*0.05, 1.5)
			guardianCounter := 1.0 - math.Min(float64(defenderGuardians)*0.03, 0.60)
			attRating += float64(totBombers) * 18.0 * offenseRatingModifier * bomberBonus * guardianCounter
		}

		// Liberator: mid-tier capital gunship, flat attack rating with no
		// situational bonus - the accessible stepping stone between
		// Bombers and the Battlecruiser.
		if totLiberators > 0 {
			libUnit := content.MustFindUnit("liberator")
			attRating += float64(totLiberators) * libUnit.AttackRating * offenseRatingModifier
		}

		// Wraith: stealth anti-air fighter. Rating scales up the more
		// drones/jets the defender is fielding, same hard-counter shape
		// as Destroyer, on top of the Defense-Grid-blinding effect it
		// already applied to turretDefenseBonus above.
		if totWraiths > 0 {
			wrUnit := content.MustFindUnit("wraith")
			wraithTargets := dronesDefender + jetsDefender
			wraithBonus := 1.0 + math.Min(float64(wraithTargets)*0.02, 1.0)
			attRating += float64(totWraiths) * wrUnit.AttackRating * offenseRatingModifier * wraithBonus
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

		// Toughness-weighted specialist casualties. The OLD model split
		// dbCas purely by headcount, which meant a lone Deathstar (the
		// only specialist unit in the fleet) absorbed 100% of ANY
		// specialist-pool casualty - i.e. even a single stray loss killed
		// it outright, making the most expensive unit in the game the
		// easiest to lose. Toughness now scales down how many of those
		// casualties actually translate into a destroyed unit at all
		// (representing armor/hull soaking hits), before splitting
		// whatever's left by headcount among the pool.
		const (
			destroyerToughness = 4.0
			bomberToughness    = 4.0
			bcToughness        = 20.0
			dsToughness        = 200.0 // bumped alongside the Phase 7 attack rating rebalance (was 150.0)
			liberatorToughness = 8.0
			wraithToughness    = 3.0
		)

		var lostAttDestroyers, lostAttBombers, lostAttBC, lostAttDS, lostAttLiberators, lostAttWraiths int
		specialistPool := totDestroyers + totBombers + totBC + totDS + totLiberators + totWraiths
		if specialistPool > 0 {
			weightedToughness := (float64(totDestroyers)*destroyerToughness +
				float64(totBombers)*bomberToughness +
				float64(totBC)*bcToughness +
				float64(totDS)*dsToughness +
				float64(totLiberators)*liberatorToughness +
				float64(totWraiths)*wraithToughness) / float64(specialistPool)

			effectiveDbCas := int(float64(dbCas) / weightedToughness)

			lostAttDestroyers = int(float64(effectiveDbCas) * float64(totDestroyers) / float64(specialistPool))
			lostAttBombers = int(float64(effectiveDbCas) * float64(totBombers) / float64(specialistPool))
			lostAttBC = int(float64(effectiveDbCas) * float64(totBC) / float64(specialistPool))
			lostAttLiberators = int(float64(effectiveDbCas) * float64(totLiberators) / float64(specialistPool))
			lostAttWraiths = int(float64(effectiveDbCas) * float64(totWraiths) / float64(specialistPool))
			lostAttDS = effectiveDbCas - lostAttDestroyers - lostAttBombers - lostAttBC - lostAttLiberators - lostAttWraiths
			if lostAttDestroyers > primaryDestroyers {
				lostAttDestroyers = primaryDestroyers
			}
			if lostAttBombers > primaryBombers {
				lostAttBombers = primaryBombers
			}
			if lostAttBC > primaryBC {
				lostAttBC = primaryBC
			}
			if lostAttLiberators > primaryLiberators {
				lostAttLiberators = primaryLiberators
			}
			if lostAttWraiths > primaryWraiths {
				lostAttWraiths = primaryWraiths
			}
			if lostAttDS < 0 {
				lostAttDS = 0
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
		newAttLiberators := primaryLiberators - lostAttLiberators
		newAttWraiths := primaryWraiths - lostAttWraiths

		_, _ = tx.ExecContext(ctx, "UPDATE raid_forces SET soldiers_mobilized = $1, mechs_mobilized = $2, destroyers_mobilized = $4, bombers_mobilized = $5, battlecruisers_mobilized = $6, deathstars_mobilized = $7, liberators_mobilized = $8, wraiths_mobilized = $9 WHERE raid_id = $3", newAttSols, newAttMechs, r.id, newAttDestroyers, newAttBombers, newAttBC, newAttDS, newAttLiberators, newAttWraiths)

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

		var smallShieldLvl, largeShieldLvl int
		if r.defenderID.Valid {
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'small_shield'", r.defenderID.String).Scan(&smallShieldLvl)
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'large_shield'", r.defenderID.String).Scan(&largeShieldLvl)
		}

		lootPercentage := 0.40
		if defenderShields > 0 {
			lootPercentage = 0.20
		}
		shieldReduction := math.Min(float64(smallShieldLvl)*0.02+float64(largeShieldLvl)*0.05, 0.30)
		lootPercentage *= (1.0 - shieldReduction)
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
				{Emoji: "🦅", Label: "Liberators", Count: totLiberators},
				{Emoji: "👻", Label: "Wraiths", Count: totWraiths},
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
				{Emoji: "🦅", Label: "Liberators", Count: lostAttLiberators},
				{Emoji: "👻", Label: "Wraiths", Count: lostAttWraiths},
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
				fmt.Sprintf("🔩 %.0f Metal", primaryMetalShare),
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
			if primaryBuggies > 0 {
				// Total wipeout of combat units, but Buggies aren't part of
				// the combat casualty pool - if any were committed, they
				// survive the rout and can still scavenge a little before
				// retreating. Nowhere near a real win's take, but not
				// nothing either - matches how transport crews would
				// realistically grab whatever's within reach and flee.
				const salvagePercent = 0.08
				salvageScrap := defenderScrap * salvagePercent
				salvageMetal := defenderMetal * salvagePercent
				salvageCrystal := defenderCrystal * salvagePercent

				returnMinutes := r.baseMarchMinutes
				resolveTime := time.Now().UTC().Add(time.Duration(returnMinutes) * time.Minute)

				_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'returning', stolen_scrap = $1, stolen_metal = $2, stolen_crystal = $3, resolve_time = $4 WHERE id = $5",
					salvageScrap, salvageMetal, salvageCrystal, resolveTime, r.id)

				if r.defenderID.Valid {
					_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = GREATEST(scrap - $1, 0), metal = GREATEST(metal - $2, 0), crystal = GREATEST(crystal - $3, 0) WHERE encampment_id = $4",
						salvageScrap, salvageMetal, salvageCrystal, r.defenderID.String)
				}

				salvageAlert := fmt.Sprintf(
					"💀 ASSAULT FORCE WIPED OUT - BUT NOT EMPTY-HANDED\n\n"+
						"Your combat units were destroyed, but %d surviving Buggy crew(s) grabbed what they could before retreating.\n"+
						"⚙️ Salvaged: %.0f Scrap, %.0f Metal, %.0f Crystal.\n"+
						"⏳ Return march engaged, arriving in %.0f minutes.",
					primaryBuggies, salvageScrap, salvageMetal, salvageCrystal, returnMinutes,
				)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.attackerUserID, salvageAlert)
			} else {
				_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'completed' WHERE id = $1", r.id)
			}
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

			var cargoMk1, cargoMk2, cargoMk3 int
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(cargo_mk1, 0), COALESCE(cargo_mk2, 0), COALESCE(cargo_mk3, 0) FROM workshop_inventory WHERE encampment_id = $1", r.attackerID).Scan(&cargoMk1, &cargoMk2, &cargoMk3)

			weightFactor := (primaryShare + primaryMetalShare + primaryCrystalShare) / 5000.0
			if haulers > 0 {
				weightFactor *= 0.50
			}
			// Cargo Ship tiers stack on top of a Hauler's base reduction,
			// each tier cutting further into the return-march loot penalty.
			if cargoMk1 > 0 {
				weightFactor *= 0.90
			}
			if cargoMk2 > 0 {
				weightFactor *= 0.80
			}
			if cargoMk3 > 0 {
				weightFactor *= 0.65
			}

			// The return trip is anchored to the SAME distance as the
			// outbound march (r.baseMarchMinutes), not some disconnected
			// flat number - carrying looted resources back only ever adds
			// time on top of that, it never makes the trip home shorter
			// than the trip out.
			returnMinutes := r.baseMarchMinutes * (1.0 + weightFactor)
			returnDuration := time.Duration(returnMinutes) * time.Minute
			resolveTime := time.Now().UTC().Add(returnDuration)

			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'returning', stolen_scrap = $1, stolen_metal = $4, stolen_crystal = $5, resolve_time = $2 WHERE id = $3", primaryShare, resolveTime, r.id, primaryMetalShare, primaryCrystalShare)

			etaAlert := fmt.Sprintf(
				"🚚 SALVAGE COMPLETE, RETURN MARCH ENGAGED\n\n"+
					"Carrying ⚙️ %.0f Scrap, 🔩 %.0f Metal, 💎 %.0f Crystal home.\n"+
					"⏳ ETA: %.0f minutes (outbound trip was %.0f minutes; extra weight from the loot adds travel time).",
				primaryShare, primaryMetalShare, primaryCrystalShare, returnMinutes, r.baseMarchMinutes,
			)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.attackerUserID, etaAlert)

			if r.defenderID.Valid {
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = GREATEST(scrap - $1, 0), metal = GREATEST(metal - $2, 0), crystal = GREATEST(crystal - $3, 0) WHERE encampment_id = $4", stolenScrap, stolenMetal, stolenCrystal, r.defenderID.String)

				// Clan War score: if the attacker and defender belong to
				// two clans with an active war between them, this victory
				// counts toward the attacker's clan war score - real
				// combat outcomes drive the war, not a one-time stub.
				e.applyClanWarScore(ctx, tx, r.attackerID, r.defenderID.String, primaryShare+primaryMetalShare+primaryCrystalShare)
			}

			for _, h := range helpers {
				_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2 WHERE encampment_id = $3", h.soldiers, h.mechs, h.encampment_id)
			}
		} else if r.roundNumber >= 5 {
			returnMinutes := r.baseMarchMinutes
			resolveTime := time.Now().UTC().Add(time.Duration(returnMinutes) * time.Minute)

			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'returning', stolen_scrap = 0, stolen_metal = 0, stolen_crystal = 0, resolve_time = $1 WHERE id = $2", resolveTime, r.id)

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
