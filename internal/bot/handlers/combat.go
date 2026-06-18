package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type CombatHandler struct {
	DB       *sql.DB
	AdminIDs []int64
}

func NewCombatHandler(db *sql.DB, adminIDStrs string) *CombatHandler {
	var ids []int64
	for _, s := range strings.Split(adminIDStrs, ",") {
		trimmed := strings.TrimSpace(s)
		if val, err := strconv.ParseInt(trimmed, 10, 64); err == nil {
			ids = append(ids, val)
		}
	}
	return &CombatHandler{
		DB:       db,
		AdminIDs: ids,
	}
}

// IsAdmin checks if the sender is an authorized developer
func (h *CombatHandler) IsAdmin(senderID int64) bool {
	for _, id := range h.AdminIDs {
		if id == senderID {
			return true
		}
	}
	return false
}

// HandleTargetMatrix maps the scan targets button to the raid board dashboard
func (h *CombatHandler) HandleTargetMatrix(c telebot.Context) error {
	return h.HandleRaidBoard(c)
}

// HandleRaidBoard displays player targets and offline AI training skirmishes
func (h *CombatHandler) HandleRaidBoard(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid context sender")
	}

	ctx := context.Background()

	var myCampID string
	var myCampName string
	var myX, myY int
	var myRegion string

	queryMe := `
		SELECT e.id, e.name, c.x, c.y, c.region 
		FROM encampments e
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE e.user_id = $1`
	
	err := h.DB.QueryRowContext(ctx, queryMe, sender.ID).Scan(&myCampID, &myCampName, &myX, &myY, &myRegion)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.MainNavigation())
	}

	queryTargets := `
		SELECT e.id, e.name, u.first_name, c.x, c.y, c.region,
		       COALESCE((SELECT r.scrap FROM resources r WHERE r.encampment_id = e.id), 0) as scrap
		FROM encampments e
		JOIN users u ON u.telegram_id = e.user_id
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE e.id != $1
		LIMIT 5`

	rows, err := h.DB.QueryContext(ctx, queryTargets, myCampID)
	if err != nil {
		log.Printf("Failed scanning target outposts: %v", err)
		return c.Send("⚠️ Failed to load target database matrix.", keyboards.CombatNavigation())
	}
	defer rows.Close()

	type target struct {
		id       string
		name     string
		owner    string
		x, y     int
		region   string
		lootable float64
	}

	var targets []target
	for rows.Next() {
		var t target
		if err := rows.Scan(&t.id, &t.name, &t.owner, &t.x, &t.y, &t.region, &t.lootable); err == nil {
			targets = append(targets, t)
		}
	}

	dashboard := "━━━━━━━━━━━━━━━━━━━━━━\n" +
		"⚔️ TACTICAL TARGET MATRIX\n" +
		"━━━━━━━━━━━━━━━━━━━━━━\n" +
		"Continental travel requires transport ships or jets.\n\n"

	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	if len(targets) > 0 {
		for i, t := range targets {
			steps := math.Abs(float64(t.x-myX)) + math.Abs(float64(t.y-myY))
			if steps == 0 {
				steps = 1
			}

			marchingMinutes := steps * 10.0
			if t.region != myRegion {
				marchingMinutes = 720.0
			}

			marchTimeStr := fmt.Sprintf("%.0fm", marchingMinutes)
			if marchingMinutes >= 60.0 {
				marchTimeStr = fmt.Sprintf("%.1fh", marchingMinutes/60.0)
			}

			dashboard += fmt.Sprintf("[%d] Outpost: %s (%s Territory)\n    Commander: %s\n    Travel Steps: %.0f | March Time: %s\n    Estimated Loot: %.1f Scrap\n\n", i+1, t.name, t.region, t.owner, steps, marchTimeStr, t.lootable)
			btnRaid := selector.Data(fmt.Sprintf("⚔️ Raid [%d]", i+1), "launch_raid", t.id)
			btnSpy := selector.Data(fmt.Sprintf("🛰️ Spy [%d]", i+1), "spy_action", t.id)
			buttons = append(buttons, selector.Row(btnRaid, btnSpy))
		}
	}

	dashboard += "🤖 AI TRAINING SKIRMISH TARGETS:\n" +
		"[AI] Rogue Drone Nest (Sector 1,1)\n" +
		"    Loot Yield: +50 Scrap | March Time: 15s\n\n"

	btnAI := selector.Data("🤖 Skirmish Rogue Drones", "launch_raid", "ai_drone_nest")
	buttons = append(buttons, selector.Row(btnAI))

	dashboard += "━━━━━━━━━━━━━━━━━━━━━━"

	selector.Inline(buttons...)
	return c.Send(dashboard, selector)
}

// HandleExpeditionRadar scans and displays active outbound and incoming tactical operations
func (h *CombatHandler) HandleExpeditionRadar(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	ctx := context.Background()

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	// Fetch outbound active marching raids
	queryOutbound := `
		SELECT r.id, ed.name, r.resolve_time 
		FROM raids r
		JOIN encampments ed ON ed.id = r.defender_id
		WHERE r.attacker_id = $1 AND r.state = 'marching'
		LIMIT 2`
	
	rowsOut, err := h.DB.QueryContext(ctx, queryOutbound, campID)
	outboundText := ""
	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	if err == nil {
		defer rowsOut.Close()
		index := 1
		for rowsOut.Next() {
			var rID, dName string
			var resTime time.Time
			if err := rowsOut.Scan(&rID, &dName, &resTime); err == nil {
				diff := time.Until(resTime)
				timeLeft := int(diff.Seconds())
				if timeLeft < 0 {
					timeLeft = 0
				}
				outboundText += fmt.Sprintf("🚀 OUTBOUND EXPEDITION [%d]:\n   Target Outpost: %s\n   Arrival: %s (%ds remaining)\n\n", index, dName, resTime.UTC().Format("15:04:05"), timeLeft)
				btnSpeed := selector.Data(fmt.Sprintf("⚡ Speedup [%d]", index), "exp_action", "speed", rID)
				btnAbort := selector.Data(fmt.Sprintf("↩️ Abort [%d]", index), "exp_action", "abort", rID)
				buttons = append(buttons, selector.Row(btnSpeed, btnAbort))
				index++
			}
		}
		rowsOut.Close()
	}

	if outboundText == "" {
		outboundText = "🛰️ OUTBOUND: Radar clean. No active offensive marching forces detected.\n\n"
	}

	// Fetch inbound hostile marching forces
	queryInbound := `
		SELECT ea.name, r.resolve_time 
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		WHERE r.defender_id = $1 AND r.state = 'marching'
		LIMIT 1`
	
	var attackerName string
	var resolveTime time.Time
	err = h.DB.QueryRowContext(ctx, queryInbound, campID).Scan(&attackerName, &resolveTime)
	inboundText := ""
	if errors.Is(err, sql.ErrNoRows) {
		inboundText = "🛡️ INBOUND: Radar clean. No incoming hostile military vectors detected."
	} else if err != nil {
		log.Printf("Inbound radar scan failed: %v", err)
		inboundText = "📡 Static: Scanner interference detected."
	} else {
		diff := time.Until(resolveTime)
		timeLeft := int(diff.Seconds())
		if timeLeft < 0 {
			timeLeft = 0
		}
		inboundText = fmt.Sprintf("🚨 RADAR WARNING: INBOUND INVASION!\n   Hostile Force: Outpost [%s]\n   Detonation Impact: %s (%ds remaining)\n\n⚠️ TIP: Spend resources to upgrade defenses in camp immediately!", attackerName, resolveTime.UTC().Format("15:04:05"), timeLeft)
	}

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🛸 ACTIVE EXPEDITION RADAR HUD\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"%s"+
			"%s\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		outboundText, inboundText,
	)

	selector.Inline(buttons...)
	return c.Send(panelText, selector)
}

// HandleScout performs a username-based target search
func (h *CombatHandler) HandleScout(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)

	targetUsername := c.Message().Payload
	if targetUsername == "" {
		return c.Send("⚠️ Syntax Error: Use `/scout [telegram_username]` (without the @ symbol).")
	}

	ctx := context.Background()

	var tID string
	var tName string
	var tOwner string
	var tX, tY int
	var tScrap float64

	query := `
		SELECT e.id, e.name, u.first_name, c.x, c.y, r.scrap
		FROM encampments e
		JOIN users u ON u.telegram_id = e.user_id
		JOIN coordinates c ON c.id = e.coordinate_id
		JOIN resources r ON r.encampment_id = e.id
		WHERE LOWER(u.username) = LOWER($1)`

	err := h.DB.QueryRowContext(ctx, query, targetUsername).Scan(&tID, &tName, &tOwner, &tX, &tY, &tScrap)
	if errors.Is(err, sql.ErrNoRows) {
		return c.Send("❌ Target Not Found: No active outpost registered to that Telegram username.")
	} else if err != nil {
		log.Printf("Scouting database scan failed: %v", err)
		return c.Send("⚠️ Error scanning target parameters.")
	}

	report := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🛰️ TARGET SCOUT INTEL\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Target Outpost: %s\n"+
			"Commander callsign: %s\n"+
			"Wasteland Location: Sector [%d, %d]\n"+
			"Lootable Vault Reserves: %.1f Scrap\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		tName, tOwner, tX, tY, tScrap,
	)

	selector := &telebot.ReplyMarkup{}
	btnRaid := selector.Data("⚔️ Launch Staged Expedition", "launch_raid", tID)
	btnSpy := selector.Data("🛰️ Intercept Signal", "spy_action", tID)

	selector.Inline(selector.Row(btnRaid, btnSpy))

	return c.Send(report, selector)
}

// HandleSpyCallback sweeps target data and decrypts active timers (64-byte safe)
func (h *CombatHandler) HandleSpyCallback(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)
	ctx := context.Background()
	sender := c.Sender()
	targetCampID := c.Args()[0]

	var myCampID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&myCampID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Decryption failed."})
	}
	defer tx.Rollback()

	// Verify and deduct 30 Energy Cells
	var energy float64
	_ = tx.QueryRowContext(ctx, "SELECT energy FROM resources WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&energy)

	if energy < 30.0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Energy: Satellite scans require 30.0 Energy Cells."})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET energy = energy - 30.0 WHERE encampment_id = $1", myCampID)

	// Fetch target module metrics
	var targetName string
	var targetLvl int
	_ = tx.QueryRowContext(ctx, "SELECT name, level FROM encampments WHERE id = $1", targetCampID).Scan(&targetName, &targetLvl)

	var tentLvl, heapLvl, genLvl int
	_ = tx.QueryRowContext(ctx, "SELECT level FROM modules WHERE encampment_id = $1 AND type = 'tent'", targetCampID).Scan(&tentLvl)
	_ = tx.QueryRowContext(ctx, "SELECT level FROM modules WHERE encampment_id = $1 AND type = 'scrap_heap'", targetCampID).Scan(&heapLvl)
	_ = tx.QueryRowContext(ctx, "SELECT level FROM modules WHERE encampment_id = $1 AND type = 'generator'", targetCampID).Scan(&genLvl)

	var upgradingModule string
	_ = tx.QueryRowContext(ctx, "SELECT type FROM modules WHERE encampment_id = $1 AND is_upgrading = TRUE LIMIT 1", targetCampID).Scan(&upgradingModule)
	if upgradingModule == "" {
		upgradingModule = "None"
	}

	// Fetch current resources
	var scrap, rations float64
	_ = tx.QueryRowContext(ctx, "SELECT scrap, rations FROM resources WHERE encampment_id = $1", targetCampID).Scan(&scrap, &rations)

	_ = tx.Commit()

	_ = c.Respond(&telebot.CallbackResponse{Text: "🛰️ Satellite Sweep Success! Decrypting telemetry..."})

	spyReport := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🛰️ SPY SATELLITE DECRYPTOR INDICES\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Target Outpost: %s\n\n"+
			"DECRYPTED RESOURCES:\n"+
			"⚙️ Scrap: %.1f\n"+
			"🥫 Rations: %.1f\n\n"+
			"MODULE STATUS GRID:\n"+
			"⛺ Tent: Level %d\n"+
			"⚙️ Scrap Heap: Level %d\n"+
			"⚡ Generator: Level %d\n\n"+
			"🔧 Active Upgrades Queue: %s\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		targetName, scrap, rations, tentLvl, heapLvl, genLvl, upgradingModule,
	)

	return c.Send(spyReport, keyboards.CombatNavigation())
}

// HandleLaunchRaidCallback registers a marching raid inside the database with dynamic regional routing and direct push alert fail-safes
func (h *CombatHandler) HandleLaunchRaidCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()

	defenderCampID := c.Args()[0]

	// Resolve Attacker Camp ID dynamically to stay 64-byte safe
	var attackerCampID string
	var myRegion string
	var myX, myY int
	err := h.DB.QueryRowContext(ctx, "SELECT id, region, x, y FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.user_id = $1", sender.ID).Scan(&attackerCampID, &myRegion, &myX, &myY)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error resolving Outpost."})
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Database transaction error."})
	}
	defer tx.Rollback()

	// 1. Fetch active global weather front to calculate movement multipliers
	var activeWeather string
	_ = tx.QueryRowContext(ctx, "SELECT active_weather FROM world_state WHERE id = 1").Scan(&activeWeather)

	// Unified military check: Read forces directly from workshop_inventory
	var soldiers, drones, jets, mechs, nukes, tanks int
	queryForces := `
		SELECT COALESCE(soldiers, 0), COALESCE(drones, 0), COALESCE(jets, 0), COALESCE(mechs, 0), COALESCE(nukes, 0), COALESCE(fusion_tanks, 0)
		FROM workshop_inventory 
		WHERE encampment_id = $1`
	
	err = tx.QueryRowContext(ctx, queryForces, attackerCampID).Scan(&soldiers, &drones, &jets, &mechs, &nukes, &tanks)
	if err != nil {
		log.Printf("Failed querying barracks stocks: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error reading military arrays."})
	}

	troopCount := soldiers + drones + jets + mechs + nukes + tanks
	if troopCount <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Action Forbidden: You must have at least 1 Soldier, Drone, Jet, or Mech in your barracks to launch a raid."})
	}

	var attackerName string
	_ = tx.QueryRowContext(ctx, "SELECT name FROM encampments WHERE id = $1", attackerCampID).Scan(&attackerName)

	var ships int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(fusion_tanks, 0), COALESCE(mechs, 0), COALESCE(ships, 0), COALESCE(jets, 0) FROM workshop_inventory WHERE encampment_id = $1", attackerCampID).Scan(&tanks, &mechs, &ships, &jets)

	// If AI Target selection
	if defenderCampID == "ai_drone_nest" {
		resolveTime := time.Now().Add(15 * time.Second)
		insertRaid := `
			INSERT INTO raids (attacker_id, defender_id, state, resolve_time) 
			VALUES ($1, '00000000-0000-0000-0000-000000000000', 'marching', $2)
			RETURNING id`
		var raidID string
		err = tx.QueryRowContext(ctx, insertRaid, attackerCampID, resolveTime).Scan(&raidID)
		if err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to register skirmish."})
		}
		_ = tx.Commit()
		_ = c.Respond(&telebot.CallbackResponse{Text: "🤖 Skirmish launched! Marching on Drone Nest..."})
		return h.renderExpeditionPanel(c, raidID, attackerName, resolveTime)
	}

	var defenderName string
	var defenderUserID int64
	var defX, defY int
	var defRegion string
	_ = tx.QueryRowContext(ctx, "SELECT e.name, e.user_id, c.x, c.y, c.region FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", defenderCampID).Scan(&defenderName, &defenderUserID, &defX, &defY, &defRegion)

	// Calculate True Grid Distance
	steps := math.Abs(float64(defX-myX)) + math.Abs(float64(defY-myY))
	if steps == 0 {
		steps = 1
	}

	// Dynamic Travel Marching Calculations (Base 10m per step + heavy machinery weight)
	marchingMinutes := (steps * 10.0) + (float64(tanks) * 3.0) + (float64(mechs) * 5.0)

	// Inter-continental logistics block
	if defRegion != myRegion {
		if jets > 0 {
			// Cargo Jet reduces inter-continental travel to flat 2 hours (120 mins)
			marchingMinutes = 120.0
		} else if ships > 0 {
			// Clipper ship allows ocean travel: takes 12 hours (720 mins)
			marchingMinutes = 720.0
		} else {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Ocean Block: Target is on a different continent. Build a Clipper Ship or Cargo Jet in the Workshop to cross!"})
		}
	}

	// Apply Weather travel multipliers
	switch activeWeather {
	case "radiation_storm":
		marchingMinutes *= 1.5
	case "solar_flare":
		marchingMinutes *= 0.7
	}

	marchDuration := time.Duration(marchingMinutes) * time.Minute

	resolveTime := time.Now().Add(marchDuration)

	var raidID string
	insertRaid := `
		INSERT INTO raids (attacker_id, defender_id, state, resolve_time) 
		VALUES ($1, $2, 'marching', $3)
		RETURNING id`
	err = tx.QueryRowContext(ctx, insertRaid, attackerCampID, defenderCampID, resolveTime).Scan(&raidID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to register raid."})
	}

	// 5. Send Direct-Push Alert to defender instantly! (Guarantees instant warning alerts)
	defenderAlert := fmt.Sprintf(
		"🚨 RADAR ALERT: HOSTILE RAID INBOUND!\n\n"+
			"Our sensors have detected a hostile staged raid marching from Outpost [%s] in %s.\n"+
			"Estimated Arrival Time: %s.\n\n"+
			"Upgrade your Tent or fortify your facilities immediately!",
		attackerName, myRegion, resolveTime.UTC().Format("15:04:05"),
	)
	
	// Direct Bot.Send Push warning
	targetUser := &telebot.User{ID: defenderUserID}
	_, err = c.Bot().Send(targetUser, defenderAlert)
	if err != nil {
		log.Printf("Failsafe Direct Push failed to deliver to %d: %v", defenderUserID, err)
	}

	// Insert into DB queue for backup tracing
	_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, TRUE)", defenderUserID, defenderAlert)

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: "🚀 Raiders deployed! Marching towards target..."})

	return h.renderExpeditionPanel(c, raidID, attackerName, resolveTime)
}

func (h *CombatHandler) renderExpeditionPanel(c telebot.Context, raidID string, attackerName string, resolveTime time.Time) error {
	diff := time.Until(resolveTime)
	timeLeft := int(diff.Seconds())
	if timeLeft < 0 {
		timeLeft = 0
	}

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🚀 ACTIVE MILITARY EXPEDITION HUD\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Outpost Force: %s\n"+
			"Estimated Arrival: %s (%ds remaining)\n\n"+
			"TACTICAL COMMAND CONTROLS:\n"+
			"⚡ [Speed Up] - Costs 100 Scrap. Advances arrival by 30s.\n"+
			"↩️ [Abort] - Recalls troops instantly back to base.\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		attackerName, resolveTime.UTC().Format("15:04:05"), timeLeft,
	)

	selector := &telebot.ReplyMarkup{}
	btnSpeed := selector.Data("⚡ Speed Up (100 Scrap)", "exp_action", "speed", raidID)
	btnAbort := selector.Data("↩️ Abort Expedition", "exp_action", "abort", raidID)

	selector.Inline(
		selector.Row(btnSpeed),
		selector.Row(btnAbort),
	)

	return c.Send(panelText, selector)
}

// HandleExpeditionActions processes inline tactical movements
func (h *CombatHandler) HandleExpeditionActions(c telebot.Context) error {
	ctx := context.Background()
	action := c.Args()[0]
	raidID := c.Args()[1]

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transaction failed."})
	}
	defer tx.Rollback()

	var state string
	var attackerID string
	var resolveTime time.Time
	err = tx.QueryRowContext(ctx, "SELECT state, attacker_id, resolve_time FROM raids WHERE id = $1 FOR UPDATE", raidID).Scan(&state, &attackerID, &resolveTime)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Expired: This expedition has already concluded."})
	}

	if state != "marching" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Already concluded."})
	}

	switch action {
	case "speed":
		var scrap float64
		_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", attackerID).Scan(&scrap)
		if scrap < 100.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Scrap. Speed Up costs 100."})
		}

		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - 100.0 WHERE encampment_id = $1", attackerID)
		newResolve := resolveTime.Add(-30 * time.Second)
		_, _ = tx.ExecContext(ctx, "UPDATE raids SET resolve_time = $1 WHERE id = $2", newResolve, raidID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "⚡ Speed boosted! Arrival time advanced."})
		resolveTime = newResolve

	case "abort":
		_, _ = tx.ExecContext(ctx, "DELETE FROM raids WHERE id = $1", raidID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "↩️ Mission aborted!"})
		_ = tx.Commit()
		return c.Send("↩️ Expedition aborted. Forces returned safely to barracks.", keyboards.MainNavigation())
	}

	_ = tx.Commit()

	var attackerName string
	_ = h.DB.QueryRowContext(ctx, "SELECT name FROM encampments WHERE id = $1", attackerID).Scan(&attackerName)

	return h.renderExpeditionPanel(c, raidID, attackerName, resolveTime)
}