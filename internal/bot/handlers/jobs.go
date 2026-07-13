package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"time"

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
		return c.Send(fmt.Sprintf("❌ Insufficient Electricity! Need %.0f, you have %.0f.", cost, electricity))
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

	return c.Send(fmt.Sprintf("🚀⚡ HYPERSPEED ENGAGED! Your nearest mission's remaining time was cut in half. New ETA: %s", newResolve.UTC().Format("15:04 MST")))
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
		return c.Send(fmt.Sprintf("❌ Insufficient Materials! Need %.0f Metal, %.0f Crystal for extension level %d.", metalCost, crystalCost, extensionLvl+1))
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - $1, crystal = crystal - $2 WHERE encampment_id = $3", metalCost, crystalCost, campID)
	_, _ = tx.ExecContext(ctx, "UPDATE encampments SET extension_lvl = extension_lvl + 1 WHERE id = $1", campID)

	if err := tx.Commit(); err != nil {
		return c.Send("⚠️ Error extending planet.")
	}

	return c.Send(fmt.Sprintf("🌍✅ PLANET EXTENDED! Storage capacity +1000 permanently (extension level %d). Next extension: %.0f Metal, %.0f Crystal.", extensionLvl+1, metalCost*2, crystalCost*2))
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
		return c.Send(fmt.Sprintf("❌ Insufficient Electricity! Need %.0f.", cost))
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

	return c.Send(fmt.Sprintf("🌀✨ TELEPORT COMPLETE! Your outpost now stands at [%d, %d] in a %s biome.", newX, newY, biome))
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
		return c.Send(fmt.Sprintf("❌ Insufficient Electricity! Need %.0f.", cost))
	}

	buffUntil := time.Now().UTC().Add(buffDuration)
	_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = electricity - $1 WHERE encampment_id = $2", cost, campID)
	_, _ = tx.ExecContext(ctx, "UPDATE encampments SET orbital_buff_until = $1 WHERE id = $2", buffUntil, campID)

	if err := tx.Commit(); err != nil {
		return c.Send("⚠️ Error activating maneuver.")
	}

	return c.Send(fmt.Sprintf("🛰️✅ ORBITAL MANEUVER ACTIVE! +30%% defense rating for the next %.0f minutes.", buffDuration.Minutes()))
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
		return c.Send(fmt.Sprintf("❌ Insufficient Scrap! Need %.0f.", cost))
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", cost, campID)
	_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1 WHERE encampment_id = $2", repaired, campID)

	if err := tx.Commit(); err != nil {
		return c.Send("⚠️ Error repairing units.")
	}

	return c.Send(fmt.Sprintf("🔧✅ FIELD REPAIRS COMPLETE! +%d Soldiers restored to fighting condition.", repaired))
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
		return c.Send(fmt.Sprintf("❌ Insufficient Scrap! Need %.0f.", cost))
	}

	remaining := time.Until(readyAt)
	newReady := readyAt.Add(-remaining / 2)

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", cost, campID)
	_, _ = tx.ExecContext(ctx, "UPDATE modules SET upgrade_ready_at = $1 WHERE id = $2", newReady, moduleID)

	if err := tx.Commit(); err != nil {
		return c.Send("⚠️ Error rushing construction.")
	}

	return c.Send("🏗️✅ CONSTRUCTION CREW DEPLOYED! Remaining build time on your active upgrade cut in half.")
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
	_, _ = h.DB.ExecContext(ctx, "UPDATE resources SET electricity = electricity + $1 WHERE encampment_id = $2", gain, campID)
	_, _ = h.DB.ExecContext(ctx, "UPDATE encampments SET last_sunlight_at = CURRENT_TIMESTAMP WHERE id = $1", campID)

	return c.Send(fmt.Sprintf("☀️✅ SUNLIGHT GATHERED! +%.0f Electricity harvested manually.", gain))
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
