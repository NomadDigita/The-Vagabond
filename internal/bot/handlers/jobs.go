package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
	"gopkg.in/telebot.v3"
)

type JobsHandler struct {
	DB *sql.DB
}

func NewJobsHandler(db *sql.DB) *JobsHandler {
	return &JobsHandler{DB: db}
}

func (h *JobsHandler) myCamp(ctx context.Context, userID int64) (string, error) {
	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", userID).Scan(&campID)
	return campID, err
}

// HandleHyperSpeed (/newjobhyperspeed) shaves time off your earliest
// active raid's remaining travel, matching the SpaceHunt tip about
// launching HyperSpeed before departing a raid.
func (h *JobsHandler) HandleHyperSpeed(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	const cost = 300.0 // Electricity
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ HyperSpeed activation failed.")
	}
	defer tx.Rollback()

	var electricity float64
	_ = tx.QueryRowContext(ctx, "SELECT electricity FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&electricity)
	if electricity < cost {
		return c.Send(fmt.Sprintf("❌ %s Need %s, you have %s.", htmlBold("Insufficient Electricity!"), htmlCode(fmt.Sprintf("%.0f", cost)), htmlCode(fmt.Sprintf("%.0f", electricity))), telebot.ModeHTML)
	}

	var raidID string
	var resolveTime time.Time
	err = tx.QueryRowContext(ctx, "SELECT id, resolve_time FROM raids WHERE attacker_id = $1 AND state IN ('marching','engaged','returning') ORDER BY resolve_time ASC LIMIT 1 FOR UPDATE", campID).Scan(&raidID, &resolveTime)
	if err != nil {
		return c.Send("❌ No active missions to accelerate. Launch a raid first!")
	}

	remaining := time.Until(resolveTime)
	if remaining < time.Minute {
		return c.Send("⚠️ That mission is about to resolve already - no need for HyperSpeed.")
	}
	newResolve := resolveTime.Add(-remaining / 2) // cuts remaining time in half

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = electricity - $1 WHERE encampment_id = $2", cost, campID)
	_, _ = tx.ExecContext(ctx, "UPDATE raids SET resolve_time = $1 WHERE id = $2", newResolve, raidID)

	if err := tx.Commit(); err != nil {
		return c.Send("⚠️ Error activating HyperSpeed.")
	}

	return c.Send(fmt.Sprintf("🚀⚡ %s Your nearest mission's remaining time was cut in half. New ETA: %s", htmlBold("HYPERSPEED ENGAGED!"), htmlCode(newResolve.UTC().Format("15:04 MST"))), telebot.ModeHTML)
}

// HandleExtendPlanet (/newjobextendplanet) permanently increases storage
// capacity - a real, growing investment rather than a one-time bonus.
func (h *JobsHandler) HandleExtendPlanet(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ Extension failed.")
	}
	defer tx.Rollback()

	var extensionLvl int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(extension_lvl, 0) FROM encampments WHERE id = $1 FOR UPDATE", campID).Scan(&extensionLvl)

	metalCost := float64(500 * (extensionLvl + 1))
	crystalCost := float64(100 * (extensionLvl + 1))

	var metal, crystal float64
	_ = tx.QueryRowContext(ctx, "SELECT metal, crystal FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&metal, &crystal)
	if metal < metalCost || crystal < crystalCost {
		return c.Send(fmt.Sprintf("❌ %s Need %s for extension level %d.", htmlBold("Insufficient Materials!"), htmlCode(fmt.Sprintf("%.0f Metal, %.0f Crystal", metalCost, crystalCost)), extensionLvl+1), telebot.ModeHTML)
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - $1, crystal = crystal - $2 WHERE encampment_id = $3", metalCost, crystalCost, campID)
	_, _ = tx.ExecContext(ctx, "UPDATE encampments SET extension_lvl = extension_lvl + 1 WHERE id = $1", campID)

	if err := tx.Commit(); err != nil {
		return c.Send("⚠️ Error extending planet.")
	}

	return c.Send(fmt.Sprintf("🌍✅ %s Storage capacity +1000 permanently (extension level %d). Next extension: %s.", htmlBold("PLANET EXTENDED!"), extensionLvl+1, htmlCode(fmt.Sprintf("%.0f Metal, %.0f Crystal", metalCost*2, crystalCost*2))), telebot.ModeHTML)
}

// HandleTeleport (/newjobteleport) relocates your outpost to a fresh
// random coordinate, on a cooldown to prevent spam-hopping.
func (h *JobsHandler) HandleTeleport(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	var lastTeleport sql.NullTime
	_ = h.DB.QueryRowContext(ctx, "SELECT last_teleport_at FROM encampments WHERE id = $1", campID).Scan(&lastTeleport)
	if lastTeleport.Valid && time.Since(lastTeleport.Time) < 24*time.Hour {
		remaining := 24*time.Hour - time.Since(lastTeleport.Time)
		return c.Send(fmt.Sprintf("⏳ Teleport is on cooldown for another %.1f hours.", remaining.Hours()))
	}

	const cost = 1000.0 // Electricity
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ Teleport failed.")
	}
	defer tx.Rollback()

	var electricity float64
	_ = tx.QueryRowContext(ctx, "SELECT electricity FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&electricity)
	if electricity < cost {
		return c.Send(fmt.Sprintf("❌ %s Need %s.", htmlBold("Insufficient Electricity!"), htmlCode(fmt.Sprintf("%.0f", cost))), telebot.ModeHTML)
	}

	newX := rand.Intn(10000)
	newY := rand.Intn(10000)
	biomes := []string{"wasteland", "irradiated_zone", "scrapyard", "ashfields", "frozen_tundra"}
	terrains := []string{"flat", "mountainous", "coastal", "urban_ruins"}
	biome := biomes[rand.Intn(len(biomes))]
	terrain := terrains[rand.Intn(len(terrains))]

	var newCoordID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO coordinates (x, y, biome, danger_level, region, terrain) 
		VALUES ($1, $2, $3, $4, $5, $6) 
		ON CONFLICT (x, y) DO UPDATE SET x = EXCLUDED.x
		RETURNING id`, newX, newY, biome, rand.Intn(5)+1, "Unknown Sector", terrain).Scan(&newCoordID)
	if err != nil {
		return c.Send("⚠️ Error finding new coordinates.")
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = electricity - $1 WHERE encampment_id = $2", cost, campID)
	_, _ = tx.ExecContext(ctx, "UPDATE encampments SET coordinate_id = $1, last_teleport_at = CURRENT_TIMESTAMP WHERE id = $2", newCoordID, campID)

	if err := tx.Commit(); err != nil {
		return c.Send("⚠️ Error completing teleport.")
	}

	return c.Send(fmt.Sprintf("🌀✨ %s Your outpost now stands at %s in a %s biome.", htmlBold("TELEPORT COMPLETE!"), htmlCode(fmt.Sprintf("[%d, %d]", newX, newY)), biome), telebot.ModeHTML)
}

// ghostProtocolCooldown and ghostProtocolCostFraction are deliberately
// severe compared to /newjobteleport's cheap, frequent utility cost -
// per the project owner's own words ("very very very costly," "once in
// 3 months"). Cost is a fraction of *current* holdings rather than a
// fixed absolute number, so it stays meaningfully painful regardless of
// where the live economy's typical balances land - there's no historical
// usage data yet to calibrate a fixed number against (see
// AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 3.4 and its "Open
// questions" item 1). Treat both as tunable constants to revisit after a
// few weeks of real usage, same spirit as aidecisions.go's
// aiMaxLevelsBelowSelfForFairTarget.
const (
	ghostProtocolCooldown     = 90 * 24 * time.Hour
	ghostProtocolCostFraction = 0.50
)

// HandleGhostProtocol (/ghostprotocol) is a separate, far more severe
// action than /newjobteleport - see
// AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 3.4 for why the
// existing cheap/frequent teleport was deliberately NOT repurposed for
// this. In addition to relocating (reusing /newjobteleport's random-
// coordinate logic), this deletes every known_locations row where this
// encampment is the target - every scout/attacker who'd locked this
// base's position loses that lock and must rediscover it from scratch.
// encampment_discoveries (the permanent "have you ever heard of this
// entity" relationship) is untouched - only the coordinate lock resets.
func (h *JobsHandler) HandleGhostProtocol(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	message, err := h.doGhostProtocol(ctx, campID)
	if err != nil {
		return c.Send(message)
	}
	return c.Send(message, telebot.ModeHTML)
}

// doGhostProtocol is the testable core of HandleGhostProtocol, following
// the same pattern as admin.go's doSetTaxRate: no telebot.Context
// dependency, so it can be exercised directly against a real database in
// tests. Returns the message to send and, on failure, a non-nil error
// (in which case the message is a plain-text failure notice, not HTML).
func (h *JobsHandler) doGhostProtocol(ctx context.Context, campID string) (string, error) {
	var lastGhost sql.NullTime
	_ = h.DB.QueryRowContext(ctx, "SELECT last_ghost_protocol_at FROM encampments WHERE id = $1", campID).Scan(&lastGhost)
	if lastGhost.Valid && time.Since(lastGhost.Time) < ghostProtocolCooldown {
		remaining := ghostProtocolCooldown - time.Since(lastGhost.Time)
		return fmt.Sprintf("⏳ Ghost Protocol is on cooldown for another %.0f days.", remaining.Hours()/24), errors.New("on cooldown")
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return "⚠️ Ghost Protocol failed.", err
	}
	defer tx.Rollback()

	var scrap, metal, crystal, dollars float64
	err = tx.QueryRowContext(ctx, "SELECT scrap, metal, crystal, dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap, &metal, &crystal, &dollars)
	if err != nil {
		return "⚠️ Error reading your resources.", err
	}
	scrapCost, metalCost, crystalCost, dollarsCost := scrap*ghostProtocolCostFraction, metal*ghostProtocolCostFraction, crystal*ghostProtocolCostFraction, dollars*ghostProtocolCostFraction

	newX := rand.Intn(10000)
	newY := rand.Intn(10000)
	biomes := []string{"wasteland", "irradiated_zone", "scrapyard", "ashfields", "frozen_tundra"}
	terrains := []string{"flat", "mountainous", "coastal", "urban_ruins"}
	biome := biomes[rand.Intn(len(biomes))]
	terrain := terrains[rand.Intn(len(terrains))]

	var newCoordID string
	err = tx.QueryRowContext(ctx, `
		INSERT INTO coordinates (x, y, biome, danger_level, region, terrain)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (x, y) DO UPDATE SET x = EXCLUDED.x
		RETURNING id`, newX, newY, biome, rand.Intn(5)+1, "Unknown Sector", terrain).Scan(&newCoordID)
	if err != nil {
		return "⚠️ Error finding new coordinates.", err
	}

	if _, err := tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1, metal = metal - $2, crystal = crystal - $3, dollars = dollars - $4 WHERE encampment_id = $5",
		scrapCost, metalCost, crystalCost, dollarsCost, campID); err != nil {
		return "⚠️ Error deducting Ghost Protocol's cost.", err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE encampments SET coordinate_id = $1, last_ghost_protocol_at = CURRENT_TIMESTAMP WHERE id = $2", newCoordID, campID); err != nil {
		return "⚠️ Error relocating.", err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM known_locations WHERE target_encampment_id = $1", campID); err != nil {
		return "⚠️ Error clearing intel locks on your position.", err
	}

	if err := tx.Commit(); err != nil {
		return "⚠️ Error completing Ghost Protocol.", err
	}

	return fmt.Sprintf("👻 %s Every scout and raider who'd locked your position has lost that lock. Your outpost now stands at %s in a %s biome, at a cost of %s Scrap, %s Metal, %s Crystal, and %s Dollars.",
		htmlBold("GHOST PROTOCOL EXECUTED"),
		htmlCode(fmt.Sprintf("[%d, %d]", newX, newY)), biome,
		htmlCode(fmt.Sprintf("%.0f", scrapCost)), htmlCode(fmt.Sprintf("%.0f", metalCost)),
		htmlCode(fmt.Sprintf("%.0f", crystalCost)), htmlCode(fmt.Sprintf("%.0f", dollarsCost))), nil
}



// HandleOrbitalManeuver (/newjoborbitalmaneuver) grants a temporary
// defense rating boost.
func (h *JobsHandler) HandleOrbitalManeuver(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	const cost = 400.0 // Electricity
	const buffDuration = 2 * time.Hour

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ Maneuver failed.")
	}
	defer tx.Rollback()

	var electricity float64
	_ = tx.QueryRowContext(ctx, "SELECT electricity FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&electricity)
	if electricity < cost {
		return c.Send(fmt.Sprintf("❌ %s Need %s.", htmlBold("Insufficient Electricity!"), htmlCode(fmt.Sprintf("%.0f", cost))), telebot.ModeHTML)
	}

	buffUntil := time.Now().UTC().Add(buffDuration)
	_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = electricity - $1 WHERE encampment_id = $2", cost, campID)
	_, _ = tx.ExecContext(ctx, "UPDATE encampments SET orbital_buff_until = $1 WHERE id = $2", buffUntil, campID)

	if err := tx.Commit(); err != nil {
		return c.Send("⚠️ Error activating maneuver.")
	}

	return c.Send(fmt.Sprintf("🛰️✅ %s +30%% defense rating for the next %s.", htmlBold("ORBITAL MANEUVER ACTIVE!"), htmlCode(fmt.Sprintf("%.0f minutes", buffDuration.Minutes()))), telebot.ModeHTML)
}

// HandleRepairUnits (/newjobrepairunits) - field repairs bring back a
// small batch of Soldiers for a Scrap cost.
func (h *JobsHandler) HandleRepairUnits(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	const cost = 200.0 // Scrap, repairs 5 Soldiers
	const repaired = 5

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ Repair failed.")
	}
	defer tx.Rollback()

	var scrap float64
	_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap)
	if scrap < cost {
		return c.Send(fmt.Sprintf("❌ %s Need %s.", htmlBold("Insufficient Scrap!"), htmlCode(fmt.Sprintf("%.0f", cost))), telebot.ModeHTML)
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", cost, campID)
	_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1 WHERE encampment_id = $2", repaired, campID)

	if err := tx.Commit(); err != nil {
		return c.Send("⚠️ Error repairing units.")
	}

	return c.Send(fmt.Sprintf("🔧✅ %s +%d Soldiers restored to fighting condition.", htmlBold("FIELD REPAIRS COMPLETE!"), repaired), telebot.ModeHTML)
}

// HandleRepairBuildings (/newjobrepairbuildings) speeds up any in-progress
// building upgrade.
func (h *JobsHandler) HandleRepairBuildings(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	const cost = 150.0 // Scrap

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ Repair failed.")
	}
	defer tx.Rollback()

	var moduleID string
	var readyAt time.Time
	err = tx.QueryRowContext(ctx, "SELECT id, upgrade_ready_at FROM modules WHERE encampment_id = $1 AND is_upgrading = TRUE ORDER BY upgrade_ready_at ASC LIMIT 1 FOR UPDATE", campID).Scan(&moduleID, &readyAt)
	if err != nil {
		return c.Send("❌ No buildings currently under construction to repair/rush.")
	}

	var scrap float64
	_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap)
	if scrap < cost {
		return c.Send(fmt.Sprintf("❌ %s Need %s.", htmlBold("Insufficient Scrap!"), htmlCode(fmt.Sprintf("%.0f", cost))), telebot.ModeHTML)
	}

	remaining := time.Until(readyAt)
	newReady := readyAt.Add(-remaining / 2)

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", cost, campID)
	_, _ = tx.ExecContext(ctx, "UPDATE modules SET upgrade_ready_at = $1 WHERE id = $2", newReady, moduleID)

	if err := tx.Commit(); err != nil {
		return c.Send("⚠️ Error rushing construction.")
	}

	return c.Send("🏗️✅ "+htmlBold("CONSTRUCTION CREW DEPLOYED!")+" Remaining build time on your active upgrade cut in half.", telebot.ModeHTML)
}

// HandleGatherSunlight (/newjobgathersunlight) - instant manual burst of
// Electricity, on a short cooldown.
func (h *JobsHandler) HandleGatherSunlight(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	var lastSunlight sql.NullTime
	_ = h.DB.QueryRowContext(ctx, "SELECT last_sunlight_at FROM encampments WHERE id = $1", campID).Scan(&lastSunlight)
	if lastSunlight.Valid && time.Since(lastSunlight.Time) < 30*time.Minute {
		remaining := 30*time.Minute - time.Since(lastSunlight.Time)
		return c.Send(fmt.Sprintf("⏳ Solar panels still recharging - %.0f minutes left.", remaining.Minutes()))
	}

	const gain = 150.0
	var curElectricity float64
	_ = h.DB.QueryRowContext(ctx, "SELECT electricity FROM resources WHERE encampment_id = $1", campID).Scan(&curElectricity)
	storageCap := storagecap.CapFor(ctx, h.DB, campID)
	newElectricity, _ := storagecap.Clamp(curElectricity, gain, storageCap)
	_, _ = h.DB.ExecContext(ctx, "UPDATE resources SET electricity = $1 WHERE encampment_id = $2", newElectricity, campID)
	_, _ = h.DB.ExecContext(ctx, "UPDATE encampments SET last_sunlight_at = CURRENT_TIMESTAMP WHERE id = $1", campID)

	return c.Send(fmt.Sprintf("☀️✅ %s +%s Electricity harvested manually.", htmlBold("SUNLIGHT GATHERED!"), htmlCode(fmt.Sprintf("%.0f", gain))), telebot.ModeHTML)
}

// ── Scan command aliases for full command-name parity ──────────────────

func (h *JobsHandler) HandleManualScanAlias(c telebot.Context) error {
	return c.Send("🔍 Manual Scan: use /scout [username] to instantly look up a rival's basic intel.")
}

func (h *JobsHandler) HandleAutoScanAlias(c telebot.Context) error {
	return c.Send("📡 Automatic Scan: use /autoscan to toggle periodic automated recon reports.")
}

func (h *JobsHandler) HandleAdvancedScanAlias(c telebot.Context) error {
	return c.Send("🛰️ Advanced Scan: after a /scout lookup, tap 'Intercept Signal' to launch a full satellite recon mission with real travel time and deeper intel.")
}

func (h *JobsHandler) HandlePublishTradeAlias(c telebot.Context) error {
	return c.Send("💱 Publish Trade: open the Market Exchange from /econ ➜ Market Exchange to list Metal or Crystal for sale to other survivors.")
}
