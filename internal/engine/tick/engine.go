package tick

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"math/rand"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/engine/agent"
	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
	"github.com/NomadDigita/The-Vagabond/internal/engine/resource"
	"github.com/NomadDigita/The-Vagabond/internal/engine/starvation"
	"github.com/NomadDigita/The-Vagabond/internal/engine/world"
	"github.com/NomadDigita/The-Vagabond/internal/game/battlereport"
	"github.com/NomadDigita/The-Vagabond/internal/game/combatmath"
	"github.com/NomadDigita/The-Vagabond/internal/game/content"
	"github.com/NomadDigita/The-Vagabond/internal/game/gear"
	"github.com/NomadDigita/The-Vagabond/internal/game/progression"
	"github.com/NomadDigita/The-Vagabond/internal/game/roadcombat"
	"github.com/NomadDigita/The-Vagabond/internal/game/scoring"
	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
	"github.com/NomadDigita/The-Vagabond/internal/game/worldintel"
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

	lastTickMu   sync.RWMutex
	lastTickAt   time.Time
	lastTickDone time.Time
}

const defaultTickInterval = 60 * time.Second

func NewEngine(db *sql.DB, interval time.Duration) *Engine {
	if interval <= 0 {
		log.Printf("Tick Engine: invalid interval %v (GAME_TICK_SECONDS must be a positive integer number of seconds) — falling back to %v", interval, defaultTickInterval)
		interval = defaultTickInterval
	}
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
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("Tick Engine: top-level PANIC (recovered), engine stays alive: %v\n%s", r, debug.Stack())
						}
					}()
					e.ProcessTick()
				}()
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

func (e *Engine) LastTickStatus() (startedAt, finishedAt time.Time) {
	e.lastTickMu.RLock()
	defer e.lastTickMu.RUnlock()
	return e.lastTickAt, e.lastTickDone
}

type tickPhase struct {
	name string
	run  func(ctx context.Context, tx *sql.Tx) error
}

func (e *Engine) runPhase(ctx context.Context, p tickPhase) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Tick phase [%s] PANIC (recovered): %v\n%s", p.name, r, debug.Stack())
		}
	}()

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

func (e *Engine) collectDailyTax(ctx context.Context, tx *sql.Tx) error {
	var lastCollected time.Time
	var taxRate int
	err := tx.QueryRowContext(ctx, "SELECT tax_rate_percent, last_collected_at FROM tax_law WHERE id = 1 FOR UPDATE").Scan(&taxRate, &lastCollected)
	if err != nil {
		return err
	}

	if time.Now().UTC().Sub(lastCollected) < 24*time.Hour {
		return nil
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
							"💰🏆 %s\n\nAs a Top-3 ranked survivor, you received 💵 %s from the Wasteland Tax Law (%s rate) collected from all players!",
							htmlBoldTick("DAILY TAX PAYOUT!"), htmlCodeTick(fmt.Sprintf("$%.2f", share)), htmlCodeTick(fmt.Sprintf("%d%%", taxRate)),
						)
						if isRealPlayer(w.userID) {
							_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", w.userID, alertMsg)
						}
					}
				}
			}
		}
	}

	_, err = tx.ExecContext(ctx, "UPDATE tax_law SET last_collected_at = CURRENT_TIMESTAMP WHERE id = 1")
	return err
}

func (e *Engine) resolveExplorationDispatches(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT d.id, d.site_id, d.encampment_id, d.user_id, s.site_name, s.reward_type, s.reward_amount, s.continent
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
		continent    string
	}
	var dispatches []resolvedDispatch
	for rows.Next() {
		var d resolvedDispatch
		if err := rows.Scan(&d.id, &d.siteID, &d.encampmentID, &d.userID, &d.siteName, &d.rewardType, &d.rewardAmount, &d.continent); err == nil {
			dispatches = append(dispatches, d)
		}
	}
	rows.Close()

	for _, d := range dispatches {
		storageCap := storagecap.CapFor(ctx, tx, d.encampmentID)

		var current float64
		column := d.rewardType
		_ = tx.QueryRowContext(ctx, fmt.Sprintf("SELECT %s FROM resources WHERE encampment_id = $1 FOR UPDATE", column), d.encampmentID).Scan(&current)
		newAmount, _ := storagecap.Clamp(current, d.rewardAmount, storageCap)
		_, err := tx.ExecContext(ctx, fmt.Sprintf("UPDATE resources SET %s = $1 WHERE encampment_id = $2", column), newAmount, d.encampmentID)
		if err != nil {
			log.Printf("Failed crediting exploration reward for dispatch %s: %v", d.id, err)
			continue
		}

		_, _ = tx.ExecContext(ctx, "UPDATE exploration_sites SET claimed_by = $1, claimed_at = CURRENT_TIMESTAMP WHERE id = $2", d.encampmentID, d.siteID)
		_, _ = tx.ExecContext(ctx, "DELETE FROM exploration_dispatches WHERE id = $1", d.id)

		discoveryNote, discoveryErr := e.resolveExplorationDiscovery(ctx, tx, d.encampmentID, d.continent)
		if discoveryErr != nil {
			log.Printf("Failed resolving world discovery for exploration dispatch %s: %v", d.id, discoveryErr)
		}

		message := fmt.Sprintf("🧭✅ %s: Your outpost has claimed %s! +%s %s credited to your reserves.",
			htmlBoldTick("EXPEDITION SUCCESSFUL"), htmlCodeTick(d.siteName), htmlCodeTick(fmt.Sprintf("%.0f", d.rewardAmount)), d.rewardType)
		if discoveryNote != "" {
			message += "\n\n" + discoveryNote
		}
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent, effect_id) VALUES ($1, $2, FALSE, $3)", d.userID, message, notifications.EffectCelebration)
	}
	return nil
}

func (e *Engine) resolveExplorationDiscovery(ctx context.Context, tx *sql.Tx, observerCampID string, continent string) (string, error) {
	var scouts int
	if err := tx.QueryRowContext(ctx, "SELECT COALESCE(scouts, 0) FROM workshop_inventory WHERE encampment_id = $1", observerCampID).Scan(&scouts); err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("reading exploration scouts: %w", err)
	}
	if rand.Float64() >= worldintel.ExplorationDiscoveryChance(scouts) {
		return "", nil
	}

	var targetCampID, targetName, targetOwner string
	var targetIsDefeated bool
	var targetEpitaph sql.NullString
	err := tx.QueryRowContext(ctx, `
		SELECT e.id, e.name, u.first_name, e.is_defeated, e.epitaph
		FROM encampments e
		JOIN users u ON u.telegram_id = e.user_id
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE e.id <> $1
		  AND c.region = $2
		  AND NOT EXISTS (
				SELECT 1 FROM encampment_discoveries d
				WHERE d.observer_encampment_id = $1 AND d.target_encampment_id = e.id
			)
		ORDER BY c.danger_level ASC, e.established_at ASC
		LIMIT 1`, observerCampID, continent).Scan(&targetCampID, &targetName, &targetOwner, &targetIsDefeated, &targetEpitaph)
	if err == nil {
		result, err := tx.ExecContext(ctx, `
			INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method)
			VALUES ($1, $2, 'exploration') ON CONFLICT DO NOTHING`, observerCampID, targetCampID)
		if err != nil {
			return "", fmt.Errorf("recording discovered encampment: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected > 0 {
			if targetIsDefeated {
				epitaphText := "Nothing more is known of its fall."
				if targetEpitaph.Valid && targetEpitaph.String != "" {
					epitaphText = targetEpitaph.String
				}
				return fmt.Sprintf("🪦 RUINS DISCOVERED: Your survey team finds only the fallen remains of %s. %s", targetName, epitaphText), nil
			}
			return fmt.Sprintf("📡 FIRST CONTACT: Your survey team discovered Outpost [%s], commanded by %s. It is now available in the Tactical Target Matrix.", targetName, targetOwner), nil
		}
		return "", nil
	}
	if err != sql.ErrNoRows {
		return "", fmt.Errorf("selecting undiscovered encampment: %w", err)
	}

	result, err := tx.ExecContext(ctx, `
		INSERT INTO encampment_discoveries (observer_encampment_id, target_key, discovery_method)
		VALUES ($1, $2, 'exploration') ON CONFLICT DO NOTHING`, observerCampID, worldintel.RogueDroneNestKey)
	if err != nil {
		return "", fmt.Errorf("recording discovered AI contact: %w", err)
	}
	if affected, _ := result.RowsAffected(); affected > 0 {
		return "📡 FIRST CONTACT: Your survey team triangulated a Rogue Drone Nest. Recon and skirmish controls are now available in the Tactical Target Matrix.", nil
	}
	return "", nil
}

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
			JOIN encampment_discoveries d ON d.target_encampment_id = e.id
			WHERE d.observer_encampment_id = $1
			  AND e.id != $1
			ORDER BY RANDOM()
			LIMIT 1`, s.campID).Scan(&targetName, &targetOwnerName, &targetX, &targetY, &targetScrap)
		if err != nil {
			continue
		}

		reportMsg := fmt.Sprintf(
			"📡🔄 %s\n\n🎯 Rival Detected: %s (Commander: %s)\n📍 Coordinates: %s\n♻️ Estimated Scrap: %s\n\n💡 Use %s for a full manual intel report.",
			htmlBoldTick("AUTOMATIC SCAN SWEEP COMPLETE"), htmlCodeTick(htmlEscapeTick(targetName)), htmlCodeTick(htmlEscapeTick(targetOwnerName)),
			htmlCodeTick(fmt.Sprintf("%d, %d", targetX, targetY)), htmlCodeTick(fmt.Sprintf("%.0f", targetScrap)),
			htmlCodeTick(fmt.Sprintf("/scout %s", htmlEscapeTick(targetOwnerName))),
		)
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", s.userID, reportMsg)
	}

	return nil
}

func (e *Engine) resolveWorldBossAttacks(ctx context.Context, tx *sql.Tx) error {
	marchingRows, err := tx.QueryContext(ctx, `
		SELECT a.id, a.boss_id, a.user_id, a.encampment_id, a.soldiers_committed, a.mechs_committed, a.march_minutes,
		       b.name, b.emoji, b.current_hp, b.max_hp, b.loot_pool_dollars, b.retaliation_rating, b.is_gauntlet, b.enrage_threshold_pct
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
		isGauntlet                     bool
		enrageThresholdPct             float64
	}
	var marching []marchingAttack
	for marchingRows.Next() {
		var m marchingAttack
		if scanErr := marchingRows.Scan(&m.id, &m.bossID, &m.userID, &m.encampmentID, &m.soldiers, &m.mechs, &m.marchMinutes,
			&m.bossName, &m.bossEmoji, &m.bossCurHP, &m.bossMaxHP, &m.lootPool, &m.retaliation, &m.isGauntlet, &m.enrageThresholdPct); scanErr == nil {
			marching = append(marching, m)
		}
	}
	marchingRows.Close()

	for _, m := range marching {
		if m.bossCurHP <= 0 {
			_, _ = tx.ExecContext(ctx, "UPDATE world_boss_attacks SET state = 'returning', resolve_time = CURRENT_TIMESTAMP + ($1 * INTERVAL '1 minute') WHERE id = $2", m.marchMinutes, m.id)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", m.userID,
				fmt.Sprintf("🏳️ %s was already defeated by another survivor before your forces arrived. Turning back - no fight, no losses.", htmlBoldTick(m.bossName)))
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

		effectiveRetaliation := m.retaliation
		if m.isGauntlet && m.enrageThresholdPct > 0 && m.bossMaxHP > 0 && (m.bossCurHP/m.bossMaxHP*100.0) <= m.enrageThresholdPct {
			effectiveRetaliation *= 2.0
		}
		totalCommitted := m.soldiers + m.mechs
		casualties := int(float64(totalCommitted) * effectiveRetaliation / 100.0)
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
			if err := e.payoutWorldBossLoot(ctx, tx, m.bossID, m.bossName, m.lootPool, m.isGauntlet); err != nil {
				log.Printf("World Boss loot payout failed: %v", err)
			}
		}

		_, _ = tx.ExecContext(ctx, `
			UPDATE world_boss_attacks 
			SET state = 'returning', soldiers_committed = $1, mechs_committed = $2, 
			    resolve_time = CURRENT_TIMESTAMP + ($3 * INTERVAL '1 minute')
			WHERE id = $4`,
			survivingSoldiers, survivingMechs, m.marchMinutes, m.id)
	}

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
				fmt.Sprintf("🚚 %s: 🪖 %s Soldiers, 🤖 %s Mechs made it back from the World Boss engagement.",
					htmlBoldTick("SURVIVORS RETURNED HOME"), htmlCodeTick(fmt.Sprintf("%d", r.soldiers)), htmlCodeTick(fmt.Sprintf("%d", r.mechs))))
		} else {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.userID,
				"💀 No survivors returned from that World Boss engagement.")
		}
	}

	return nil
}

func (e *Engine) payoutWorldBossLoot(ctx context.Context, tx *sql.Tx, bossID, bossName string, lootPool float64, isGauntlet bool) error {
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
			alertMsg := fmt.Sprintf("☠️🎉 <b>BOSS SLAIN: %s</b>\n\nYour cumulative <code>%.0f damage</code> earned you 💵 <code>$%.2f</code> from the loot pool.", htmlEscapeTick(bossName), ct.damage, share)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent, effect_id) VALUES ($1, $2, FALSE, $3)", ct.userID, alertMsg, notifications.EffectCelebration)

			if isGauntlet {
				dropRoll := rand.Float64()
				if dropRoll < 0.30 {
					_, _ = gear.GrantRandomDrop(ctx, tx, ct.campID, gear.RarityLegendary)
				} else {
					_, _ = gear.GrantRandomDrop(ctx, tx, ct.campID, gear.RarityEpic)
				}
			} else {
				roll := rand.Float64()
				switch {
				case roll < 0.02:
					_, _ = gear.GrantRandomDrop(ctx, tx, ct.campID, gear.RarityLegendary)
				case roll < 0.10:
					_, _ = gear.GrantRandomDrop(ctx, tx, ct.campID, gear.RarityEpic)
				}
			}
		}
	}

	_, _ = tx.ExecContext(ctx, "DELETE FROM world_boss_contributions WHERE boss_id = $1", bossID)
	_, _ = tx.ExecContext(ctx, `
		UPDATE world_bosses 
		SET current_hp = max_hp, loot_pool_dollars = loot_pool_dollars * 1.10, last_defeated_at = CURRENT_TIMESTAMP, total_kills = total_kills + 1
		WHERE id = $1`, bossID)

	return nil
}

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
		return
	}

	if isClanA {
		_, _ = tx.ExecContext(ctx, "UPDATE clan_wars SET score_a = score_a + $1 WHERE id = $2", lootValue, warID)
	} else {
		_, _ = tx.ExecContext(ctx, "UPDATE clan_wars SET score_b = score_b + $1 WHERE id = $2", lootValue, warID)
	}
}

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

	const spoilsPool = 10000.0

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
			drawMsg := fmt.Sprintf("🤝 %s\n\n%s 🆚 %s: <code>%.0f - %.0f</code>. Neither side prevailed.",
				htmlBoldTick("CLAN WAR ENDED IN A DRAW!"), htmlBoldTick(htmlEscapeTick(winnerName)), htmlBoldTick(htmlEscapeTick(loserName)), winnerScore, loserScore)
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

			winMsg := fmt.Sprintf("🏆⚔️ %s\n\nYour Clan defeated %s! Final score: <code>%.0f - %.0f</code>.\n💵 Your share of the spoils: <code>$%.2f</code>",
				htmlBoldTick("CLAN WAR VICTORY!"), htmlBoldTick(htmlEscapeTick(loserName)), winnerScore, loserScore, share)
			for _, uid := range winnerMembers {
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent, effect_id) VALUES ($1, $2, FALSE, $3)", uid, winMsg, notifications.EffectCelebration)
			}
		}

		loseMsg := fmt.Sprintf("💀⚔️ %s\n\nYour Clan lost to %s. Final score: <code>%.0f - %.0f</code>.",
			htmlBoldTick("CLAN WAR DEFEAT"), htmlBoldTick(htmlEscapeTick(winnerName)), winnerScore, loserScore)
		e.notifyClanMembers(ctx, tx, loserClanID, loseMsg, notifications.EffectPoop)
	}

	return nil
}

func (e *Engine) notifyClanMembers(ctx context.Context, tx *sql.Tx, clanID, message string, effectID ...string) {
	rows, err := tx.QueryContext(ctx, "SELECT user_id FROM user_clans WHERE clan_id = $1", clanID)
	if err != nil {
		return
	}
	defer rows.Close()
	var effect string
	if len(effectID) > 0 {
		effect = effectID[0]
	}
	for rows.Next() {
		var uid int64
		if rows.Scan(&uid) == nil {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent, effect_id) VALUES ($1, $2, FALSE, NULLIF($3, ''))", uid, message, effect)
		}
	}
}

func (e *Engine) ProcessTick() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Tick Engine: ProcessTick PANIC (recovered): %v\n%s", r, debug.Stack())
		}
	}()

	start := time.Now()
	log.Println("⌛ Processing master game tick pass...")

	e.lastTickMu.Lock()
	e.lastTickAt = start.UTC()
	e.lastTickMu.Unlock()

	ctx := context.Background()

	phases := []tickPhase{
		{"weather", func(ctx context.Context, tx *sql.Tx) error { return e.weatherEngine.RunWeatherPass(ctx, tx) }},
		{"resources", func(ctx context.Context, tx *sql.Tx) error { return e.resourceProcessor.RunResourcePass(ctx, tx) }},
		{"agents", func(ctx context.Context, tx *sql.Tx) error { return e.agentProcessor.RunAgentPass(ctx, tx) }},
		{"logistics_consumption", e.applyActiveLogisticsConsumption},
		{"route_recovery", e.resumeRouteRecoveries},
		{"route_discovery", e.discoverRouteContacts},
		{"road_encounters", e.evaluateRoadEncounters},
		{"road_base_encounters", e.evaluateRoadBaseEncounters},
		{"route_weather_incidents", e.evaluateRouteWeatherIncidents},
		{"supply_convoys", e.processSupplyConvoys},
		{"stalled_columns", e.resolveStalledColumns},
		{"ai_civilization_spawn", e.spawnNewAIFactions},
		{"ai_civilization_growth", e.growAICivilizations},
		{"ai_civilization_jobs", e.runAIJobs},
		{"ai_civilization_decisions", e.decideAIFactionActions},
		{"arena_matchmaking", e.processArenaMatchmaking},
		{"espionage", e.resolvePendingEspionageMissions},
		{"mining", e.resolveCompletedMiningQueues},
		{"starvation", func(ctx context.Context, tx *sql.Tx) error { return e.starvationEngine.RunStarvationPass(ctx, tx) }},
		{"construction", e.resolveCompletedUpgrades},
		{"staged_raid_departures", e.autoLaunchExpiredStagedRaids},
		{"coop_rallies", e.resolveCompletedCoopRallies},
		{"raid_combat", e.resolveRaidCombats},
		{"faction_defeat_check", e.checkForFactionDefeat},
		{"world_boss_attacks", e.resolveWorldBossAttacks},
		{"relic_convoy_spawn", e.relicConvoySpawn},
		{"relic_convoy_expire", e.relicConvoyExpire},
		{"wandering_merchant_spawn", e.wanderingMerchantSpawn},
		{"wandering_merchant_expire", e.wanderingMerchantExpire},
		{"clan_wars", e.resolveClanWars},
		{"daily_tax", e.collectDailyTax},
		{"constellation_dividend", e.constellationDividend},
		{"constellation_expire", e.constellationExpire},
		{"crafting_resolve", func(ctx context.Context, tx *sql.Tx) error { return gear.ResolveCraftingQueue(ctx, tx) }},
		{"exploration_resolve", e.resolveExplorationDispatches},
		{"scout_missions", e.processScoutMissions},
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

	if time.Now().UTC().Sub(e.lastIdleCheck) >= 20*time.Minute {
		e.runPhase(ctx, tickPhase{"idle_miner_sweep", e.notifyIdleMiners})
		e.lastIdleCheck = time.Now().UTC()
	}

	if time.Now().UTC().Sub(e.lastAutoScan) >= 30*time.Minute {
		e.runPhase(ctx, tickPhase{"auto_scan_sweep", e.autoScanSweep})
		e.lastAutoScan = time.Now().UTC()
	}

	e.lastTickMu.Lock()
	e.lastTickDone = time.Now().UTC()
	e.lastTickMu.Unlock()

	log.Printf("Tick pass complete. Duration: %s", time.Since(start))
}

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

			_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2 WHERE encampment_id = $3", sols, mechs, r.attackerID)
			_, _ = tx.ExecContext(ctx, "DELETE FROM raids WHERE id = $1", r.id)

			cancelMsg := "🤝 " + htmlBoldTick("CO-OP NOTICE") + ": Your staged lobby was cancelled and initial forces refunded because no ally commanders joined before departure."
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", creatorUserID, cancelMsg)
			continue
		}

		_, _ = tx.ExecContext(ctx, "UPDATE raid_coop_members SET state = 'stationed' WHERE raid_id = $1", r.id)

		var myX, myY int
		var myRegion string
		_ = tx.QueryRowContext(ctx, "SELECT c.x, c.y, c.region FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", r.attackerID).Scan(&myX, &myY, &myRegion)

		var defX, defY int
		var defRegion string
		if r.defenderID.Valid {
			_ = tx.QueryRowContext(ctx, "SELECT c.x, c.y, c.region FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", r.defenderID.String).Scan(&defX, &defY, &defRegion)
		} else {
			defX, defY, defRegion = 1, 1, myRegion
		}

		steps := math.Abs(float64(defX-myX)) + math.Abs(float64(defY-myY))
		if steps == 0 {
			steps = 1
		}
		marchingMinutes := steps * 10.0
		arrival := time.Now().UTC().Add(time.Duration(marchingMinutes) * time.Minute)
		legStartedAt := time.Now().UTC()

		_, _ = tx.ExecContext(ctx, `
			UPDATE raids SET state = 'marching', resolve_time = $1,
			                 origin_x = $2, origin_y = $3, destination_x = $4, destination_y = $5,
			                 origin_region = $6, destination_region = $7,
			                 leg_started_at = $8, leg_total_minutes = $9, base_march_minutes = $9
			WHERE id = $10`,
			arrival, myX, myY, defX, defY, myRegion, defRegion, legStartedAt, marchingMinutes, r.id)

		launchMsg := "🤝 " + htmlBoldTick("CO-OP LOBBY DEPARTED") + ": The staging window expired. Your joint military forces have successfully departed towards target."
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", creatorUserID, launchMsg)
	}
	return nil
}

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
		alert := fmt.Sprintf("⛏️ %s: Outpost %s currently has %s idle miners stationed in the hangar. Open Active Mining on camp deck to deploy them!",
			htmlBoldTick("WORKSTATION ALERT"), htmlCodeTick(htmlEscapeTick(t.campName)), htmlCodeTick(fmt.Sprintf("%d", t.idle)))
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

		alertMsg := fmt.Sprintf("⛏️ <b>EXTRACTION COMPLETE</b>: Your miners successfully returned with <code>+%.1f %s</code>! %s", gain, m.resType, resourceEmojiTick(m.resType))
		queryNotif := "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)"
		_, _ = tx.ExecContext(ctx, queryNotif, m.userID, alertMsg)
	}

	return nil
}

func (e *Engine) resolvePendingEspionageMissions(ctx context.Context, tx *sql.Tx) error {
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

		returnDuration := 2 * time.Minute
		returnResolveTime := time.Now().UTC().Add(returnDuration)

		_, _ = tx.ExecContext(ctx, "UPDATE spy_missions SET resolved = TRUE, resolve_time = $1 WHERE id = $2", returnResolveTime, s.id)

		defenderAlert := fmt.Sprintf(
			"🛰️ %s Outpost %s has successfully scanned your warehouse telemetry!\n"+
				"The spy satellite is now returning to orbit. You have <code>%s</code> left to launch an Interceptor Drone and vaporize the intel before it lands. ⏳",
			htmlBoldTick("ESPIONAGE BREACH:"), htmlBoldTick(htmlEscapeTick(s.spyName)), formatDurationTick(returnDuration),
		)
		if isRealPlayer(s.targetUserID) {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", s.targetUserID, defenderAlert)
		}
	}

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
			"🛰️ %s\n\nYour spy satellite has returned safely to hangar. Telemetry fully decrypted:\n\nTarget Outpost: %s\n\n%s\n⚙️ Scrap: %s\n🥫 Rations: %s\n\n%s\n⛺ Tent: Level %s\n⚙️ Scrap Heap: Level %s\n⚡ Generator: Level %s",
			htmlBoldTick("ESPIONAGE DOWNLINK COMPLETED!"), htmlCodeTick(htmlEscapeTick(targetName)),
			htmlBoldTick("DECRYPTED RESOURCES:"), htmlCodeTick(fmt.Sprintf("%.1f", scrap)), htmlCodeTick(fmt.Sprintf("%.1f", rations)),
			htmlBoldTick("MODULE STATUS GRID:"), htmlCodeTick(fmt.Sprintf("%d", tentLvl)), htmlCodeTick(fmt.Sprintf("%d", heapLvl)), htmlCodeTick(fmt.Sprintf("%d", genLvl)),
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
		       COALESCE(r.attacker_electricity, 100.0), COALESCE(r.attacker_logistics, 100.0),
		       COALESCE(r.base_march_minutes, 15.0), COALESCE(ed.name, 'Rogue Drone Nest'),
		       COALESCE(r.movement_state, 'moving'), COALESCE(ea.is_ai_faction, FALSE)
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
		electricity      float64
		logistics        float64
		baseMarchMinutes float64
		defenderName     string
		movementState    string
		isAIFaction      bool
	}

	var exps []activeExp
	for rows.Next() {
		var ex activeExp
		if err := rows.Scan(&ex.id, &ex.attackerID, &ex.state, &ex.resolveTime, &ex.userID,
			&ex.rations, &ex.ammo, &ex.electricity, &ex.logistics, &ex.baseMarchMinutes, &ex.defenderName, &ex.movementState, &ex.isAIFaction); err == nil {
			exps = append(exps, ex)
		}
	}

	for _, ex := range exps {
		if ex.movementState != "moving" {
			continue
		}

		if ex.isAIFaction {
			continue
		}

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

		if metalFuel > 0 && newMetalFuel <= 0 {
			delayedResolve := ex.resolveTime.UTC().Add(3 * time.Minute)
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET resolve_time = $1 WHERE id = $2", delayedResolve, ex.id)
		}

		tickSeconds := e.TickInterval.Seconds()
		if tickSeconds <= 0 {
			tickSeconds = float64(defaultTickInterval / time.Second)
		}
		legTicks := math.Max(1.0, math.Ceil((ex.baseMarchMinutes*60.0)/tickSeconds))
		rationsDrainPerTick := math.Max(0.5, 100.0/(legTicks*1.4))
		ammoDrainPerTick := rationsDrainPerTick
		electricityDrainPerTick := math.Max(0.5, 100.0/(legTicks*1.8))
		logisticsDrainPerTick := math.Max(0.5, 100.0/(legTicks*2.2))

		oldRations := ex.rations
		oldAmmo := ex.ammo
		oldElectricity := ex.electricity
		oldLogistics := ex.logistics
		newRations := math.Max(oldRations-rationsDrainPerTick, 0.0)
		newAmmo := math.Max(oldAmmo-ammoDrainPerTick, 0.0)
		newElectricity := math.Max(oldElectricity-electricityDrainPerTick, 0.0)
		newLogistics := math.Max(oldLogistics-logisticsDrainPerTick, 0.0)

		_, _ = tx.ExecContext(ctx, "UPDATE raids SET attacker_rations = $1, attacker_ammo = $2, attacker_electricity = $4, attacker_logistics = $5 WHERE id = $3", newRations, newAmmo, ex.id, newElectricity, newLogistics)

		if oldRations > 25.0 && newRations <= 25.0 {
			if isRealPlayer(ex.userID) {
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ex.userID,
					fmt.Sprintf("🍖 %s: Your marching force toward %s is down to %s rations.", htmlBoldTick("RATIONS RUNNING LOW"), htmlCodeTick(htmlEscapeTick(ex.defenderName)), htmlCodeTick(fmt.Sprintf("%.0f%%", newRations))))
			}
		} else if oldRations > 0 && newRations <= 0 {
			if isRealPlayer(ex.userID) {
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ex.userID,
					fmt.Sprintf("🍖❌ %s: Your force marching on %s has run out of rations! Combat strength will suffer until they return.", htmlBoldTick("OUT OF RATIONS"), htmlCodeTick(htmlEscapeTick(ex.defenderName))))
			}
		}
		if oldAmmo > 25.0 && newAmmo <= 25.0 {
			if isRealPlayer(ex.userID) {
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ex.userID,
					fmt.Sprintf("🎖️ %s: Your marching force toward %s is down to %s ammunition.", htmlBoldTick("AMMUNITION RUNNING LOW"), htmlCodeTick(htmlEscapeTick(ex.defenderName)), htmlCodeTick(fmt.Sprintf("%.0f%%", newAmmo))))
			}
		} else if oldAmmo > 0 && newAmmo <= 0 {
			if isRealPlayer(ex.userID) {
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ex.userID,
					fmt.Sprintf("🎖️❌ %s: Your force marching on %s has run dry! Combat strength will suffer until they return.", htmlBoldTick("OUT OF AMMUNITION"), htmlCodeTick(htmlEscapeTick(ex.defenderName))))
			}
		}
		if oldElectricity > 25.0 && newElectricity <= 25.0 {
			if isRealPlayer(ex.userID) {
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ex.userID,
					fmt.Sprintf("⚡ %s: Your expedition toward %s is down to %s carried electricity.", htmlBoldTick("FIELD POWER LOW"), htmlCodeTick(htmlEscapeTick(ex.defenderName)), htmlCodeTick(fmt.Sprintf("%.0f%%", newElectricity))))
			}
		} else if oldElectricity > 0 && newElectricity <= 0 {
			if isRealPlayer(ex.userID) {
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ex.userID,
					fmt.Sprintf("⚡❌ %s: High-tech units marching on %s are losing effectiveness until reinforcements arrive or they return.", htmlBoldTick("FIELD POWER DEPLETED"), htmlCodeTick(htmlEscapeTick(ex.defenderName))))
			}
		}
		if oldLogistics > 25.0 && newLogistics <= 25.0 {
			if isRealPlayer(ex.userID) {
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ex.userID,
					fmt.Sprintf("🔩 %s: Your expedition toward %s is down to %s repair/grid equipment.", htmlBoldTick("LOGISTICS SUPPLIES LOW"), htmlCodeTick(htmlEscapeTick(ex.defenderName)), htmlCodeTick(fmt.Sprintf("%.0f%%", newLogistics))))
			}
		} else if oldLogistics > 0 && newLogistics <= 0 {
			if isRealPlayer(ex.userID) {
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ex.userID,
					fmt.Sprintf("🔩❌ %s: Vehicles and grid equipment in the force marching on %s are no longer fully operational.", htmlBoldTick("LOGISTICS DEPLETED"), htmlCodeTick(htmlEscapeTick(ex.defenderName))))
			}
		}

		const powerOutageGraceTicks = 5

		foodAmmoOut := newRations <= 0 && newAmmo <= 0
		powerOut := newElectricity <= 0 && newLogistics <= 0

		if ex.state == "marching" && foodAmmoOut {
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET movement_state = 'awaiting_reinforcement', paused_at = CURRENT_TIMESTAMP WHERE id = $1", ex.id)
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ex.userID,
				fmt.Sprintf("🛑 %s: Your force marching on %s has run out of food and ammunition and cannot continue. Dispatch a resupply convoy or order a retreat from your ⚔️ Expedition Radar.", htmlBoldTick("COLUMN HALTED"), htmlCodeTick(htmlEscapeTick(ex.defenderName))))
		} else if ex.state == "marching" && powerOut {
			var wasOffline bool
			var outageTicks int
			_ = tx.QueryRowContext(ctx, "SELECT high_tech_offline, power_outage_ticks FROM raids WHERE id = $1", ex.id).Scan(&wasOffline, &outageTicks)
			outageTicks++

			if outageTicks > powerOutageGraceTicks {
				_, _ = tx.ExecContext(ctx, "UPDATE raids SET movement_state = 'awaiting_reinforcement', paused_at = CURRENT_TIMESTAMP, power_outage_ticks = $1 WHERE id = $2", outageTicks, ex.id)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ex.userID,
					fmt.Sprintf("🛑 %s: Sustained power/logistics failure in the force marching on %s has finally forced a full halt. Dispatch a resupply convoy or order a retreat from your ⚔️ Expedition Radar.", htmlBoldTick("COLUMN HALTED"), htmlCodeTick(htmlEscapeTick(ex.defenderName))))
			} else {
				_, _ = tx.ExecContext(ctx, "UPDATE raids SET high_tech_offline = TRUE, power_outage_ticks = $1 WHERE id = $2", outageTicks, ex.id)
				if !wasOffline {
					_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", ex.userID,
						fmt.Sprintf("⚠️ %s: Power/logistics failure in the force marching on %s has disabled Mech and capital-unit tech bonuses. The column presses on, but a full halt follows if power isn't restored soon.", htmlBoldTick("HIGH TECH OFFLINE"), htmlCodeTick(htmlEscapeTick(ex.defenderName))))
				}
			}
		} else if ex.state == "marching" {
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET high_tech_offline = FALSE, power_outage_ticks = 0 WHERE id = $1 AND (high_tech_offline = TRUE OR power_outage_ticks > 0)", ex.id)
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
				qu.powerRating = soldiers + mechs*5
				participants = append(participants, qu)
			}
		}
		rows.Close()

		if len(participants) >= e.requiredMatchCount(b) {
			requiredCount := e.requiredMatchCount(b)
			matched := participants[:requiredCount]

			shuffled := make([]queuedUser, len(matched))
			copy(shuffled, matched)
			rand.Shuffle(len(shuffled), func(i, j int) { shuffled[i], shuffled[j] = shuffled[j], shuffled[i] })

			teamA := shuffled[:requiredCount/2]
			teamB := shuffled[requiredCount/2:]

			var powerA, powerB int
			for _, p := range teamA {
				powerA += p.powerRating
			}
			for _, p := range teamB {
				powerB += p.powerRating
			}

			if powerA <= 0 {
				powerA = 1
			}
			if powerB <= 0 {
				powerB = 1
			}

			winProbA := float64(powerA) / float64(powerA+powerB)

			var winners, losers []queuedUser
			if rand.Float64() < winProbA {
				winners, losers = teamA, teamB
			} else {
				winners, losers = teamB, teamA
			}

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

				winAlert := fmt.Sprintf("🏟️ %s\n\nYou won the %s team clash! 🎉\n🏆 Reward: <code>+$%.0f Cash</code> credited.", htmlBoldTick("ARENA REPORT: TEAM VICTORY!"), b, lootWon)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent, effect_id) VALUES ($1, $2, FALSE, $3)", w.userID, winAlert, notifications.EffectCelebration)
			}

			for _, l := range losers {
				loseAlert := fmt.Sprintf("🏟️ %s\n\nYou lost the %s team clash. Keep training commander! 💪", htmlBoldTick("ARENA REPORT: TEAM DEFEAT"), b)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent, effect_id) VALUES ($1, $2, FALSE, $3)", l.userID, loseAlert, notifications.EffectThumbsDown)
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

				alert := fmt.Sprintf("⏳ %s\n\nNo equivalent opponents were found for the %s queue. Your entry fee of %s has been refunded.",
					htmlBoldTick("ARENA QUEUE TIMEOUT"), htmlCodeTick(b), htmlCodeTick(fmt.Sprintf("$%.0f", refundDollars)))
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
			"🏗️ %s\n\nOutpost: %s\nUpgrade completed successfully!\nYour %s has successfully reached Level %s.",
			htmlBoldTick("CONSTRUCTION COMPLETE"), htmlCodeTick(htmlEscapeTick(c.campName)), htmlCodeTick(c.modType), htmlCodeTick(fmt.Sprintf("%d", newLvl)),
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

func clampRouteProgress(progress float64) float64 {
	return math.Min(1, math.Max(0, progress))
}

func routeProgressAt(state string, progress float64, progressAt sql.NullTime, legMinutes float64, resolveTime, now time.Time) float64 {
	if legMinutes <= 0 {
		return clampRouteProgress(progress)
	}
	if progressAt.Valid {
		elapsed := math.Max(0, now.UTC().Sub(progressAt.Time.UTC()).Minutes())
		delta := elapsed / legMinutes
		if state == "returning" {
			return clampRouteProgress(progress - delta)
		}
		return clampRouteProgress(progress + delta)
	}

	remaining := math.Min(1, math.Max(0, now.UTC().Sub(resolveTime.UTC()).Minutes()/-legMinutes))
	if state == "returning" {
		return remaining
	}
	return 1 - remaining
}

func (e *Engine) resumeRouteRecoveries(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE raids
		SET movement_state = 'moving', pause_reason = NULL, next_route_event_at = NULL,
			route_progress_at = CURRENT_TIMESTAMP,
			resolve_time = CURRENT_TIMESTAMP + (
				CASE WHEN state = 'returning'
					THEN COALESCE(route_progress, 1.0)
					ELSE 1.0 - COALESCE(route_progress, 0.0)
				END * COALESCE(route_leg_minutes, base_march_minutes, 15.0) * INTERVAL '1 minute'
			)
		WHERE movement_state = 'battle_recovery'
		  AND next_route_event_at IS NOT NULL
		  AND next_route_event_at <= CURRENT_TIMESTAMP
		  AND state IN ('marching', 'returning')`)
	if err != nil {
		return fmt.Errorf("resuming route recovery columns: %w", err)
	}
	return nil
}

func resumeRoadRaids(ctx context.Context, tx *sql.Tx, primaryRaidID, secondaryRaidID string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE raids
		SET movement_state = 'moving', pause_reason = NULL, next_route_event_at = NULL,
			route_progress_at = CURRENT_TIMESTAMP,
			resolve_time = CURRENT_TIMESTAMP + (
				CASE WHEN state = 'returning'
					THEN COALESCE(route_progress, 1.0)
					ELSE 1.0 - COALESCE(route_progress, 0.0)
				END * COALESCE(route_leg_minutes, base_march_minutes, 15.0) * INTERVAL '1 minute'
			)
		WHERE (id = $1 OR id = $2)
		  AND movement_state IN ('encounter_pending', 'encounter_battle')
		  AND state IN ('marching', 'returning')`, primaryRaidID, secondaryRaidID)
	return err
}

func (e *Engine) discoverRouteContacts(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT r.attacker_id, ea.user_id, COALESCE(r.defender_id::text, ''), r.state,
		       COALESCE(r.leg_started_at, r.created_at, CURRENT_TIMESTAMP),
		       COALESCE(r.leg_total_minutes, r.base_march_minutes, 15.0),
		       r.paused_at,
		       r.origin_x, r.origin_y, r.destination_x, r.destination_y,
		       r.origin_region, r.destination_region
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		WHERE r.state IN ('marching', 'returning')
		  AND r.origin_x IS NOT NULL AND r.origin_y IS NOT NULL
		  AND r.destination_x IS NOT NULL AND r.destination_y IS NOT NULL
		  AND r.origin_region IS NOT NULL AND r.destination_region IS NOT NULL
		  AND r.resolve_time > CURRENT_TIMESTAMP`)
	if err != nil {
		return fmt.Errorf("querying route discovery candidates: %w", err)
	}
	defer rows.Close()

	type movingRaid struct {
		attackerID        string
		attackerUserID    int64
		defenderID        string
		state             string
		legStartedAt      time.Time
		legTotalMinutes   float64
		pausedAt          sql.NullTime
		originX, originY  int
		destinationX      int
		destinationY      int
		originRegion      string
		destinationRegion string
	}
	var raids []movingRaid
	for rows.Next() {
		var raid movingRaid
		if err := rows.Scan(&raid.attackerID, &raid.attackerUserID, &raid.defenderID, &raid.state,
			&raid.legStartedAt, &raid.legTotalMinutes, &raid.pausedAt, &raid.originX, &raid.originY,
			&raid.destinationX, &raid.destinationY, &raid.originRegion, &raid.destinationRegion); err != nil {
			return fmt.Errorf("scanning route discovery candidate: %w", err)
		}
		raids = append(raids, raid)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating route discovery candidates: %w", err)
	}

	for _, raid := range raids {
		effectiveNow := time.Now().UTC()
		if raid.pausedAt.Valid {
			effectiveNow = raid.pausedAt.Time.UTC()
		}
		progress := roadcombat.RouteProgress(raid.legStartedAt.UTC(), raid.legTotalMinutes, effectiveNow)
		pos := roadcombat.CurrentPosition(raid.state, raid.originX, raid.originY, raid.destinationX, raid.destinationY, progress)
		currentX := int(math.Round(pos.X))
		currentY := int(math.Round(pos.Y))
		currentRegion := raid.originRegion
		if progress >= 0.5 {
			currentRegion = raid.destinationRegion
		}

		var contactID, contactName string
		var contactUserID int64
		err := tx.QueryRowContext(ctx, `
			SELECT e.id, e.name, e.user_id
			FROM encampments e
			JOIN coordinates c ON c.id = e.coordinate_id
			WHERE e.id <> $1
			  AND ($2 = '' OR e.id::text <> $2)
			  AND c.region = $3
			  AND ABS(c.x - $4) + ABS(c.y - $5) <= 1
			  AND NOT EXISTS (
				SELECT 1 FROM encampment_discoveries d
				WHERE d.observer_encampment_id = $1 AND d.target_encampment_id = e.id
			)
			ORDER BY ABS(c.x - $4) + ABS(c.y - $5), e.established_at
			LIMIT 1`, raid.attackerID, raid.defenderID, currentRegion, currentX, currentY).Scan(&contactID, &contactName, &contactUserID)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return fmt.Errorf("locating route contact: %w", err)
		}

		result, err := tx.ExecContext(ctx, `
			INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method)
			VALUES ($1, $2, 'route') ON CONFLICT DO NOTHING`, raid.attackerID, contactID)
		if err != nil {
			return fmt.Errorf("recording outbound route contact: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			continue
		}
		if isRealPlayer(raid.attackerUserID) {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", raid.attackerUserID,
				fmt.Sprintf("📡 %s: Your expedition passed close enough to discover Outpost %s. It is now visible in the Tactical Target Matrix.", htmlBoldTick("ROUTE CONTACT"), htmlCodeTick(htmlEscapeTick(contactName))))
		}

		result, err = tx.ExecContext(ctx, `
			INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method)
			VALUES ($1, $2, 'route') ON CONFLICT DO NOTHING`, contactID, raid.attackerID)
		if err != nil {
			return fmt.Errorf("recording reciprocal route contact: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected == 1 && isRealPlayer(contactUserID) {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", contactUserID,
				"📡 "+htmlBoldTick("ROUTE CONTACT")+": Your sensors detected a foreign expedition moving near your outpost. Its commander has now discovered your location.")
		}
	}
	return nil
}

type roadMover struct {
	raidID         string
	attackerID     string
	attackerUserID int64
	attackerName   string
	state          string
	pos            roadcombat.Position
	region         string
}

func (e *Engine) evaluateRoadEncounters(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT r.id, r.attacker_id, ea.user_id, ea.name, r.state,
		       COALESCE(r.leg_started_at, r.created_at, CURRENT_TIMESTAMP),
		       COALESCE(r.leg_total_minutes, r.base_march_minutes, 15.0),
		       r.origin_x, r.origin_y, r.destination_x, r.destination_y
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		WHERE r.state IN ('marching', 'returning')
		  AND r.movement_state = 'moving'
		  AND r.active_encounter_id IS NULL
		  AND r.active_base_encounter_id IS NULL
		  AND r.origin_x IS NOT NULL AND r.destination_x IS NOT NULL
		ORDER BY r.id`)
	if err != nil {
		return fmt.Errorf("querying road-encounter candidates: %w", err)
	}
	defer rows.Close()

	var movers []roadMover
	for rows.Next() {
		var m roadMover
		var legStartedAt time.Time
		var legTotalMinutes float64
		var ox, oy, dx, dy int
		if err := rows.Scan(&m.raidID, &m.attackerID, &m.attackerUserID, &m.attackerName, &m.state,
			&legStartedAt, &legTotalMinutes, &ox, &oy, &dx, &dy); err != nil {
			return fmt.Errorf("scanning road-encounter candidate: %w", err)
		}
		progress := roadcombat.RouteProgress(legStartedAt.UTC(), legTotalMinutes, time.Now().UTC())
		m.pos = roadcombat.CurrentPosition(m.state, ox, oy, dx, dy, progress)
		movers = append(movers, m)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating road-encounter candidates: %w", err)
	}

	for i := 0; i < len(movers); i++ {
		for j := i + 1; j < len(movers); j++ {
			a, b := movers[i], movers[j]
			if a.attackerID == b.attackerID {
				continue
			}
			if !roadcombat.InEncounterRange(a.pos, b.pos) {
				continue
			}

			raidA, raidB := a.raidID, b.raidID
			if raidA > raidB {
				a, b = b, a
				raidA, raidB = raidB, raidA
			}

			var recentCount int
			_ = tx.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM road_encounters
				WHERE raid_a_id = $1 AND raid_b_id = $2
				  AND status = 'resolved' AND resolved_at > $3`,
				raidA, raidB, time.Now().UTC().Add(-roadcombat.EncounterCooldown)).Scan(&recentCount)
			if recentCount > 0 {
				continue
			}

			if rand.Float64() >= roadcombat.EncounterRollChance {
				continue
			}

			locX := int(math.Round((a.pos.X + b.pos.X) / 2))
			locY := int(math.Round((a.pos.Y + b.pos.Y) / 2))
			deadline := time.Now().UTC().Add(roadcombat.ResponseWindow)

			var encounterID string
			err := tx.QueryRowContext(ctx, `
				INSERT INTO road_encounters (raid_a_id, raid_b_id, location_x, location_y, response_deadline)
				VALUES ($1, $2, $3, $4, $5)
				ON CONFLICT DO NOTHING
				RETURNING id`, raidA, raidB, locX, locY, deadline).Scan(&encounterID)
			if err != nil {
				if err == sql.ErrNoRows {
					continue
				}
				return fmt.Errorf("creating road encounter: %w", err)
			}

			_, _ = tx.ExecContext(ctx, `
				UPDATE raids SET movement_state = 'encounter_pending', active_encounter_id = $1, paused_at = CURRENT_TIMESTAMP
				WHERE id IN ($2, $3)`, encounterID, raidA, raidB)

			_, _ = tx.ExecContext(ctx, `INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method) VALUES ($1, $2, 'route') ON CONFLICT DO NOTHING`, a.attackerID, b.attackerID)
			_, _ = tx.ExecContext(ctx, `INSERT INTO encampment_discoveries (observer_encampment_id, target_encampment_id, discovery_method) VALUES ($1, $2, 'route') ON CONFLICT DO NOTHING`, b.attackerID, a.attackerID)

			for _, side := range []struct {
				userID    int64
				otherName string
			}{
				{a.attackerUserID, b.attackerName},
				{b.attackerUserID, a.attackerName},
			} {
				msg := fmt.Sprintf(
					"🚧 %s\n\nYour expedition has encountered forces commanded by %s on the road.\nYou have %s to decide: Attack, or Continue on your way (open your ⚔️ Expedition Radar to choose). Taking no action lets your column pass them peacefully.",
					htmlBoldTick("ROAD CONTACT!"), htmlCodeTick(htmlEscapeTick(side.otherName)), htmlCodeTick(formatDurationTick(roadcombat.ResponseWindow)),
				)
				if isRealPlayer(side.userID) {
					_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", side.userID, msg)
				}
			}
		}
	}

	return e.expireRoadEncounters(ctx, tx)
}

func (e *Engine) expireRoadEncounters(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, raid_a_id, raid_b_id FROM road_encounters
		WHERE status = 'pending' AND response_deadline <= CURRENT_TIMESTAMP`)
	if err != nil {
		return fmt.Errorf("querying expired road encounters: %w", err)
	}
	var expired []struct{ id, raidA, raidB string }
	for rows.Next() {
		var row struct{ id, raidA, raidB string }
		if err := rows.Scan(&row.id, &row.raidA, &row.raidB); err == nil {
			expired = append(expired, row)
		}
	}
	rows.Close()

	for _, row := range expired {
		if err := e.resolveEncounterAsContinue(ctx, tx, row.id, row.raidA, row.raidB, "timeout"); err != nil {
			log.Printf("road encounter %s failed to auto-resolve: %v", row.id, err)
		}
	}
	return nil
}

func (e *Engine) resolveEncounterAsContinue(ctx context.Context, tx *sql.Tx, encounterID, raidAID, raidBID, outcome string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE road_encounters SET status = 'resolved', outcome = $1, resolved_at = CURRENT_TIMESTAMP
		WHERE id = $2 AND status = 'pending'`, outcome, encounterID)
	if err != nil {
		return err
	}

	for _, raidID := range []string{raidAID, raidBID} {
		_, _ = tx.ExecContext(ctx, `
			UPDATE raids
			SET movement_state = 'moving',
			    active_encounter_id = NULL,
			    leg_started_at = leg_started_at + (CURRENT_TIMESTAMP - COALESCE(paused_at, CURRENT_TIMESTAMP)),
			    resolve_time = resolve_time + (CURRENT_TIMESTAMP - COALESCE(paused_at, CURRENT_TIMESTAMP)),
			    paused_at = NULL
			WHERE id = $1 AND active_encounter_id = $2`, raidID, encounterID)

		var userID int64
		var attackerID string
		if err := tx.QueryRowContext(ctx, "SELECT attacker_id FROM raids WHERE id = $1", raidID).Scan(&attackerID); err == nil {
			_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", attackerID).Scan(&userID)
			if isRealPlayer(userID) {
				note := "🛣️ " + htmlBoldTick("ROAD CONTACT RESOLVED") + ": Both columns continued on their way without engaging."
				if outcome == "timeout" {
					note = "🛣️ " + htmlBoldTick("ROAD CONTACT RESOLVED") + ": No attack order was given in time - your column continued on its way."
				}
				_ = notifications.Queue(ctx, tx, userID, note, "route_status")
			}
		}
	}
	return nil
}

func (e *Engine) evaluateRouteWeatherIncidents(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT r.id, ea.user_id, r.state,
		       COALESCE(r.leg_started_at, r.created_at, CURRENT_TIMESTAMP),
		       COALESCE(r.leg_total_minutes, r.base_march_minutes, 15.0),
		       r.origin_x, r.origin_y, r.destination_x, r.destination_y,
		       r.origin_region, r.destination_region
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		WHERE r.state IN ('marching', 'returning')
		  AND r.movement_state = 'moving'
		  AND r.active_incident_id IS NULL
		  AND r.origin_x IS NOT NULL AND r.destination_x IS NOT NULL
		  AND r.origin_region IS NOT NULL AND r.destination_region IS NOT NULL`)
	if err != nil {
		return fmt.Errorf("querying route weather candidates: %w", err)
	}

	type candidate struct {
		raidID                          string
		userID                          int64
		state                           string
		legStartedAt                    time.Time
		legTotalMinutes                 float64
		originX, originY                int
		destinationX, destinationY      int
		originRegion, destinationRegion string
	}
	var candidates []candidate
	for rows.Next() {
		var c candidate
		if err := rows.Scan(&c.raidID, &c.userID, &c.state, &c.legStartedAt, &c.legTotalMinutes,
			&c.originX, &c.originY, &c.destinationX, &c.destinationY, &c.originRegion, &c.destinationRegion); err == nil {
			candidates = append(candidates, c)
		}
	}
	rows.Close()

	incidentTypes := []string{"flood", "storm", "heatwave", "sandstorm", "emp", "radiation"}

	for _, c := range candidates {
		progress := roadcombat.RouteProgress(c.legStartedAt.UTC(), c.legTotalMinutes, time.Now().UTC())
		pos := roadcombat.CurrentPosition(c.state, c.originX, c.originY, c.destinationX, c.destinationY, progress)
		currentRegion := c.originRegion
		if progress >= 0.5 {
			currentRegion = c.destinationRegion
		}

		var activeEventType string
		_ = tx.QueryRowContext(ctx, `
			SELECT event_type FROM world_events
			WHERE continent = $1 AND expires_at > CURRENT_TIMESTAMP
			ORDER BY expires_at DESC LIMIT 1`, currentRegion).Scan(&activeEventType)

		incidentType := incidentTypes[rand.Intn(len(incidentTypes))]
		rollChance := roadcombat.IncidentBaseRollChance
		if activeEventType != "" {
			directLocalName := ""
			switch activeEventType {
			case "radiation_storm":
				directLocalName = "radiation"
			case "sandstorm", "emp":
				directLocalName = activeEventType
			}

			if directLocalName != "" && roadcombat.IncidentMatchesActiveWeather(activeEventType, directLocalName) {
				incidentType = directLocalName
				rollChance = roadcombat.IncidentElevatedRollChance
			} else {
				for _, candidateType := range incidentTypes {
					if roadcombat.IncidentMatchesActiveWeather(activeEventType, candidateType) {
						incidentType = candidateType
						rollChance = roadcombat.IncidentElevatedRollChance
						break
					}
				}
			}
		}

		if rand.Float64() >= rollChance {
			continue
		}

		severity := 1 + rand.Intn(3)
		clearedAt := time.Now().UTC().Add(roadcombat.IncidentDuration(severity))
		locX := int(math.Round(pos.X))
		locY := int(math.Round(pos.Y))

		var incidentID string
		err := tx.QueryRowContext(ctx, `
			INSERT INTO route_incidents (raid_id, incident_type, severity, location_x, location_y, cleared_at)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT DO NOTHING
			RETURNING id`, c.raidID, incidentType, severity, locX, locY, clearedAt).Scan(&incidentID)
		if err != nil {
			if err == sql.ErrNoRows {
				continue
			}
			return fmt.Errorf("creating route incident: %w", err)
		}

		_, _ = tx.ExecContext(ctx, `
			UPDATE raids SET movement_state = 'camped', active_incident_id = $1, paused_at = CURRENT_TIMESTAMP
			WHERE id = $2`, incidentID, c.raidID)

		effectNote := e.applyRouteIncidentOnsetEffect(ctx, tx, c.raidID, incidentType)

		icon, label := routeIncidentPresentation(incidentType)
		hours := int(roadcombat.IncidentDuration(severity).Hours())
		msg := fmt.Sprintf("%s %s: Your column has been forced to make temporary camp by conditions on the road. Expected clear time: ~%s. The journey will resume automatically once conditions improve.",
			icon, htmlBoldTick(label), htmlCodeTick(fmt.Sprintf("%dh", hours)))
		if effectNote != "" {
			msg += "\n" + effectNote
		}
		if isRealPlayer(c.userID) {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", c.userID, msg)
		}
	}

	return e.clearRouteWeatherIncidents(ctx, tx)
}

func (e *Engine) applyRouteIncidentOnsetEffect(ctx context.Context, tx *sql.Tx, raidID, incidentType string) string {
	clampFloor := func(v, delta float64) float64 {
		v -= delta
		if v < 0 {
			return 0
		}
		return v
	}

	switch incidentType {
	case "flood":
		var rations float64
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(attacker_rations,0) FROM raids WHERE id = $1", raidID).Scan(&rations)
		_, _ = tx.ExecContext(ctx, "UPDATE raids SET attacker_rations = $1 WHERE id = $2", clampFloor(rations, 10), raidID)
		return "🍞 Rising water spoiled some dry rations (-10 supply)."

	case "storm":
		var electricity float64
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(attacker_electricity,0) FROM raids WHERE id = $1", raidID).Scan(&electricity)
		_, _ = tx.ExecContext(ctx, "UPDATE raids SET attacker_electricity = $1 WHERE id = $2", clampFloor(electricity, 10), raidID)
		return "⚡ Storm damage knocked out field generators (-10 electricity)."

	case "heatwave":
		var rations float64
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(attacker_rations,0) FROM raids WHERE id = $1", raidID).Scan(&rations)
		_, _ = tx.ExecContext(ctx, "UPDATE raids SET attacker_rations = $1 WHERE id = $2", clampFloor(rations, 15), raidID)
		return "🥵 Heat and dehydration burned through extra rations (-15 supply)."

	case "sandstorm":
		var logistics float64
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(attacker_logistics,0) FROM raids WHERE id = $1", raidID).Scan(&logistics)
		_, _ = tx.ExecContext(ctx, "UPDATE raids SET attacker_logistics = $1 WHERE id = $2", clampFloor(logistics, 10), raidID)
		return "🏜️ Fine sand fouled logistics equipment (-10 logistics)."

	case "emp":
		var electricity, logistics float64
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(attacker_electricity,0), COALESCE(attacker_logistics,0) FROM raids WHERE id = $1", raidID).Scan(&electricity, &logistics)
		_, _ = tx.ExecContext(ctx, "UPDATE raids SET attacker_electricity = $1, attacker_logistics = $2 WHERE id = $3", electricity*0.4, logistics*0.4, raidID)
		return "💥 An electromagnetic surge knocked out 60% of remaining electrical/logistics capacity."

	case "radiation":
		force, err := e.loadRoadFieldForceForIncident(ctx, tx, raidID)
		if err != nil {
			return ""
		}
		lost := roadcombat.CasualtiesFor(force, 0.05)
		survivors := roadcombat.Survivors(force, lost)
		_, _ = tx.ExecContext(ctx, `
			UPDATE raid_forces SET soldiers_mobilized = $1, mechs_mobilized = $2, destroyers_mobilized = $3,
			    bombers_mobilized = $4, battlecruisers_mobilized = $5, deathstars_mobilized = $6,
			    liberators_mobilized = $7, wraiths_mobilized = $8
			WHERE raid_id = $9`,
			survivors.Soldiers, survivors.Mechs, survivors.Destroyers, survivors.Bombers,
			survivors.Battlecruisers, survivors.Deathstars, survivors.Liberators, survivors.Wraiths, raidID)
		if lost.Soldiers > 0 || lost.Mechs > 0 {
			return fmt.Sprintf("☢️ Radiation exposure claimed %d Soldiers and %d Mechs before the column could take shelter.", lost.Soldiers, lost.Mechs)
		}
		return "☢️ Radiation exposure was survived without casualties this time."
	}
	return ""
}

func (e *Engine) loadRoadFieldForceForIncident(ctx context.Context, tx *sql.Tx, raidID string) (roadcombat.FieldForce, error) {
	var f roadcombat.FieldForce
	err := tx.QueryRowContext(ctx, `
		SELECT COALESCE(soldiers_mobilized,0), COALESCE(mechs_mobilized,0), COALESCE(destroyers_mobilized,0),
		       COALESCE(bombers_mobilized,0), COALESCE(battlecruisers_mobilized,0), COALESCE(deathstars_mobilized,0),
		       COALESCE(liberators_mobilized,0), COALESCE(wraiths_mobilized,0)
		FROM raid_forces WHERE raid_id = $1`, raidID).Scan(
		&f.Soldiers, &f.Mechs, &f.Destroyers, &f.Bombers, &f.Battlecruisers, &f.Deathstars, &f.Liberators, &f.Wraiths)
	return f, err
}

func routeIncidentPresentation(incidentType string) (string, string) {
	switch incidentType {
	case "flood":
		return "🌊", "FLASH FLOOD"
	case "storm":
		return "⛈️", "SEVERE STORM"
	case "heatwave":
		return "🔥", "HEATWAVE"
	case "sandstorm":
		return "🏜️", "SANDSTORM"
	case "emp":
		return "💥", "EMP SURGE"
	case "radiation":
		return "☢️", "RADIATION ZONE"
	default:
		return "🌍", "WEATHER DELAY"
	}
}

func (e *Engine) clearRouteWeatherIncidents(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, raid_id FROM route_incidents
		WHERE resolved = FALSE AND cleared_at <= CURRENT_TIMESTAMP`)
	if err != nil {
		return fmt.Errorf("querying cleared route incidents: %w", err)
	}
	var cleared []struct{ id, raidID string }
	for rows.Next() {
		var row struct{ id, raidID string }
		if err := rows.Scan(&row.id, &row.raidID); err == nil {
			cleared = append(cleared, row)
		}
	}
	rows.Close()

	for _, row := range cleared {
		_, _ = tx.ExecContext(ctx, "UPDATE route_incidents SET resolved = TRUE WHERE id = $1", row.id)
		_, _ = tx.ExecContext(ctx, `
			UPDATE raids
			SET movement_state = 'moving', active_incident_id = NULL,
			    leg_started_at = leg_started_at + (CURRENT_TIMESTAMP - COALESCE(paused_at, CURRENT_TIMESTAMP)),
			    resolve_time = resolve_time + (CURRENT_TIMESTAMP - COALESCE(paused_at, CURRENT_TIMESTAMP)),
			    paused_at = NULL
			WHERE id = $1 AND active_incident_id = $2`, row.raidID, row.id)

		var userID int64
		_ = tx.QueryRowContext(ctx, "SELECT ea.user_id FROM raids r JOIN encampments ea ON ea.id = r.attacker_id WHERE r.id = $1", row.raidID).Scan(&userID)
		if isRealPlayer(userID) {
			_ = notifications.Queue(ctx, tx, userID,
				"☀️ "+htmlBoldTick("CONDITIONS CLEARED")+": Your column has broken camp and resumed its journey.", "route_status")
		}
	}
	return nil
}

func (e *Engine) processSupplyConvoys(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, home_encampment_id, target_raid_id, rations_carried, ammo_carried, electricity_carried, logistics_carried
		FROM supply_convoys
		WHERE state = 'marching' AND resolve_time <= CURRENT_TIMESTAMP`)
	if err != nil {
		return fmt.Errorf("querying due supply convoys: %w", err)
	}
	type convoy struct {
		id, homeID, targetRaidID              string
		rations, ammo, electricity, logistics float64
		haulersCommitted, tankersCommitted    int
	}
	var convoys []convoy
	for rows.Next() {
		var c convoy
		if err := rows.Scan(&c.id, &c.homeID, &c.targetRaidID, &c.rations, &c.ammo, &c.electricity, &c.logistics); err == nil {
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(haulers_committed,1), COALESCE(tankers_committed,1) FROM supply_convoys WHERE id = $1", c.id).Scan(&c.haulersCommitted, &c.tankersCommitted)
			convoys = append(convoys, c)
		}
	}
	rows.Close()

	const convoyAmbushChance = 0.08

	for _, c := range convoys {
		var movementState, state string
		var curRations, curAmmo, curElectricity, curLogistics float64
		err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(movement_state,'moving'), state,
			       COALESCE(attacker_rations,0), COALESCE(attacker_ammo,0),
			       COALESCE(attacker_electricity,0), COALESCE(attacker_logistics,0)
			FROM raids WHERE id = $1 FOR UPDATE`, c.targetRaidID).Scan(&movementState, &state, &curRations, &curAmmo, &curElectricity, &curLogistics)

		delivered := err == nil && movementState == "awaiting_reinforcement" && (state == "marching" || state == "returning")
		if !delivered {
			_, _ = tx.ExecContext(ctx, "UPDATE supply_convoys SET state = 'failed' WHERE id = $1", c.id)
			_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET haulers = haulers + $1, tankers = tankers + $2 WHERE encampment_id = $3", c.haulersCommitted, c.tankersCommitted, c.homeID)
			var userID int64
			_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", c.homeID).Scan(&userID)
			if isRealPlayer(userID) {
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", userID,
					"📦❌ "+htmlBoldTick("CONVOY MISSED CONTACT")+": Your resupply convoy reached the column's last known position, but it had already moved on or returned. The supplies were lost, but the Hauler and Tanker crews made it back home.")
			}
			continue
		}

		if rand.Float64() < convoyAmbushChance {
			_, _ = tx.ExecContext(ctx, "UPDATE supply_convoys SET state = 'ambushed' WHERE id = $1", c.id)
			var homeUserID int64
			_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", c.homeID).Scan(&homeUserID)
			if isRealPlayer(homeUserID) {
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", homeUserID,
					"📦💥 "+htmlBoldTick("CONVOY AMBUSHED")+": Your resupply convoy was ambushed en route and lost, along with its Hauler and Tanker crews. The stranded column is still waiting for supplies.")
			}
			continue
		}

		clamp100 := func(v, add float64) float64 {
			total := v + add
			if total > 100 {
				return 100
			}
			return total
		}
		newRations := clamp100(curRations, c.rations)
		newAmmo := clamp100(curAmmo, c.ammo)
		newElectricity := clamp100(curElectricity, c.electricity)
		newLogistics := clamp100(curLogistics, c.logistics)

		_, _ = tx.ExecContext(ctx, `
			UPDATE raids
			SET attacker_rations = $1, attacker_ammo = $2, attacker_electricity = $3, attacker_logistics = $4,
			    movement_state = 'moving',
			    leg_started_at = leg_started_at + (CURRENT_TIMESTAMP - COALESCE(paused_at, CURRENT_TIMESTAMP)),
			    resolve_time = resolve_time + (CURRENT_TIMESTAMP - COALESCE(paused_at, CURRENT_TIMESTAMP)),
			    paused_at = NULL
			WHERE id = $5`, newRations, newAmmo, newElectricity, newLogistics, c.targetRaidID)

		_, _ = tx.ExecContext(ctx, "UPDATE supply_convoys SET state = 'delivered' WHERE id = $1", c.id)
		_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET haulers = haulers + $1, tankers = tankers + $2 WHERE encampment_id = $3", c.haulersCommitted, c.tankersCommitted, c.homeID)

		var userID int64
		_ = tx.QueryRowContext(ctx, "SELECT ea.user_id FROM raids r JOIN encampments ea ON ea.id = r.attacker_id WHERE r.id = $1", c.targetRaidID).Scan(&userID)
		if isRealPlayer(userID) {
			_ = notifications.Queue(ctx, tx, userID,
				"📦✅ "+htmlBoldTick("CONVOY ARRIVED")+": Reinforcement supplies have reached your stranded column. The journey resumes.", "route_status")
		}
	}
	return nil
}

const stalledColumnGraceAIFaction = 15 * time.Minute
const stalledColumnGraceRealPlayer = 3 * time.Hour

const stalledColumnMinResupplyPct = 20.0

func (e *Engine) resolveStalledColumns(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT r.id, r.attacker_id, r.state, ea.user_id, r.paused_at,
		       COALESCE(ea.is_ai_faction, FALSE)
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		WHERE r.movement_state = 'awaiting_reinforcement'
		  AND r.paused_at IS NOT NULL
		  AND (r.state = 'marching' OR r.state = 'returning')
		  AND NOT EXISTS (
		      SELECT 1 FROM supply_convoys sc
		      WHERE sc.target_raid_id = r.id AND sc.state = 'marching'
		  )`)
	if err != nil {
		return fmt.Errorf("querying stalled columns: %w", err)
	}

	type stalled struct {
		id         string
		attackerID string
		state      string
		userID     int64
		pausedAt   time.Time
		isAI       bool
	}
	var candidates []stalled
	for rows.Next() {
		var s stalled
		if err := rows.Scan(&s.id, &s.attackerID, &s.state, &s.userID, &s.pausedAt, &s.isAI); err == nil {
			candidates = append(candidates, s)
		}
	}
	rows.Close()

	now := time.Now().UTC()
	for _, s := range candidates {
		grace := stalledColumnGraceRealPlayer
		if s.isAI || !isRealPlayer(s.userID) {
			grace = stalledColumnGraceAIFaction
		}
		if now.Sub(s.pausedAt.UTC()) < grace {
			continue
		}

		clamp := func(v float64) float64 {
			if v < stalledColumnMinResupplyPct {
				return stalledColumnMinResupplyPct
			}
			return v
		}
		var rations, ammo, electricity, logistics float64
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(attacker_rations,0), COALESCE(attacker_ammo,0), COALESCE(attacker_electricity,0), COALESCE(attacker_logistics,0) FROM raids WHERE id = $1 FOR UPDATE", s.id).
			Scan(&rations, &ammo, &electricity, &logistics)

		_, execErr := tx.ExecContext(ctx, `
			UPDATE raids
			SET attacker_rations = $1, attacker_ammo = $2, attacker_electricity = $3, attacker_logistics = $4,
			    movement_state = 'moving',
			    high_tech_offline = FALSE, power_outage_ticks = 0,
			    leg_started_at = leg_started_at + (CURRENT_TIMESTAMP - paused_at),
			    resolve_time = resolve_time + (CURRENT_TIMESTAMP - paused_at),
			    paused_at = NULL
			WHERE id = $5`,
			clamp(rations), clamp(ammo), clamp(electricity), clamp(logistics), s.id)
		if execErr != nil {
			continue
		}

		if isRealPlayer(s.userID) {
			verb := "marching on"
			if s.state == "returning" {
				verb = "returning to base after"
			}
			_ = notifications.Queue(ctx, tx, s.userID,
				fmt.Sprintf("🛟 %s: Your column %s its objective sat stranded too long with no resupply convoy dispatched. An emergency logistics airdrop has restored just enough supplies (%.0f%%) to get it moving again.",
					htmlBoldTick("EMERGENCY RESUPPLY"), verb, stalledColumnMinResupplyPct), "route_status")
		}
	}
	return nil
}

const aiCivilizationMaxSoldiersPerLevel = 25
const aiCivilizationMaxMechsPerLevel = 4

var aiResearchTechColumns = []string{
	"econ_tech_lvl", "production_tech_lvl", "integrity_tech_lvl",
	"defense_tech_lvl", "intel_tech_lvl", "speed_tech_lvl", "military_tech_lvl",
}

const aiResearchMaxLevel = 20
const aiResearchCostPerLevel = 8

var aiUpgradeableModules = []string{
	"tent", "scrap_heap", "generator",
	"light_laser", "heavy_laser", "gauss_cannon", "ion_cannon", "plasma_turret", "anti_missile", "warehouse",
	"hangar", "radar", "solar_panel", "starport", "technology_center", "trade_beacon",
	"small_shield", "large_shield", "engineering_bay", "metal_mine", "crystal_mine",
}

const aiModuleUpgradeCostPerLevel = 150

const aiExplorationDispatchRationsCost = 30.0
const aiExplorationDispatchMetalCost = 15.0
const aiExplorationMinTravelMinutes = 20
const aiExplorationMaxTravelMinutes = 45

type aiExplorationSiteTemplate struct {
	siteType   string
	namePrefix string
	rewardType string
	minAmount  float64
	maxAmount  float64
	weight     int
}

var aiExplorationTemplates = []aiExplorationSiteTemplate{
	{"Scrapyard", "Scrap Yard", "scrap", 150, 400, 20},
	{"Cache", "Supply Cache", "metal", 100, 300, 18},
	{"Depot", "Ration Depot", "rations", 80, 200, 16},
	{"Generator", "Power Cell Cluster", "electricity", 60, 150, 14},
	{"Reserve", "Fuel Reserve", "hydrogen", 40, 100, 10},
	{"Beacon", "Signal Beacon", "dollars", 300, 800, 10},
	{"Artifact", "Tech Artifact", "neuro_cores", 10, 30, 6},
	{"Ruins", "Ancient Ruins", "ether", 15, 40, 4},
	{"Vein", "Crystal Vein", "crystal", 5, 15, 2},
}

func aiRollExplorationTemplate() aiExplorationSiteTemplate {
	total := 0
	for _, t := range aiExplorationTemplates {
		total += t.weight
	}
	roll := rand.Intn(total)
	for _, t := range aiExplorationTemplates {
		if roll < t.weight {
			return t
		}
		roll -= t.weight
	}
	return aiExplorationTemplates[0]
}

const economicCollapseWarningThreshold = 50.0

var aiClanLeaveChance = 0.005

func economicCollapseWarning(remainingTotal float64) string {
	if remainingTotal >= economicCollapseWarningThreshold {
		return ""
	}
	return "\n\n💀 " + htmlBoldTick("ECONOMIC COLLAPSE") + ": Sustained raiding has left this outpost's stockpiles critically depleted."
}

func isRealPlayer(userID int64) bool {
	return userID > 0
}

func (e *Engine) growAICivilizations(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "SELECT id, level, user_id, name FROM encampments WHERE is_ai_faction = TRUE AND is_defeated = FALSE")
	if err != nil {
		return fmt.Errorf("querying AI civilizations: %w", err)
	}
	type faction struct {
		id     string
		level  int
		userID int64
		name   string
	}
	var factions []faction
	for rows.Next() {
		var f faction
		if err := rows.Scan(&f.id, &f.level, &f.userID, &f.name); err == nil {
			factions = append(factions, f)
		}
	}
	rows.Close()

	for _, f := range factions {
		storageCapacity := storagecap.CapFor(ctx, tx, f.id)

		for _, res := range []struct {
			column string
			gain   float64
		}{
			{"scrap", float64(f.level) * 2.0},
			{"metal", float64(f.level) * 1.0},
			{"rations", float64(f.level) * 0.75},
			{"electricity", float64(f.level) * 0.5},
		} {
			var current float64
			query := fmt.Sprintf("SELECT COALESCE(%s,0) FROM resources WHERE encampment_id = $1", res.column)
			_ = tx.QueryRowContext(ctx, query, f.id).Scan(&current)
			newAmount, _ := storagecap.Clamp(current, res.gain, storageCapacity)
			updateQuery := fmt.Sprintf("UPDATE resources SET %s = $1 WHERE encampment_id = $2", res.column)
			_, _ = tx.ExecContext(ctx, updateQuery, newAmount, f.id)
		}

		var currentCrystal float64
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(crystal,0) FROM resources WHERE encampment_id = $1", f.id).Scan(&currentCrystal)
		newCrystal, _ := storagecap.Clamp(currentCrystal, float64(f.level)*0.02, storageCapacity)
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET crystal = $1 WHERE encampment_id = $2", newCrystal, f.id)

		var currentNeuroCores float64
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(neuro_cores,0) FROM resources WHERE encampment_id = $1", f.id).Scan(&currentNeuroCores)
		newNeuroCores, _ := storagecap.Clamp(currentNeuroCores, float64(f.level)*0.1, storageCapacity)
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET neuro_cores = $1 WHERE encampment_id = $2", newNeuroCores, f.id)

		if rand.Float64() < 0.03 {
			var listingID, itemType, sellerID string
			var qty int
			var price float64
			err := tx.QueryRowContext(ctx, `
				SELECT id, item_type, quantity, price_dollars, seller_id FROM market_exchange
				WHERE is_sold = FALSE AND seller_id != $1 AND ask_type = 'dollars'
				ORDER BY random() LIMIT 1`, f.id).Scan(&listingID, &itemType, &qty, &price, &sellerID)
			if err == nil {
				var buyerDollars float64
				_ = tx.QueryRowContext(ctx, "SELECT COALESCE(dollars,0) FROM resources WHERE encampment_id = $1", f.id).Scan(&buyerDollars)
				if buyerDollars >= price {
					var isSold bool
					lockErr := tx.QueryRowContext(ctx, "SELECT is_sold FROM market_exchange WHERE id = $1 FOR UPDATE", listingID).Scan(&isSold)
					if lockErr == nil && !isSold {
						_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars - $1 WHERE encampment_id = $2", price, f.id)

						var sellerDollars float64
						_ = tx.QueryRowContext(ctx, "SELECT COALESCE(dollars,0) FROM resources WHERE encampment_id = $1", sellerID).Scan(&sellerDollars)
						sellerCap := storagecap.CapFor(ctx, tx, sellerID)
						newSellerDollars, _ := storagecap.Clamp(sellerDollars, price, sellerCap)
						_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = $1 WHERE encampment_id = $2", newSellerDollars, sellerID)

						columnName := "metal"
						switch itemType {
						case "crystal", "scrap":
							columnName = itemType
						}
						var buyerCurrent float64
						_ = tx.QueryRowContext(ctx, fmt.Sprintf("SELECT COALESCE(%s,0) FROM resources WHERE encampment_id = $1", columnName), f.id).Scan(&buyerCurrent)
						buyerCap := storagecap.CapFor(ctx, tx, f.id)
						newBuyerVal, _ := storagecap.Clamp(buyerCurrent, float64(qty), buyerCap)
						_, _ = tx.ExecContext(ctx, fmt.Sprintf("UPDATE resources SET %s = $1 WHERE encampment_id = $2", columnName), newBuyerVal, f.id)

						_, _ = tx.ExecContext(ctx, "UPDATE market_exchange SET is_sold = TRUE WHERE id = $1", listingID)

						var sellerUserID int64
						_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", sellerID).Scan(&sellerUserID)
						if sellerUserID > 0 {
							alertMsg := fmt.Sprintf("💱 <b>MARKET SALE</b>: An AI faction has purchased your public listing for <code>%d</code> %s. +<code>$%.0f</code> transferred instantly to reserves.",
								qty, htmlEscapeTick(itemType), price)
							_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", sellerUserID, alertMsg)
						}
					}
				}
			}
		}

		if rand.Float64() < 0.02 {
			var alreadyInClan bool
			_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM user_clans WHERE user_id = $1)", f.userID).Scan(&alreadyInClan)
			if !alreadyInClan {
				if rand.Float64() < 0.5 {
					var clanID string
					if err := tx.QueryRowContext(ctx, "INSERT INTO clans (name, leader_id) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING RETURNING id", f.name+" Alliance", f.userID).Scan(&clanID); err == nil && clanID != "" {
						_, _ = tx.ExecContext(ctx, "INSERT INTO user_clans (user_id, clan_id, role) VALUES ($1, $2, 'Leader')", f.userID, clanID)
					}
				} else {
					var clanID, clanName string
					err := tx.QueryRowContext(ctx, `
						SELECT c.id, c.name FROM clans c
						WHERE c.recruiting = TRUE AND c.leader_id > 0
						ORDER BY random() LIMIT 1`).Scan(&clanID, &clanName)
					if err == nil {
						if _, err := tx.ExecContext(ctx, "INSERT INTO clan_applications (clan_id, user_id, status) VALUES ($1, $2, 'pending') ON CONFLICT (clan_id, user_id) DO UPDATE SET status = 'pending'", clanID, f.userID); err == nil {
							var leaderID int64
							_ = tx.QueryRowContext(ctx, "SELECT leader_id FROM clans WHERE id = $1", clanID).Scan(&leaderID)
							alertMsg := fmt.Sprintf("📬 <b>NEW CLAN APPLICATION</b>: <code>%s</code> wants to join <code>%s</code>! Review it via <code>/clan</code>.", htmlEscapeTick(f.name), htmlEscapeTick(clanName))
							_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", leaderID, alertMsg)
						}
					}
				}
			}
		}

		if rand.Float64() < aiClanLeaveChance {
			var myClanID, myRole string
			if err := tx.QueryRowContext(ctx, "SELECT clan_id, role FROM user_clans WHERE user_id = $1", f.userID).Scan(&myClanID, &myRole); err == nil {
				if myRole == "Leader" {
					var clanName string
					_ = tx.QueryRowContext(ctx, "SELECT name FROM clans WHERE id = $1", myClanID).Scan(&clanName)
					memberRows, memErr := tx.QueryContext(ctx, "SELECT user_id FROM user_clans WHERE clan_id = $1 AND user_id != $2", myClanID, f.userID)
					var realMemberIDs []int64
					if memErr == nil {
						for memberRows.Next() {
							var uid int64
							if scanErr := memberRows.Scan(&uid); scanErr == nil && uid > 0 {
								realMemberIDs = append(realMemberIDs, uid)
							}
						}
						memberRows.Close()
					}
					if _, err := tx.ExecContext(ctx, "DELETE FROM clans WHERE id = $1", myClanID); err == nil {
						disbandMsg := fmt.Sprintf("💥 <b>CLAN DISBANDED</b>: %s's Leader, the AI faction <code>%s</code>, has moved on. Your Clan no longer exists - form or join a new one via <code>/clan</code>.", htmlEscapeTick(clanName), htmlEscapeTick(f.name))
						for _, uid := range realMemberIDs {
							_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", uid, disbandMsg)
						}
					}
				} else {
					_, _ = tx.ExecContext(ctx, "DELETE FROM user_clans WHERE user_id = $1", f.userID)
				}
			}
		}

		if rand.Float64() < 0.05 {
			var myLedClanID string
			if err := tx.QueryRowContext(ctx, "SELECT clan_id FROM user_clans WHERE user_id = $1 AND role = 'Leader'", f.userID).Scan(&myLedClanID); err == nil {
				appRows, appErr := tx.QueryContext(ctx, "SELECT user_id FROM clan_applications WHERE clan_id = $1 AND status = 'pending'", myLedClanID)
				if appErr == nil {
					var applicantIDs []int64
					for appRows.Next() {
						var uid int64
						if scanErr := appRows.Scan(&uid); scanErr == nil {
							applicantIDs = append(applicantIDs, uid)
						}
					}
					appRows.Close()

					for _, applicantID := range applicantIDs {
						var members int
						_ = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_clans WHERE clan_id = $1", myLedClanID).Scan(&members)
						if members >= 15 {
							break
						}
						var alreadyIn bool
						_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM user_clans WHERE user_id = $1)", applicantID).Scan(&alreadyIn)
						if alreadyIn {
							_, _ = tx.ExecContext(ctx, "DELETE FROM clan_applications WHERE clan_id = $1 AND user_id = $2", myLedClanID, applicantID)
							continue
						}
						_, _ = tx.ExecContext(ctx, "INSERT INTO user_clans (user_id, clan_id, role) VALUES ($1, $2, 'Soldier')", applicantID, myLedClanID)
						_, _ = tx.ExecContext(ctx, "DELETE FROM clan_applications WHERE clan_id = $1 AND user_id = $2", myLedClanID, applicantID)
						if applicantID > 0 {
							_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", applicantID, "✅ <b>APPLICATION ACCEPTED</b>: Welcome to your new Clan! 🎉")
						}
					}
				}
			}
		}

		if rand.Float64() < 0.01 {
			var bestAffordable *gear.Definition
			for i := range gear.Catalog {
				def := gear.Catalog[i]
				if !def.Craftable {
					continue
				}
				if bestAffordable != nil && def.ResourceBonusPct <= bestAffordable.ResourceBonusPct {
					continue
				}
				if gear.AffordCraft(ctx, tx, f.id, def) {
					bestAffordable = &gear.Catalog[i]
				}
			}
			if bestAffordable != nil {
				for resource, amount := range bestAffordable.CraftCost {
					_, _ = tx.ExecContext(ctx, fmt.Sprintf("UPDATE resources SET %s = %s - $1 WHERE encampment_id = $2", resource, resource), amount, f.id)
				}
				_, _ = tx.ExecContext(ctx, `
					INSERT INTO player_gear (encampment_id, gear_key, quantity, equipped)
					VALUES ($1, $2, 1, FALSE)
					ON CONFLICT (encampment_id, gear_key) DO UPDATE SET quantity = player_gear.quantity + 1`,
					f.id, bestAffordable.Key)
				_ = gear.Equip(ctx, tx, f.id, bestAffordable.Key)
			}
		}

		if rand.Float64() < 0.02 {
			var myClanID string
			if err := tx.QueryRowContext(ctx, "SELECT clan_id FROM user_clans WHERE user_id = $1 AND role = 'Leader'", f.userID).Scan(&myClanID); err == nil {
				var alreadyAtWar bool
				_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM clan_wars WHERE (clan_a_id = $1 OR clan_b_id = $1) AND status = 'active')", myClanID).Scan(&alreadyAtWar)
				if !alreadyAtWar {
					var enemyID, enemyName string
					err := tx.QueryRowContext(ctx, `
						SELECT id, name FROM clans
						WHERE id != $1
						AND id NOT IN (SELECT clan_a_id FROM clan_wars WHERE status = 'active' UNION SELECT clan_b_id FROM clan_wars WHERE status = 'active')
						ORDER BY random() LIMIT 1`, myClanID).Scan(&enemyID, &enemyName)
					if err == nil {
						endsAt := time.Now().UTC().Add(48 * time.Hour)
						if _, err := tx.ExecContext(ctx, "INSERT INTO clan_wars (clan_a_id, clan_b_id, ends_at) VALUES ($1, $2, $3)", myClanID, enemyID, endsAt); err == nil {
							var myClanName string
							_ = tx.QueryRowContext(ctx, "SELECT name FROM clans WHERE id = $1", myClanID).Scan(&myClanName)
							alert := fmt.Sprintf("🚨⚔️ <b>CLAN WAR DECLARED!</b> ⚔️🚨\n\n%s has declared war on your Clan! Duration: <code>48 hours</code>.\n📊 Every successful raid your Clan members win against enemy Clan members earns War Score.",
								htmlEscapeTick(myClanName))
							rows, _ := tx.QueryContext(ctx, "SELECT user_id FROM user_clans WHERE clan_id IN ($1, $2)", myClanID, enemyID)
							if rows != nil {
								var memberIDs []int64
								for rows.Next() {
									var uid int64
									if rows.Scan(&uid) == nil {
										memberIDs = append(memberIDs, uid)
									}
								}
								rows.Close()
								for _, uid := range memberIDs {
									if uid > 0 {
										_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", uid, alert)
									}
								}
							}
						}
					}
				}
			}
		}

		if rand.Float64() < 0.01 {
			var myClanID string
			if err := tx.QueryRowContext(ctx, "SELECT clan_id FROM user_clans WHERE user_id = $1 AND role = 'Leader'", f.userID).Scan(&myClanID); err == nil {
				var existingFed sql.NullString
				_ = tx.QueryRowContext(ctx, "SELECT federation_id FROM clans WHERE id = $1", myClanID).Scan(&existingFed)
				if !existingFed.Valid {
					const aiFederationFoundCost = 5000.0
					if rand.Float64() < 0.5 {
						var crystal float64
						_ = tx.QueryRowContext(ctx, "SELECT COALESCE(crystal,0) FROM resources WHERE encampment_id = $1", f.id).Scan(&crystal)
						if crystal >= aiFederationFoundCost {
							var myClanName string
							_ = tx.QueryRowContext(ctx, "SELECT name FROM clans WHERE id = $1", myClanID).Scan(&myClanName)
							var fedID string
							if err := tx.QueryRowContext(ctx, "INSERT INTO federations (name, founder_clan_id) VALUES ($1, $2) ON CONFLICT (name) DO NOTHING RETURNING id", myClanName+" Federation", myClanID).Scan(&fedID); err == nil && fedID != "" {
								_, _ = tx.ExecContext(ctx, "UPDATE resources SET crystal = crystal - $1 WHERE encampment_id = $2", aiFederationFoundCost, f.id)
								_, _ = tx.ExecContext(ctx, "UPDATE clans SET federation_id = $1 WHERE id = $2", fedID, myClanID)
							}
						}
					} else {
						var fedID string
						if err := tx.QueryRowContext(ctx, "SELECT id FROM federations ORDER BY random() LIMIT 1").Scan(&fedID); err == nil {
							_, _ = tx.ExecContext(ctx, "UPDATE clans SET federation_id = $1 WHERE id = $2", fedID, myClanID)
						}
					}
				}
			}
		}

		if rand.Float64() < 0.02 {
			var myClanID string
			if err := tx.QueryRowContext(ctx, "SELECT clan_id FROM user_clans WHERE user_id = $1 AND role = 'Leader'", f.userID).Scan(&myClanID); err == nil {
				var pactID string
				err := tx.QueryRowContext(ctx, `
					SELECT id FROM clan_diplomacy
					WHERE (clan_a_id = $1 OR clan_b_id = $1) AND status = 'pending' AND proposed_by != $2
					ORDER BY random() LIMIT 1`, myClanID, f.userID).Scan(&pactID)
				if err == nil {
					newStatus := "rejected"
					if rand.Float64() < 0.8 {
						newStatus = "active"
					}
					_, _ = tx.ExecContext(ctx, "UPDATE clan_diplomacy SET status = $1, responded_at = CURRENT_TIMESTAMP WHERE id = $2", newStatus, pactID)
				} else {
					var targetClanID, targetClanName string
					err := tx.QueryRowContext(ctx, `
						SELECT c.id, c.name FROM clans c
						WHERE c.id != $1
						AND NOT EXISTS (
							SELECT 1 FROM clan_diplomacy cd
							WHERE ((cd.clan_a_id = $1 AND cd.clan_b_id = c.id) OR (cd.clan_a_id = c.id AND cd.clan_b_id = $1))
							AND cd.status IN ('pending', 'active')
						)
						ORDER BY random() LIMIT 1`, myClanID).Scan(&targetClanID, &targetClanName)
					if err == nil {
						pactType := "nap"
						if rand.Float64() < 0.5 {
							pactType = "alliance"
						}
						if _, err := tx.ExecContext(ctx, "INSERT INTO clan_diplomacy (clan_a_id, clan_b_id, pact_type, proposed_by) VALUES ($1, $2, $3, $4)", myClanID, targetClanID, pactType, f.userID); err == nil {
							var targetLeaderID int64
							_ = tx.QueryRowContext(ctx, "SELECT leader_id FROM clans WHERE id = $1", targetClanID).Scan(&targetLeaderID)
							if targetLeaderID > 0 {
								label := "🕊️ Non-Aggression Pact"
								if pactType == "alliance" {
									label = "🤝 Alliance"
								}
								alertMsg := fmt.Sprintf("🕊️ <b>DIPLOMATIC PROPOSAL</b>: %s proposed to your Clan by an AI-led Clan! Review it via <code>/diplomacy</code>.", label)
								_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", targetLeaderID, alertMsg)
							}
						}
					}
				}
			}
		}

		if rand.Float64() < 0.02 {
			var alreadyQueued bool
			_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM arena_queue WHERE user_id = $1)", f.userID).Scan(&alreadyQueued)
			if !alreadyQueued {
				const aiArenaSoloEntryFee = 50.0
				var dollars float64
				_ = tx.QueryRowContext(ctx, "SELECT COALESCE(dollars,0) FROM resources WHERE encampment_id = $1", f.id).Scan(&dollars)
				if dollars >= aiArenaSoloEntryFee {
					_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars - $1 WHERE encampment_id = $2", aiArenaSoloEntryFee, f.id)
					_, _ = tx.ExecContext(ctx, "INSERT INTO arena_queue (user_id, bracket) VALUES ($1, 'solo') ON CONFLICT DO NOTHING", f.userID)
				}
			}
		}

		if rand.Float64() < 0.04 {
			_, _ = tx.ExecContext(ctx, "INSERT INTO research_states (encampment_id) VALUES ($1) ON CONFLICT (encampment_id) DO NOTHING", f.id)

			techColumn := aiResearchTechColumns[rand.Intn(len(aiResearchTechColumns))]
			var currentLvl int
			lockQuery := fmt.Sprintf("SELECT %s FROM research_states WHERE encampment_id = $1 FOR UPDATE", techColumn)
			_ = tx.QueryRowContext(ctx, lockQuery, f.id).Scan(&currentLvl)

			if currentLvl < aiResearchMaxLevel {
				cost := currentLvl * aiResearchCostPerLevel
				var neuroCores float64
				_ = tx.QueryRowContext(ctx, "SELECT COALESCE(neuro_cores,0) FROM resources WHERE encampment_id = $1", f.id).Scan(&neuroCores)
				if neuroCores >= float64(cost) {
					_, _ = tx.ExecContext(ctx, "UPDATE resources SET neuro_cores = neuro_cores - $1 WHERE encampment_id = $2", cost, f.id)
					updateQuery := fmt.Sprintf("UPDATE research_states SET %s = %s + 1 WHERE encampment_id = $1", techColumn, techColumn)
					_, _ = tx.ExecContext(ctx, updateQuery, f.id)
				}
			}
		}

		if rand.Float64() < 0.04 {
			var alreadyUpgrading bool
			_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM modules WHERE encampment_id = $1 AND is_upgrading = TRUE)", f.id).Scan(&alreadyUpgrading)
			if !alreadyUpgrading {
				modType := aiUpgradeableModules[rand.Intn(len(aiUpgradeableModules))]

				var currentLvl int
				moduleQuery := `
					INSERT INTO modules (encampment_id, type, level)
					VALUES ($1, $2, 1)
					ON CONFLICT (encampment_id, type)
					DO UPDATE SET level = modules.level
					RETURNING level`
				_ = tx.QueryRowContext(ctx, moduleQuery, f.id, modType).Scan(&currentLvl)

				if currentLvl < f.level {
					cost := currentLvl * aiModuleUpgradeCostPerLevel
					var scrap float64
					_ = tx.QueryRowContext(ctx, "SELECT COALESCE(scrap,0) FROM resources WHERE encampment_id = $1", f.id).Scan(&scrap)
					if scrap >= float64(cost) {
						_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", cost, f.id)
						readyAt := time.Now().UTC().Add(20 * time.Second)
						_, _ = tx.ExecContext(ctx, "UPDATE modules SET is_upgrading = TRUE, upgrade_ready_at = $1 WHERE encampment_id = $2 AND type = $3", readyAt, f.id, modType)
					}
				}
			}
		}

		if rand.Float64() < 0.04 {
			var alreadyDispatched bool
			_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM exploration_dispatches WHERE encampment_id = $1)", f.id).Scan(&alreadyDispatched)
			if !alreadyDispatched {
				var rations, metal float64
				_ = tx.QueryRowContext(ctx, "SELECT COALESCE(rations,0), COALESCE(metal,0) FROM resources WHERE encampment_id = $1", f.id).Scan(&rations, &metal)
				if rations >= aiExplorationDispatchRationsCost && metal >= aiExplorationDispatchMetalCost {
					var region string
					_ = tx.QueryRowContext(ctx, "SELECT c.region FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", f.id).Scan(&region)

					tmpl := aiRollExplorationTemplate()
					sector := rand.Intn(99) + 1
					siteName := fmt.Sprintf("%s (Sector %d)", tmpl.namePrefix, sector)
					rewardAmount := tmpl.minAmount + rand.Float64()*(tmpl.maxAmount-tmpl.minAmount)
					expiresAt := time.Now().UTC().Add(24 * time.Hour)

					var siteID string
					err := tx.QueryRowContext(ctx, `
						INSERT INTO exploration_sites (continent, site_name, site_type, reward_type, reward_amount, expires_at)
						VALUES ($1, $2, $3, $4, $5, $6)
						RETURNING id`, region, siteName, tmpl.siteType, tmpl.rewardType, rewardAmount, expiresAt).Scan(&siteID)
					if err == nil {
						travelMinutes := aiExplorationMinTravelMinutes + rand.Intn(aiExplorationMaxTravelMinutes-aiExplorationMinTravelMinutes+1)
						resolveTime := time.Now().UTC().Add(time.Duration(travelMinutes) * time.Minute)
						_, _ = tx.ExecContext(ctx, "INSERT INTO exploration_dispatches (site_id, encampment_id, user_id, resolve_time) VALUES ($1, $2, $3, $4)",
							siteID, f.id, f.userID, resolveTime)
						_, _ = tx.ExecContext(ctx, "UPDATE resources SET rations = rations - $1, metal = metal - $2 WHERE encampment_id = $3",
							aiExplorationDispatchRationsCost, aiExplorationDispatchMetalCost, f.id)
					}
				}
			}
		}

		var heroInjuries string
		var heroExists bool
		_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM heroes WHERE encampment_id = $1)", f.id).Scan(&heroExists)
		if !heroExists {
			var playerFaction string
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(faction,'') FROM users WHERE telegram_id = $1", f.userID).Scan(&playerFaction)
			heroName, heroTrait, heroInjuriesStart, heroSuperpower := "Waste Phantom", "Salvage Specialist", "Cybernetic Hand", "⚙️ Scrap Recovery (+10% combat scrap loot)"
			if playerFaction == "steel_vanguard" {
				heroName, heroTrait, heroInjuriesStart, heroSuperpower = "Iron Warden", "Fortress Tactician", "Scarred Eye", "🛡️ Kinetic Barrier (Reduces incoming damage by 15%)"
			}
			_, _ = tx.ExecContext(ctx, `
				INSERT INTO heroes (encampment_id, name, trait, injuries, battles_survived, superpower, level, xp)
				VALUES ($1, $2, $3, $4, 0, $5, 1, 0)
				ON CONFLICT (encampment_id) DO NOTHING`,
				f.id, heroName, heroTrait, heroInjuriesStart, heroSuperpower)
		}
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(injuries,'') FROM heroes WHERE encampment_id = $1", f.id).Scan(&heroInjuries)

		if rand.Float64() < 0.03 {
			var scrap float64
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(scrap,0) FROM resources WHERE encampment_id = $1", f.id).Scan(&scrap)
			const aiHeroTrainScrapCost = 50.0
			if scrap >= aiHeroTrainScrapCost {
				var currentLvl, currentXp int
				_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level,1), COALESCE(xp,0) FROM heroes WHERE encampment_id = $1 FOR UPDATE", f.id).Scan(&currentLvl, &currentXp)
				newXp := currentXp + 20
				newLvl := currentLvl
				if newXp >= 100 {
					newLvl++
					newXp = 0
				}
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", aiHeroTrainScrapCost, f.id)
				_, _ = tx.ExecContext(ctx, "UPDATE heroes SET level = $1, xp = $2 WHERE encampment_id = $3", newLvl, newXp, f.id)
			}
		}

		if rand.Float64() < 0.03 && heroInjuries != "" && heroInjuries != "Perfect Health" {
			var rations float64
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(rations,0) FROM resources WHERE encampment_id = $1", f.id).Scan(&rations)
			const aiHeroHealRationsCost = 50.0
			if rations >= aiHeroHealRationsCost {
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET rations = rations - $1 WHERE encampment_id = $2", aiHeroHealRationsCost, f.id)
				_, _ = tx.ExecContext(ctx, "UPDATE heroes SET injuries = 'Perfect Health' WHERE encampment_id = $1", f.id)
			}
		}

		if rand.Float64() < 0.03 {
			var metal, crystal float64
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(metal,0), COALESCE(crystal,0) FROM resources WHERE encampment_id = $1", f.id).Scan(&metal, &crystal)
			switch {
			case crystal >= 40.0:
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET crystal = crystal - 20.0 WHERE encampment_id = $1", f.id)
				_, _ = tx.ExecContext(ctx, "INSERT INTO market_exchange (seller_id, item_type, quantity, price_dollars, is_sold) VALUES ($1, 'crystal', 20, 300.0, FALSE)", f.id)
			case metal >= 100.0:
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - 50.0 WHERE encampment_id = $1", f.id)
				_, _ = tx.ExecContext(ctx, "INSERT INTO market_exchange (seller_id, item_type, quantity, price_dollars, is_sold) VALUES ($1, 'metal', 50, 150.0, FALSE)", f.id)
			}
		}

		if rand.Float64() < 0.05 {
			var soldiers, mechs int
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers,0), COALESCE(mechs,0) FROM workshop_inventory WHERE encampment_id = $1", f.id).Scan(&soldiers, &mechs)
			maxSoldiers := f.level * aiCivilizationMaxSoldiersPerLevel
			maxMechs := f.level * aiCivilizationMaxMechsPerLevel
			if soldiers < maxSoldiers {
				_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + 1 WHERE encampment_id = $1", f.id)
			} else if mechs < maxMechs {
				_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET mechs = mechs + 1 WHERE encampment_id = $1", f.id)
			} else if f.level < 15 {
				_, _ = tx.ExecContext(ctx, "UPDATE encampments SET level = level + 1 WHERE id = $1", f.id)
			}
		}
	}
	return nil
}

func (e *Engine) emitRaidRadarWarnings(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT r.id, r.resolve_time, COALESCE(r.base_march_minutes, 15.0),
		       ea.name, ed.user_id, rf.route_type,
		       COALESCE((SELECT m.level FROM modules m WHERE m.encampment_id = ed.id AND m.type = 'radar'), 0),
		       COALESCE((SELECT wi.scouts FROM workshop_inventory wi WHERE wi.encampment_id = ed.id), 0),
		       COALESCE((SELECT wi.observers FROM workshop_inventory wi WHERE wi.encampment_id = ed.id), 0)
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		JOIN encampments ed ON ed.id = r.defender_id
		JOIN raid_forces rf ON rf.raid_id = r.id
		WHERE r.state = 'marching'
		  AND r.radar_alert_sent_at IS NULL
		  AND r.resolve_time > CURRENT_TIMESTAMP`)
	if err != nil {
		return fmt.Errorf("querying pending raid radar warnings: %w", err)
	}
	defer rows.Close()

	type pendingRadarAlert struct {
		raidID       string
		resolveTime  time.Time
		baseMinutes  float64
		attackerName string
		defenderID   int64
		routeType    string
		radarLevel   int
		scouts       int
		observers    int
	}
	var alerts []pendingRadarAlert
	for rows.Next() {
		var alert pendingRadarAlert
		if err := rows.Scan(&alert.raidID, &alert.resolveTime, &alert.baseMinutes,
			&alert.attackerName, &alert.defenderID, &alert.routeType,
			&alert.radarLevel, &alert.scouts, &alert.observers); err != nil {
			return fmt.Errorf("scanning pending raid radar warning: %w", err)
		}
		alerts = append(alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterating pending raid radar warnings: %w", err)
	}

	for _, alert := range alerts {
		remaining := time.Until(alert.resolveTime.UTC()).Minutes()
		warningAt := worldintel.RadarWarningMinutes(alert.baseMinutes, alert.radarLevel, alert.scouts, alert.observers, alert.routeType)
		if warningAt <= 0 || remaining > warningAt {
			continue
		}

		result, err := tx.ExecContext(ctx, `
			UPDATE raids SET radar_alert_sent_at = CURRENT_TIMESTAMP
			WHERE id = $1 AND radar_alert_sent_at IS NULL`, alert.raidID)
		if err != nil {
			return fmt.Errorf("marking raid radar warning sent: %w", err)
		}
		if affected, _ := result.RowsAffected(); affected != 1 {
			continue
		}

		proximityAlert := fmt.Sprintf(
			"🛰️ %s: An offensive fleet is approaching your coordinate perimeter!\nHostile Force: Outpost %s\nEstimated impact: %s. Your radar network detected it at the current warning range.",
			htmlBoldTick("RADAR WARNING"), htmlCodeTick(htmlEscapeTick(alert.attackerName)), htmlCodeTick(fmt.Sprintf("%d minute(s)", int(math.Ceil(remaining)))),
		)
		if isRealPlayer(alert.defenderID) {
			if _, err := tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", alert.defenderID, proximityAlert); err != nil {
				return fmt.Errorf("queueing raid radar warning: %w", err)
			}
		}
	}
	return nil
}

func (e *Engine) resolveRaidCombats(ctx context.Context, tx *sql.Tx) error {
	if err := e.emitRaidRadarWarnings(ctx, tx); err != nil {
		return err
	}

	query := `
		SELECT r.id, r.attacker_id, r.defender_id, r.state, r.round_number,
		       ea.name as attacker_name, ea.user_id as attacker_user_id,
		       COALESCE(ed.name, 'Rogue Drone Nest') as defender_name, 
		       COALESCE(ed.user_id, 0) as defender_user_id, r.resolve_time, 
		       COALESCE(r.stolen_scrap, 0.0) as stolen_scrap,
		       COALESCE(r.stolen_metal, 0.0) as stolen_metal,
		       COALESCE(r.stolen_crystal, 0.0) as stolen_crystal,
		       COALESCE(r.stolen_rations, 0.0) as stolen_rations,
		       COALESCE(r.stolen_electricity, 0.0) as stolen_electricity,
		       COALESCE(r.stolen_hydrogen, 0.0) as stolen_hydrogen,
		       COALESCE(r.stolen_neuro_cores, 0.0) as stolen_neuro_cores,
		       COALESCE(r.stolen_dollars, 0.0) as stolen_dollars,
		       COALESCE(r.base_march_minutes, 15.0) as base_march_minutes,
		       COALESCE(r.attacker_rations, 100.0) as attacker_rations,
		       COALESCE(r.attacker_ammo, 100.0) as attacker_ammo,
		       COALESCE(r.attacker_losses, 0) as attacker_losses,
		       COALESCE(r.defender_losses, 0) as defender_losses
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		LEFT JOIN encampments ed ON ed.id = r.defender_id
		WHERE ((r.state = 'marching' OR r.state = 'returning') AND COALESCE(r.movement_state, 'moving') = 'moving')
		   OR r.state = 'engaged'`

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed querying combat raids: %w", err)
	}
	defer rows.Close()

	type activeRaid struct {
		id                string
		attackerID        string
		defenderID        sql.NullString
		state             string
		roundNumber       int
		attackerName      string
		attackerUserID    int64
		defenderName      string
		defenderUserID    int64
		resolveTime       time.Time
		stolenScrap       float64
		stolenMetal       float64
		stolenCrystal     float64
		stolenRations     float64
		stolenElectricity float64
		stolenHydrogen    float64
		stolenNeuroCores  float64
		stolenDollars     float64
		baseMarchMinutes  float64
		attackerRations   float64
		attackerAmmo      float64
		attackerLosses    int
		defenderLosses    int
	}

	var raids []activeRaid
	for rows.Next() {
		var r activeRaid
		err := rows.Scan(
			&r.id, &r.attackerID, &r.defenderID, &r.state, &r.roundNumber,
			&r.attackerName, &r.attackerUserID,
			&r.defenderName, &r.defenderUserID, &r.resolveTime, &r.stolenScrap,
			&r.stolenMetal, &r.stolenCrystal, &r.stolenRations, &r.stolenElectricity,
			&r.stolenHydrogen, &r.stolenNeuroCores, &r.stolenDollars, &r.baseMarchMinutes,
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
			var soldiersMob, mechsMob, buggiesMob, shipsMob, jetsMob, nukesMob, destroyersMob, bombersMob, bcMob, dsMob, liberatorsMob, wraithsMob int
			var haulersMob, tankersMob, cargoMk1Mob, cargoMk2Mob, cargoMk3Mob int
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers_mobilized, 0), COALESCE(mechs_mobilized, 0), COALESCE(buggies_mobilized, 0), COALESCE(ships_mobilized, 0), COALESCE(jets_mobilized, 0), COALESCE(nukes_mobilized, 0), COALESCE(destroyers_mobilized, 0), COALESCE(bombers_mobilized, 0), COALESCE(battlecruisers_mobilized, 0), COALESCE(deathstars_mobilized, 0), COALESCE(liberators_mobilized, 0), COALESCE(wraiths_mobilized, 0), COALESCE(haulers_mobilized, 0), COALESCE(tankers_mobilized, 0), COALESCE(cargo_mk1_mobilized, 0), COALESCE(cargo_mk2_mobilized, 0), COALESCE(cargo_mk3_mobilized, 0) FROM raid_forces WHERE raid_id = $1", r.id).Scan(&soldiersMob, &mechsMob, &buggiesMob, &shipsMob, &jetsMob, &nukesMob, &destroyersMob, &bombersMob, &bcMob, &dsMob, &liberatorsMob, &wraithsMob, &haulersMob, &tankersMob, &cargoMk1Mob, &cargoMk2Mob, &cargoMk3Mob)

			_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2, buggies = buggies + $3, ships = ships + $4, jets = jets + $5, nukes = nukes + $6, destroyers = destroyers + $7, bombers = bombers + $8, battlecruisers = battlecruisers + $9, deathstars = deathstars + $10, liberators = liberators + $11, wraiths = wraiths + $12, haulers = haulers + $13, tankers = tankers + $14, cargo_mk1 = cargo_mk1 + $15, cargo_mk2 = cargo_mk2 + $16, cargo_mk3 = cargo_mk3 + $17 WHERE encampment_id = $18", soldiersMob, mechsMob, buggiesMob, shipsMob, jetsMob, nukesMob, destroyersMob, bombersMob, bcMob, dsMob, liberatorsMob, wraithsMob, haulersMob, tankersMob, cargoMk1Mob, cargoMk2Mob, cargoMk3Mob, r.attackerID)
			var curScrap, curMetal, curCrystal, curRations, curElectricity, curHydrogen, curNeuroCores, curDollars float64
			_ = tx.QueryRowContext(ctx, "SELECT scrap, metal, crystal, rations, electricity, hydrogen, neuro_cores, dollars FROM resources WHERE encampment_id = $1", r.attackerID).Scan(&curScrap, &curMetal, &curCrystal, &curRations, &curElectricity, &curHydrogen, &curNeuroCores, &curDollars)
			attackerCap := storagecap.CapFor(ctx, tx, r.attackerID)
			newScrap, _ := storagecap.Clamp(curScrap, r.stolenScrap, attackerCap)
			newMetal, _ := storagecap.Clamp(curMetal, r.stolenMetal, attackerCap)
			newCrystal, _ := storagecap.Clamp(curCrystal, r.stolenCrystal, attackerCap)
			newRations, _ := storagecap.Clamp(curRations, r.stolenRations, attackerCap)
			newElectricity, _ := storagecap.Clamp(curElectricity, r.stolenElectricity, attackerCap)
			newHydrogen, _ := storagecap.Clamp(curHydrogen, r.stolenHydrogen, attackerCap)
			_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = $1, metal = $2, crystal = $3, rations = $5, electricity = $6, hydrogen = $7, neuro_cores = $8, dollars = $9 WHERE encampment_id = $4", newScrap, newMetal, newCrystal, r.attackerID, newRations, newElectricity, newHydrogen, curNeuroCores+r.stolenNeuroCores, curDollars+r.stolenDollars)
			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'completed' WHERE id = $1", r.id)
			if r.attackerUserID > 0 {
				_ = progression.RecordEvent(ctx, tx, r.attackerUserID, progression.TriggerRaidCompleted, 1)
			}

			rowsCoop, errCoop := tx.QueryContext(ctx, "SELECT encampment_id, soldiers_contributed, mechs_contributed FROM raid_coop_members WHERE raid_id = $1", r.id)
			if errCoop == nil {
				type coopSurvivor struct {
					campID   string
					soldiers int
					mechs    int
				}
				var survivors []coopSurvivor
				for rowsCoop.Next() {
					var s coopSurvivor
					if scanErr := rowsCoop.Scan(&s.campID, &s.soldiers, &s.mechs); scanErr == nil {
						survivors = append(survivors, s)
					}
				}
				rowsCoop.Close()

				for _, s := range survivors {
					_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2 WHERE encampment_id = $3", s.soldiers, s.mechs, s.campID)

					var helperUserID int64
					_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", s.campID).Scan(&helperUserID)
					if isRealPlayer(helperUserID) {
						helperAlert := fmt.Sprintf("🚀 <b>RETURN MARCH COMPLETED</b>: Your contributed forces survived the co-op campaign and have returned safely to your own base (<code>+%d Soldiers, +%d Mechs</code>). 🎖️", s.soldiers, s.mechs)
						_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", helperUserID, helperAlert)
					}
				}
				_, _ = tx.ExecContext(ctx, "DELETE FROM raid_coop_members WHERE raid_id = $1", r.id)
			}

			alertMsg := fmt.Sprintf(
				"🚀 <b>RETURN MARCH COMPLETED</b> 🎉\n\n"+
					"Your survivors returned to base carrying:\n"+
					"⚙️ <code>+%.1f Scrap</code>  🔩 <code>+%.1f Metal</code>  🔮 <code>+%.1f Crystal</code>\n"+
					"🍖 <code>+%.1f Rations</code>  ⚡ <code>+%.1f Electricity</code>  🎈 <code>+%.1f Hydrogen</code>\n"+
					"🧠 <code>+%.1f Neuro Cores</code>  💵 <code>+$%.1f</code>",
				r.stolenScrap, r.stolenMetal, r.stolenCrystal, r.stolenRations, r.stolenElectricity, r.stolenHydrogen, r.stolenNeuroCores, r.stolenDollars,
			)
			queryNotif := "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)"
			if isRealPlayer(r.attackerUserID) {
				_, _ = tx.ExecContext(ctx, queryNotif, r.attackerUserID, alertMsg)
			}
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
				"⚔️ %s\n\n"+
					"Your forces have arrived at Outpost %s. 🎯\n"+
					"Deployments are actively engaged in battlefield skirmishes.\n"+
					"⏳ %s",
				htmlBoldTick("CAMPAIGN ENGAGEMENT ACTIVATED!"),
				htmlBoldTick(htmlEscapeTick(r.defenderName)),
				htmlItalicTick("Decisive Resolution progress starting next tick cycle."),
			)
			if isRealPlayer(r.attackerUserID) {
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.attackerUserID, arrivalAlert)
			}

			if r.defenderID.Valid && isRealPlayer(r.defenderUserID) {
				defenderEngagedAlert := fmt.Sprintf(
					"⚔️ %s\n\n"+
						"Hostile forces from Outpost %s have arrived and are actively engaging your defenses! 🚨\n"+
						"⏳ %s",
					htmlBoldTick("UNDER ATTACK: ENGAGEMENT ACTIVATED!"),
					htmlBoldTick(htmlEscapeTick(r.attackerName)),
					htmlItalicTick("Decisive Resolution progress starting next tick cycle."),
				)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.defenderUserID, defenderEngagedAlert)
			}
			continue
		}

		var primarySoldiers, primaryMechs, primaryBuggies, primaryDestroyers, primaryBombers, primaryBC, primaryDS, primaryLiberators, primaryWraiths, primaryShips, primaryJets int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers_mobilized, 0), COALESCE(mechs_mobilized, 0), COALESCE(buggies_mobilized, 0), COALESCE(destroyers_mobilized, 0), COALESCE(bombers_mobilized, 0), COALESCE(battlecruisers_mobilized, 0), COALESCE(deathstars_mobilized, 0), COALESCE(liberators_mobilized, 0), COALESCE(wraiths_mobilized, 0), COALESCE(ships_mobilized, 0), COALESCE(jets_mobilized, 0) FROM raid_forces WHERE raid_id = $1 FOR UPDATE", r.id).Scan(&primarySoldiers, &primaryMechs, &primaryBuggies, &primaryDestroyers, &primaryBombers, &primaryBC, &primaryDS, &primaryLiberators, &primaryWraiths, &primaryShips, &primaryJets)

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

			_ = tx.QueryRowContext(ctx,
				"SELECT COALESCE(SUM(level), 0) FROM modules WHERE encampment_id = $1 AND type IN ('light_laser', 'heavy_laser', 'gauss_cannon', 'ion_cannon', 'plasma_turret')",
				r.defenderID.String).Scan(&defenderTurretLevels)

			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'light_laser'", r.defenderID.String).Scan(&defenderLightLaserLvl)
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'heavy_laser'", r.defenderID.String).Scan(&defenderHeavyLaserLvl)
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'gauss_cannon'", r.defenderID.String).Scan(&defenderGaussCannonLvl)
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'ion_cannon'", r.defenderID.String).Scan(&defenderIonCannonLvl)
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(level, 0) FROM modules WHERE encampment_id = $1 AND type = 'plasma_turret'", r.defenderID.String).Scan(&defenderPlasmaTurretLvl)

			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(guardians, 0), COALESCE(observers, 0) FROM workshop_inventory WHERE encampment_id = $1", r.defenderID.String).Scan(&defenderGuardians, &defenderObservers)

			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(superpower, '') FROM heroes WHERE encampment_id = $1", r.defenderID.String).Scan(&defenderHeroSuperpower)
			_ = tx.QueryRowContext(ctx, "SELECT (orbital_buff_until IS NOT NULL AND orbital_buff_until > CURRENT_TIMESTAMP) FROM encampments WHERE id = $1", r.defenderID.String).Scan(&defenderOrbitalBuffActive)
		}

		defenseForce := soldiersDefender + dronesDefender + jetsDefender + mechsDefender

		var attackerRegion string
		_ = tx.QueryRowContext(ctx, "SELECT c.region FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", r.attackerID).Scan(&attackerRegion)
		activeWeather := world.ActiveEventFor(ctx, tx, attackerRegion)

		orbitalBonus := 0.0
		if defenderOrbitalBuffActive {
			orbitalBonus = 0.30
		}

		offenseRatingModifier := 1.0

		lightLaserBonus := float64(defenderLightLaserLvl) * 0.02
		heavyLaserBonus := float64(defenderHeavyLaserLvl) * 0.03 * (1.0 + math.Min(float64(totSoldiers)*0.005, 0.5))
		gaussCannonBonus := float64(defenderGaussCannonLvl) * 0.05 * (1.0 + math.Min(float64(totMechs+totLiberators)*0.02, 1.0))
		ionCannonBonus := float64(defenderIonCannonLvl) * 0.05 * (1.0 + math.Min(float64(totDestroyers+totWraiths)*0.03, 1.0))
		plasmaTurretBonus := float64(defenderPlasmaTurretLvl) * 0.08
		turretDefenseBonus := lightLaserBonus + heavyLaserBonus + gaussCannonBonus + ionCannonBonus + plasmaTurretBonus

		if totWraiths > 0 {
			turretDefenseBonus *= 1.0 - math.Min(float64(totWraiths)*0.03, 0.40)
		}

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

		mechOffenseCount := totMechs

		var weatherNotice string = ""
		switch activeWeather {
		case "solar_flare":
			offenseRatingModifier *= (0.80 + rand.Float64()*0.40)
			weatherNotice = "\n⚠️ SOLAR FLARE ACTIVE: Scrambling electronic communication and target locks! Offensive variance fluctuated."
		case "radiation_storm":
			offenseRatingModifier *= 0.75
			weatherNotice = "\n⚠️ RADIATION STORM ACTIVE: Fallout morale penalty applied! Base offense rating reduced by 25%."
		case "acid_rain":
			mechOffenseCount = totMechs / 2
			weatherNotice = "\n⚠️ ACID RAIN ACTIVE: Corrosive rain detected! Armored Mech defensive structures degraded by 50%."
		case "emp":
			offenseRatingModifier *= 0.60
			weatherNotice = "\n⚠️ EMP ACTIVE: Electromagnetic pulse degraded fleet targeting systems! Offense rating reduced by 40%."
		case "sandstorm":
			offenseRatingModifier *= 0.85
			weatherNotice = "\n⚠️ SANDSTORM ACTIVE: Reduced visibility hampered target acquisition! Offense rating reduced by 15%."
		}

		if r.attackerRations <= 0 || r.attackerAmmo <= 0 {
			offenseRatingModifier *= 0.50
		}

		mechOffenseMultiplier := 1.50 * (1.0 + float64(attackerMilitaryTechLvl-1)*0.25)

		var attackerHighTechOffline bool
		_ = tx.QueryRowContext(ctx, "SELECT high_tech_offline FROM raids WHERE id = $1", r.id).Scan(&attackerHighTechOffline)
		if attackerHighTechOffline {
			mechOffenseMultiplier = 1.0
		}

		attRating := (float64(attackForce) * 15.0 * offenseRatingModifier) * (1.0 + (float64(attackerTanks) * 0.50) + (float64(mechOffenseCount) * mechOffenseMultiplier))

		if totDestroyers > 0 {
			destroyerTargets := dronesDefender + jetsDefender
			destroyerBonus := 1.0 + math.Min(float64(destroyerTargets)*0.02, 1.0)
			attRating += float64(totDestroyers) * 20.0 * offenseRatingModifier * destroyerBonus
		}

		if totBombers > 0 {
			bomberBonus := 1.0 + math.Min(float64(defenderTurretLevels)*0.05, 1.5)
			guardianCounter := 1.0 - math.Min(float64(defenderGuardians)*0.03, 0.60)
			attRating += float64(totBombers) * 18.0 * offenseRatingModifier * bomberBonus * guardianCounter
		}

		if totLiberators > 0 {
			libUnit := content.MustFindUnit("liberator")
			attRating += float64(totLiberators) * libUnit.AttackRating * offenseRatingModifier
		}

		if totWraiths > 0 {
			wrUnit := content.MustFindUnit("wraith")
			wraithTargets := dronesDefender + jetsDefender
			wraithBonus := 1.0 + math.Min(float64(wraithTargets)*0.02, 1.0)
			attRating += float64(totWraiths) * wrUnit.AttackRating * offenseRatingModifier * wraithBonus
		}

		if totBC > 0 {
			bcUnit := content.MustFindUnit("battlecruiser")
			attRating += float64(totBC) * bcUnit.AttackRating * offenseRatingModifier
		}

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

		const (
			destroyerToughness = 4.0
			bomberToughness    = 4.0
			bcToughness        = 20.0
			dsToughness        = 200.0
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

			specialistLosses := combatmath.AllocateSpecialistLosses(
				effectiveDbCas,
				combatmath.SpecialistCounts{
					Destroyers:     totDestroyers,
					Bombers:        totBombers,
					Battlecruisers: totBC,
					DoomsdayRigs:   totDS,
					Liberators:     totLiberators,
					Wraiths:        totWraiths,
				},
				combatmath.SpecialistToughness{
					Destroyers:     destroyerToughness,
					Bombers:        bomberToughness,
					Battlecruisers: bcToughness,
					DoomsdayRigs:   dsToughness,
					Liberators:     liberatorToughness,
					Wraiths:        wraithToughness,
				},
			)
			lostAttDestroyers = specialistLosses.Destroyers
			lostAttBombers = specialistLosses.Bombers
			lostAttBC = specialistLosses.Battlecruisers
			lostAttDS = specialistLosses.DoomsdayRigs
			lostAttLiberators = specialistLosses.Liberators
			lostAttWraiths = specialistLosses.Wraiths

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

		var defenderScrap, defenderMetal, defenderCrystal, defenderRations, defenderElectricity, defenderHydrogen, defenderNeuroCores, defenderDollars float64
		if r.defenderID.Valid {
			_ = tx.QueryRowContext(ctx, "SELECT scrap, metal, crystal, rations, electricity, hydrogen, neuro_cores, dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", r.defenderID.String).Scan(&defenderScrap, &defenderMetal, &defenderCrystal, &defenderRations, &defenderElectricity, &defenderHydrogen, &defenderNeuroCores, &defenderDollars)
		} else {
			defenderScrap = 125.0
			defenderMetal = 300.0
			defenderCrystal = 12.0
			defenderRations = 80.0
			defenderElectricity = 60.0
			defenderHydrogen = 15.0
			defenderNeuroCores = 1.0
			defenderDollars = 25.0
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
		stolenCrystal := defenderCrystal * lootPercentage * 0.35
		stolenRations := defenderRations * lootPercentage
		stolenElectricity := defenderElectricity * lootPercentage
		stolenHydrogen := defenderHydrogen * lootPercentage * 0.75
		stolenNeuroCores := defenderNeuroCores * lootPercentage * 0.25
		stolenDollars := defenderDollars * lootPercentage * 0.50
		primaryShare := stolenScrap
		primaryMetalShare := stolenMetal
		primaryCrystalShare := stolenCrystal
		primaryRationsShare := stolenRations
		primaryElectricityShare := stolenElectricity
		primaryHydrogenShare := stolenHydrogen
		primaryNeuroCoresShare := stolenNeuroCores
		primaryDollarsShare := stolenDollars

		var defenderNotes []string
		if defenderTurretLevels > 0 {
			defenderNotes = append(defenderNotes, fmt.Sprintf("🛡️ Defense Grid: %d turret level(s) engaged.", defenderTurretLevels))
		}
		if defenderGuardians > 0 || defenderObservers > 0 {
			defenderNotes = append(defenderNotes, fmt.Sprintf("🤖 %d Guardian(s), 👁️ %d Observer(s) dug in.", defenderGuardians, defenderObservers))
		}
		if defenderShields > 0 {
			defenderNotes = append(defenderNotes, fmt.Sprintf("🧿 Nuclear Shields: %d active.", defenderShields))
		}
		if defenderHeroSuperpower != "" {
			label := "🎖️ Commander's Superpower"
			if !r.defenderID.Valid {
				label = "👑 Warlord's Superpower"
			}
			defenderNotes = append(defenderNotes, fmt.Sprintf("%s: %s.", label, defenderHeroSuperpower))
		}

		var attackerNotes []string
		if attackerHeroSuperpower != "" {
			attackerNotes = append(attackerNotes, fmt.Sprintf("🎖️ Commander's Superpower: %s.", attackerHeroSuperpower))
		}
		if primaryBuggies+primaryShips+primaryJets > 0 {
			attackerNotes = append(attackerNotes, fmt.Sprintf("🚚 Escort: %d Buggy(s), %d Ship(s), %d Jet(s) along for transport only - non-combat, unaffected by this fight.", primaryBuggies, primaryShips, primaryJets))
		}

		report := battlereport.Round{
			Number:        r.roundNumber,
			AttackerName:  r.attackerName,
			DefenderName:  r.defenderName,
			AttackerNotes: attackerNotes,
			DefenderNotes: defenderNotes,
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
				fmt.Sprintf("🔮 %.0f Crystal", primaryCrystalShare),
				fmt.Sprintf("♻️ %.0f Scrap", primaryShare),
				fmt.Sprintf("🍖 %.0f Rations", primaryRationsShare),
				fmt.Sprintf("⚡ %.0f Electricity", primaryElectricityShare),
				fmt.Sprintf("💧 %.0f Hydrogen", primaryHydrogenShare),
				fmt.Sprintf("🧠 %.1f Neuro Cores", primaryNeuroCoresShare),
				fmt.Sprintf("💵 %.0f Dollars", primaryDollarsShare),
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
		if !defenderStillStanding && r.defenderID.Valid {
			remainingTotal := math.Max(defenderScrap-primaryShare, 0) + math.Max(defenderMetal-primaryMetalShare, 0) +
				math.Max(defenderCrystal-primaryCrystalShare, 0) + math.Max(defenderRations-primaryRationsShare, 0) +
				math.Max(defenderElectricity-primaryElectricityShare, 0) + math.Max(defenderDollars-primaryDollarsShare, 0)
			reportText += economicCollapseWarning(remainingTotal)
		}

		var attackerEffect, defenderEffect string
		switch report.Outcome {
		case battlereport.OutcomeAttackerWon:
			attackerEffect, defenderEffect = notifications.EffectCelebration, notifications.EffectPoop
		case battlereport.OutcomeDefenderWon:
			attackerEffect, defenderEffect = notifications.EffectPoop, notifications.EffectCelebration
		}

		if isRealPlayer(r.attackerUserID) {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent, effect_id) VALUES ($1, $2, FALSE, NULLIF($3, ''))", r.attackerUserID, reportText, attackerEffect)
		}
		if r.defenderID.Valid && isRealPlayer(r.defenderUserID) {
			_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent, effect_id) VALUES ($1, $2, FALSE, NULLIF($3, ''))", r.defenderUserID, reportText, defenderEffect)
		}

		if !attackerStillStanding {
			if primaryBuggies > 0 {
				const salvagePercent = 0.08
				salvageScrap := defenderScrap * salvagePercent
				salvageMetal := defenderMetal * salvagePercent
				salvageCrystal := defenderCrystal * salvagePercent

				returnMinutes := r.baseMarchMinutes
				resolveTime := time.Now().UTC().Add(time.Duration(returnMinutes) * time.Minute)

				_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'returning', stolen_scrap = $1, stolen_metal = $2, stolen_crystal = $3, resolve_time = $4, leg_started_at = CURRENT_TIMESTAMP, leg_total_minutes = $6, movement_state = 'moving' WHERE id = $5",
					salvageScrap, salvageMetal, salvageCrystal, resolveTime, r.id, returnMinutes)

				if r.defenderID.Valid {
					_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = GREATEST(scrap - $1, 0), metal = GREATEST(metal - $2, 0), crystal = GREATEST(crystal - $3, 0) WHERE encampment_id = $4",
						salvageScrap, salvageMetal, salvageCrystal, r.defenderID.String)
				}

				salvageAlert := fmt.Sprintf(
					"💀 <b>ASSAULT FORCE WIPED OUT - BUT NOT EMPTY-HANDED</b>\n\n"+
						"Your combat units were destroyed, but %d surviving Buggy crew(s) grabbed what they could before retreating.\n"+
						"⚙️ Salvaged: <code>%.0f Scrap, %.0f Metal, %.0f Crystal</code>.\n"+
						"⏳ Return march engaged, arriving in <code>%.0f minutes</code>.",
					primaryBuggies, salvageScrap, salvageMetal, salvageCrystal, returnMinutes,
				)
				if isRealPlayer(r.attackerUserID) {
					_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.attackerUserID, salvageAlert)
				}
			} else {
				_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'completed' WHERE id = $1", r.id)
				if isRealPlayer(r.attackerUserID) {
					_ = progression.RecordEvent(ctx, tx, r.attackerUserID, progression.TriggerRaidCompleted, 1)
				}
			}
		} else if !defenderStillStanding {
			primRatio := float64(primarySoldiers+primaryMechs) / float64(totSoldiers+totMechs)
			if math.IsNaN(primRatio) {
				primRatio = 1.0
			}
			primaryShare = stolenScrap * primRatio
			primaryMetalShare = stolenMetal * primRatio
			primaryCrystalShare = stolenCrystal * primRatio
			primaryRationsShare = stolenRations * primRatio
			primaryElectricityShare = stolenElectricity * primRatio
			primaryHydrogenShare = stolenHydrogen * primRatio
			primaryNeuroCoresShare = stolenNeuroCores * primRatio
			primaryDollarsShare = stolenDollars * primRatio

			if strings.Contains(attackerHeroSuperpower, "Scrap Recovery") {
				primaryShare *= 1.10
				primaryMetalShare *= 1.10
				primaryCrystalShare *= 1.10
				primaryRationsShare *= 1.10
				primaryElectricityShare *= 1.10
				primaryHydrogenShare *= 1.10
				primaryNeuroCoresShare *= 1.10
				primaryDollarsShare *= 1.10
			}

			var haulers int
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(haulers, 0) FROM workshop_inventory WHERE encampment_id = $1", r.attackerID).Scan(&haulers)

			var cargoMk1, cargoMk2, cargoMk3 int
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(cargo_mk1, 0), COALESCE(cargo_mk2, 0), COALESCE(cargo_mk3, 0) FROM workshop_inventory WHERE encampment_id = $1", r.attackerID).Scan(&cargoMk1, &cargoMk2, &cargoMk3)

			weightFactor := (primaryShare + primaryMetalShare + primaryCrystalShare*4 + primaryRationsShare + primaryElectricityShare + primaryHydrogenShare*2 + primaryNeuroCoresShare*20 + primaryDollarsShare/10) / 5000.0
			if haulers > 0 {
				weightFactor *= 0.50
			}
			if cargoMk1 > 0 {
				weightFactor *= 0.90
			}
			if cargoMk2 > 0 {
				weightFactor *= 0.80
			}
			if cargoMk3 > 0 {
				weightFactor *= 0.65
			}

			returnMinutes := r.baseMarchMinutes * (1.0 + weightFactor)
			returnDuration := time.Duration(returnMinutes) * time.Minute
			resolveTime := time.Now().UTC().Add(returnDuration)

			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'returning', stolen_scrap = $1, stolen_metal = $4, stolen_crystal = $5, stolen_rations = $6, stolen_electricity = $7, stolen_hydrogen = $8, stolen_neuro_cores = $9, stolen_dollars = $10, resolve_time = $2, leg_started_at = CURRENT_TIMESTAMP, leg_total_minutes = $11, movement_state = 'moving' WHERE id = $3", primaryShare, resolveTime, r.id, primaryMetalShare, primaryCrystalShare, primaryRationsShare, primaryElectricityShare, primaryHydrogenShare, primaryNeuroCoresShare, primaryDollarsShare, returnMinutes)

			etaAlert := fmt.Sprintf(
				"🚚 %s\n\nCarrying ⚙️ %s Scrap, 🔩 %s Metal, 🔮 %s Crystal, 🍖 %s Rations, ⚡ %s Electricity, 💧 %s Hydrogen, 🧠 %s Neuro Cores, 💵 %s home.\n⏳ ETA: %s (outbound trip was %s; extra weight from the loot adds travel time).",
				htmlBoldTick("SALVAGE COMPLETE, RETURN MARCH ENGAGED"),
				htmlCodeTick(fmt.Sprintf("%.0f", primaryShare)), htmlCodeTick(fmt.Sprintf("%.0f", primaryMetalShare)), htmlCodeTick(fmt.Sprintf("%.0f", primaryCrystalShare)),
				htmlCodeTick(fmt.Sprintf("%.0f", primaryRationsShare)), htmlCodeTick(fmt.Sprintf("%.0f", primaryElectricityShare)), htmlCodeTick(fmt.Sprintf("%.0f", primaryHydrogenShare)),
				htmlCodeTick(fmt.Sprintf("%.1f", primaryNeuroCoresShare)), htmlCodeTick(fmt.Sprintf("$%.0f", primaryDollarsShare)),
				htmlCodeTick(fmt.Sprintf("%.0f minutes", returnMinutes)), htmlCodeTick(fmt.Sprintf("%.0f minutes", r.baseMarchMinutes)),
			)
			if isRealPlayer(r.attackerUserID) {
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", r.attackerUserID, etaAlert)
			}

			if r.defenderID.Valid {
				_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = GREATEST(scrap - $1, 0), metal = GREATEST(metal - $2, 0), crystal = GREATEST(crystal - $3, 0), rations = GREATEST(rations - $5, 0), electricity = GREATEST(electricity - $6, 0), hydrogen = GREATEST(hydrogen - $7, 0), neuro_cores = GREATEST(neuro_cores - $8, 0), dollars = GREATEST(dollars - $9, 0) WHERE encampment_id = $4", stolenScrap, stolenMetal, stolenCrystal, r.defenderID.String, stolenRations, stolenElectricity, stolenHydrogen, stolenNeuroCores, stolenDollars)

				e.applyClanWarScore(ctx, tx, r.attackerID, r.defenderID.String, primaryShare+primaryMetalShare+primaryCrystalShare)
			}

			for _, h := range helpers {
				_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2 WHERE encampment_id = $3", h.soldiers, h.mechs, h.encampment_id)
			}
		} else if r.roundNumber >= 5 {
			returnMinutes := r.baseMarchMinutes
			resolveTime := time.Now().UTC().Add(time.Duration(returnMinutes) * time.Minute)

			_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'returning', stolen_scrap = 0, stolen_metal = 0, stolen_crystal = 0, resolve_time = $1, leg_started_at = CURRENT_TIMESTAMP, leg_total_minutes = $3, movement_state = 'moving' WHERE id = $2", resolveTime, r.id, returnMinutes)

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
