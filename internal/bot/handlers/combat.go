package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"math/rand"
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

func (h *CombatHandler) IsAdmin(senderID int64) bool {
	for _, id := range h.AdminIDs {
		if id == senderID {
			return true
		}
	}
	return false
}

func (h *CombatHandler) HandleTargetMatrix(c telebot.Context) error {
	return h.HandleRaidBoard(c)
}

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

	_ = c.Send("⚔️ Syncing tactical coordinate systems...", keyboards.CombatNavigation())

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
	rows.Close()

	dashboard := "━━━━━━━━━━━━━━━━━━━━━━\n" +
		"⚔️ TACTICAL TARGET MATRIX\n" +
		"━━━━━━━━━━━━━━━━━━━━━━\n" +
		"Select an action button to initiate an offensive sweep. Co-Op calls allow teammates to coordinate power.\n\n"

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
			btnCoop := selector.Data(fmt.Sprintf("🤝 Co-Op [%d]", i+1), "stage_coop", t.id)
			buttons = append(buttons, selector.Row(btnRaid, btnSpy), selector.Row(btnCoop))
		}
	}

	queryCoops := `
		SELECT r.id, ea.name, ed.name, r.resolve_time 
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		JOIN encampments ed ON ed.id = r.defender_id
		WHERE r.state = 'staged' AND r.attacker_id != $1`

	rowsCoop, err := h.DB.QueryContext(ctx, queryCoops, myCampID)
	if err == nil {
		defer rowsCoop.Close()
		hasCoops := false
		for rowsCoop.Next() {
			var rID, aName, dName string
			var resTime time.Time
			if err := rowsCoop.Scan(&rID, &aName, &dName, &resTime); err == nil {
				if !hasCoops {
					dashboard += "🤝 ACTIVE CO-OP RECRUITMENT LOBBIES:\n"
					hasCoops = true
				}
				timeLeft := int(resTime.UTC().Sub(time.Now().UTC()).Seconds())
				if timeLeft < 0 {
					timeLeft = 0
				}
				dashboard += fmt.Sprintf("• %s is recruiting to raid Outpost %s!\n  Departure window expires in: %ds\n\n", aName, dName, timeLeft)
				btnJoin := selector.Data(fmt.Sprintf("🤝 Join %s", aName), "join_coop", rID)
				buttons = append(buttons, selector.Row(btnJoin))
			}
		}
	}

	dashboard += "🤖 AI TRAINING SKIRMISH TARGETS:\n" +
		"[AI] Rogue Drone Nest (Sector 1,1)\n" +
		"    Loot Yield: +50 Scrap | Journey Time: Dynamic\n\n"

	btnAI := selector.Data("🤖 Skirmish Rogue Drones", "launch_raid", "ai_drone_nest")
	buttons = append(buttons, selector.Row(btnAI))

	dashboard += "━━━━━━━━━━━━━━━━━━━━━━"

	selector.Inline(buttons...)
	return c.Send(dashboard, selector)
}

func (h *CombatHandler) HandleExpeditionRadar(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	ctx := context.Background()

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	queryOutbound := `
		SELECT r.id, COALESCE(ed.name, 'Rogue Drone Nest'), r.resolve_time, r.state, r.round_number, r.attacker_rations, r.attacker_ammo
		FROM raids r
		LEFT JOIN encampments ed ON ed.id = r.defender_id
		WHERE r.attacker_id = $1 AND (r.state = 'marching' OR r.state = 'engaged' OR r.state = 'staged' OR r.state = 'returning')
		LIMIT 2`

	rowsOut, err := h.DB.QueryContext(ctx, queryOutbound, campID)
	outboundText := ""
	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	if err == nil {
		defer rowsOut.Close()
		index := 1
		for rowsOut.Next() {
			var rID, dName, rState string
			var rRound int
			var rRations, rAmmo float64
			var resTime time.Time
			if err := rowsOut.Scan(&rID, &dName, &resTime, &rState, &rRound, &rRations, &rAmmo); err == nil {
				diff := resTime.UTC().Sub(time.Now().UTC())
				timeLeft := int(diff.Seconds())
				if timeLeft < 0 {
					timeLeft = 0
				}
				switch rState {
				case "marching":
					outboundText += fmt.Sprintf("🚀 OUTBOUND EXPEDITION [%d] (MARCHING):\n   Target: %s\n   Arrival: %s (%ds remaining)\n\n", index, dName, resTime.UTC().Format("15:04:05"), timeLeft)
				case "staged":
					outboundText += fmt.Sprintf("🤝 STAGED CO-OP RAID [%d] (PREPARING):\n   Target: %s\n   Departure Window: %s (%ds remaining)\n\n", index, dName, resTime.UTC().Format("15:04:05"), timeLeft)
				case "returning":
					outboundText += fmt.Sprintf("↩️ RETURN MARCH [%d] (RETURNING):\n   Target: %s\n   Base Arrival: %s (%ds remaining)\n\n", index, dName, resTime.UTC().Format("15:04:05"), timeLeft)
				default:
					outboundText += fmt.Sprintf("⚔️ ACTIVE ENGAGEMENT [%d] (COMBAT - Round %d):\n   Target: %s\n   Decisive Resolution: %s (%ds remaining)\n   Supplies: Rations %.0f%% | Ammunition: %.0f%%\n\n", index, rRound, dName, resTime.UTC().Format("15:04:05"), timeLeft, rRations, rAmmo)
				}
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

	queryInbound := `
		SELECT ea.name, r.resolve_time, r.state 
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		WHERE r.defender_id = $1 AND (r.state = 'marching' OR r.state = 'engaged')
		LIMIT 1`

	var attackerName, rState string
	var resolveTime time.Time
	err = h.DB.QueryRowContext(ctx, queryInbound, campID).Scan(&attackerName, &resolveTime, &rState)
	inboundText := ""
	if errors.Is(err, sql.ErrNoRows) {
		inboundText = "🛡️ INBOUND: Radar clean. No incoming hostile military vectors detected.\n\n"
	} else if err != nil {
		log.Printf("Inbound radar scan failed: %v", err)
		inboundText = "📡 Static: Scanner interference detected.\n\n"
	} else {
		diff := resolveTime.UTC().Sub(time.Now().UTC())
		timeLeft := int(diff.Seconds())
		if timeLeft < 0 {
			timeLeft = 0
		}
		if rState == "marching" {
			inboundText = fmt.Sprintf("🚨 INBOUND INVASION WARNING!\n   Hostile Force: Outpost [%s]\n   March Arrival: %s (%ds remaining)\n\n", attackerName, resolveTime.UTC().Format("15:04:05"), timeLeft)
		} else {
			inboundText = fmt.Sprintf("💥 BASE SIEGE UNDERWAY!\n   Invading Force: Outpost [%s]\n   Silo Impact Window: %s (%ds remaining)\n\n⚠️ TIP: Repair defensive modules immediately!\n\n", attackerName, resolveTime.UTC().Format("15:04:05"), timeLeft)
		}
	}

	querySpies := `
		SELECT s.id, ea.name, ed.name, s.created_at, (s.spy_id = $1) as is_outbound, s.resolved
		FROM spy_missions s
		JOIN encampments ea ON ea.id = s.spy_id
		JOIN encampments ed ON ed.id = s.target_id
		WHERE s.is_intercepted = FALSE AND (s.spy_id = $1 OR s.target_id = $1)`

	rowsSpies, err := h.DB.QueryContext(ctx, querySpies, campID)
	spyText := ""
	if err == nil {
		defer rowsSpies.Close()
		for rowsSpies.Next() {
			var spyID, eaName, edName string
			var createdAt time.Time
			var isOutbound, resolved bool
			if err := rowsSpies.Scan(&spyID, &eaName, &edName, &createdAt, &isOutbound, &resolved); err == nil {
				timeLeft := 120 - int(time.Since(createdAt.UTC()).Seconds())
				if timeLeft < 0 {
					timeLeft = 0
				}
				if isOutbound {
					if !resolved {
						spyText += fmt.Sprintf("🛰️ ACTIVE OUTBOUND SCAN: Scanning %s\n   Uplink Status: Decrypting (%ds remaining)\n\n", edName, timeLeft)
					} else {
						spyText += fmt.Sprintf("↩️ SATELLITE RETURNING: Carrying Intel from %s\n   Arrival Status: Downlinking (%ds remaining)\n\n", edName, timeLeft)
					}
				} else {
					if !resolved {
						spyText += fmt.Sprintf("📡 INCOMING ESPIONAGE BREACH: Rival %s\n   Uplink Status: Intercept Window (%ds remaining)\n\n", eaName, timeLeft)
					} else {
						spyText += fmt.Sprintf("📡 RETREAT INTERCEPT WINDOW: Rival %s\n   Status: Chase Chase Chase! (%ds remaining)\n\n", eaName, timeLeft)
					}
					btnIntercept := selector.Data("🛡️ Launch Interceptor Drone", "launch_interceptor", spyID)
					buttons = append(buttons, selector.Row(btnIntercept))
				}
			}
		}
	}

	if spyText == "" {
		spyText = "🛰️ ESPIONAGE: No active satellite link signals detected on local frequencies."
	}

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🛸 ACTIVE EXPEDITION RADAR HUD\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"%s"+
			"%s"+
			"📡 COGNITIVE SIGNAL SCANNER:\n"+
			"%s\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		outboundText, inboundText, spyText,
	)

	selector.Inline(buttons...)
	return c.Send(panelText, selector)
}

// HandleAutoScanToggle toggles the SpaceHunt-style "Automatic Scan" job:
// when enabled, the tick engine periodically runs a lightweight scan
// against a random rival outpost and reports it directly to the player,
// without them needing to manually run /scout each time.
func (h *CombatHandler) HandleAutoScanToggle(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	ctx := context.Background()

	var campID string
	var currentlyEnabled bool
	err := h.DB.QueryRowContext(ctx, "SELECT id, auto_scan_enabled FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID, &currentlyEnabled)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	newState := !currentlyEnabled
	_, err = h.DB.ExecContext(ctx, "UPDATE encampments SET auto_scan_enabled = $1 WHERE id = $2", newState, campID)
	if err != nil {
		return c.Send("⚠️ Error updating Automatic Scan job.")
	}

	if newState {
		return c.Send("📡✅ AUTOMATIC SCAN ENGAGED: Your Radar will now periodically sweep the Wasteland and report on nearby rivals automatically.")
	}
	return c.Send("📡❌ AUTOMATIC SCAN DISENGAGED: Radar sweeps paused. Run /autoscan again to re-enable.")
}

func (h *CombatHandler) HandleScout(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)

	targetUsername := c.Message().Payload
	if targetUsername == "" {
		return c.Send("⚠️ Syntax Error: Use `/scout [telegram_username]` (without the @ symbol).")
	}

	if len(targetUsername) > 32 {
		return c.Send("❌ Input Blocked: Username must be 32 characters or fewer.")
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

func (h *CombatHandler) HandleSpyCallback(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)
	ctx := context.Background()
	sender := c.Sender()
	targetCampID := c.Args()[0]

	var myCampID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&myCampID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Decryption setup failure."})
	}
	defer tx.Rollback()

	var spyDevices int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(drones, 0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&spyDevices)

	if spyDevices <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Action Blocked: You must assemble a Spy Device in the Heavy Workshop first!"})
	}

	var electricity float64
	_ = tx.QueryRowContext(ctx, "SELECT electricity FROM resources WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&electricity)

	if electricity < 30.0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Electricity: Satellite scans require 30.0 Electricity Cells."})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = electricity - 30.0 WHERE encampment_id = $1", myCampID)
	_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET drones = drones - 1 WHERE encampment_id = $1", myCampID)

	var myX, myY int
	_ = tx.QueryRowContext(ctx, "SELECT c.x, c.y FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", myCampID).Scan(&myX, &myY)

	var defX, defY int
	_ = tx.QueryRowContext(ctx, "SELECT c.x, c.y FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", targetCampID).Scan(&defX, &defY)

	steps := math.Abs(float64(defX-myX)) + math.Abs(float64(defY-myY))
	if steps == 0 {
		steps = 1
	}

	marchingMinutes := steps * 1.5

	var speedTechLvl int = 1
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(speed_tech_lvl, 1) FROM research_states WHERE encampment_id = $1", myCampID).Scan(&speedTechLvl)
	speedBonus := math.Min(float64(speedTechLvl-1)*0.04, 0.60)
	marchingMinutes *= (1.0 - speedBonus)
	if marchingMinutes < 0.5 {
		marchingMinutes = 0.5
	}

	resolveTime := time.Now().UTC().Add(time.Duration(marchingMinutes) * time.Minute)

	var attackerName string
	_ = tx.QueryRowContext(ctx, "SELECT name FROM encampments WHERE id = $1", myCampID).Scan(&attackerName)

	var defenderUserID int64
	_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", targetCampID).Scan(&defenderUserID)

	var spyID string
	queryInsertSpy := `
		INSERT INTO spy_missions (spy_id, target_id, is_intercepted, resolved, resolve_time) 
		VALUES ($1, $2, FALSE, FALSE, $3) 
		RETURNING id`
	err = tx.QueryRowContext(ctx, queryInsertSpy, myCampID, targetCampID, resolveTime).Scan(&spyID)
	if err != nil {
		log.Printf("Failed registering spy mission: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing espionage index."})
	}

	_ = tx.Commit()

	msg, errAnim := c.Bot().Send(c.Recipient(), "📡 ESTABLISHING SECURE COGNITIVE FREQUENCIES...")
	if errAnim == nil {
		time.Sleep(300 * time.Millisecond)
		_, _ = c.Bot().Edit(msg, "🛰️ DEPLOYING ESPIONAGE SATELLITE...\n[▰▱▱▱▱▱▱▱▱▱] 15% - Rockets fired, ascending to low-orbit...")
		time.Sleep(300 * time.Millisecond)
		_, _ = c.Bot().Edit(msg, "🛰️ DEPLOYING ESPIONAGE SATELLITE...\n[▰▰▰▰▰▱▱▱▱▱] 50% - Calibrating coordinate scanner lenses...")
		time.Sleep(300 * time.Millisecond)
		_, _ = c.Bot().Edit(msg, "🛰️ DEPLOYING ESPIONAGE SATELLITE...\n[▰▰▰▰▰▰▰▰▰▰] 100% - Low-orbit aligned! Commencing telemetry decrypts...")
		time.Sleep(300 * time.Millisecond)
		_ = c.Bot().Delete(msg)
	}

	defenderAlert := fmt.Sprintf(
		"🛰️ ESPIONAGE INTRUSION DETECTED!\n\n"+
			"A hostile Spy Satellite launched by Outpost [%s] has breached your wireless perimeter and is transmitting warehouse telemetry!\n\n"+
			"⚠️ Intercept Window: 30 seconds. Spend 10.0 Electricity Cells to vaporize the uplink.",
		attackerName,
	)

	selector := &telebot.ReplyMarkup{}
	btnIntercept := selector.Data("🛡️ Launch Interceptor Drone", "launch_interceptor", spyID)
	selector.Inline(selector.Row(btnIntercept))

	defenderUser := &telebot.User{ID: defenderUserID}
	_, err = c.Bot().Send(defenderUser, defenderAlert, selector)
	if err != nil {
		log.Printf("Failsafe satellite alerts failed: %v", err)
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🛰️ Satellite positioned! Syncing telemetry stream..."})
	return c.Send("🛰️ Downlink established. Decryption completes on next clock ticks...")
}

func (h *CombatHandler) HandleLaunchInterceptor(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	spyID := c.Args()[0]

	var myCampID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&myCampID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Connection error."})
	}
	defer tx.Rollback()

	var interceptors int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(drones, 0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&interceptors)
	if interceptors <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Action Blocked: You must construct an Interceptor Drone inside the workshop first!"})
	}

	var isIntercepted bool
	var resolved bool
	var attackerCampID string
	var createdAt time.Time
	var resolveTime time.Time
	err = tx.QueryRowContext(ctx, "SELECT is_intercepted, resolved, spy_id, created_at, resolve_time FROM spy_missions WHERE id = $1 FOR UPDATE", spyID).Scan(&isIntercepted, &resolved, &attackerCampID, &createdAt, &resolveTime)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Connection Closed: This satellite has already returned to orbit."})
	}

	if isIntercepted {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Already Neutralized."})
	}

	// Enforce the intercept window: the deadline is whatever resolve_time
	// currently represents (outbound arrival before the tick engine flips
	// `resolved`, or the return-to-orbit deadline afterward). Without this
	// check the satellite stays interceptable indefinitely, even long
	// after it should have safely landed.
	if time.Now().UTC().After(resolveTime) {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Too Late: The intercept window has closed. The satellite is out of range."})
	}

	var electricity float64
	_ = tx.QueryRowContext(ctx, "SELECT electricity FROM resources WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&electricity)

	if electricity < 10.0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Electricity: Drones require 10.0 Electricity Cells."})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = electricity - 10.0 WHERE encampment_id = $1", myCampID)
	_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET drones = drones - 1 WHERE encampment_id = $1", myCampID)

	var attackerUserID int64
	_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", attackerCampID).Scan(&attackerUserID)

	var attackerTechLvl, defenderTechLvl int = 1, 1
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(intel_tech_lvl, 1) FROM research_states WHERE encampment_id = $1", attackerCampID).Scan(&attackerTechLvl)
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(intel_tech_lvl, 1) FROM research_states WHERE encampment_id = $1", myCampID).Scan(&defenderTechLvl)

	interceptChance := 0.50 + float64(defenderTechLvl-attackerTechLvl)*0.10

	if resolved {
		interceptChance -= 0.20
	}

	if interceptChance < 0.05 {
		interceptChance = 0.05
	} else if interceptChance > 0.95 {
		interceptChance = 0.95
	}

	rSource := rand.NewSource(time.Now().UnixNano() + sender.ID)
	rGen := rand.New(rSource)

	if rGen.Float64() < interceptChance {
		_, _ = tx.ExecContext(ctx, "UPDATE spy_missions SET is_intercepted = TRUE, resolved = TRUE WHERE id = $1", spyID)
		_ = tx.Commit()

		attackerUser := &telebot.User{ID: attackerUserID}
		_, _ = c.Bot().Send(attackerUser, "💥 SPY INTERCEPTED: Your spy satellite was intercepted and destroyed by defender air defense interceptor systems! Telemetry lost.")
		
		_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("🛡️ SUCCESS: Drone intercepted hostile satellite! (%d%% Success Probability)", int(interceptChance*100))})
		return c.Send(fmt.Sprintf("🛡️ SUCCESS: Your Interceptor Drone destroyed the spy satellite! (%d%% probability hit). Telemetry was vaporized.", int(interceptChance*100)))
	}

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ FAILED: The satellite evaded your defense systems! (%d%% Success Probability)", int(interceptChance*100))})
	return c.Send("❌ CHASE FAILED: The spy satellite evaded your defense systems and continued its route. Interceptor drone was lost.")
}

func (h *CombatHandler) HandleStageCoopCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	defenderCampID := c.Args()[0]

	var attackerCampID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&attackerCampID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Staging failed."})
	}
	defer tx.Rollback()

	resolveTime := time.Now().UTC().Add(5 * time.Minute)

	insertRaid := `
		INSERT INTO raids (attacker_id, defender_id, state, resolve_time) 
		VALUES ($1, $2, 'staged', $3)`
	_, err = tx.ExecContext(ctx, insertRaid, attackerCampID, defenderCampID, resolveTime)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error registering co-op lobby."})
	}

	_ = tx.Commit()
	return c.Respond(&telebot.CallbackResponse{Text: "🤝 Co-Op lobby staged! Aligned alliance players can join your force."})
}

func (h *CombatHandler) HandleJoinCoopCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	raidID := c.Args()[0]

	var helperCampID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&helperCampID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Establish outpost camp first."})
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Joining failed."})
	}
	defer tx.Rollback()

	var state string
	var attackerCampID string
	err = tx.QueryRowContext(ctx, "SELECT state, attacker_id FROM raids WHERE id = $1 FOR UPDATE", raidID).Scan(&state, &attackerCampID)
	if err != nil || state != "staged" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Lobby Expired: This staged raid has already departed."})
	}

	var availSoldiers, availMechs int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers, 0), COALESCE(mechs, 0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", helperCampID).Scan(&availSoldiers, &availMechs)

	conSoldiers := availSoldiers / 2
	conMechs := availMechs / 2

	if conSoldiers <= 0 && conMechs <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Troop Requirement: You must have active forces stationed to join co-op raids!"})
	}

	var creatorX, creatorY, helperX, helperY int
	_ = tx.QueryRowContext(ctx, "SELECT c.x, c.y FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", attackerCampID).Scan(&creatorX, &creatorY)
	_ = tx.QueryRowContext(ctx, "SELECT c.x, c.y FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", helperCampID).Scan(&helperX, &helperY)

	steps := math.Abs(float64(creatorX-helperX)) + math.Abs(float64(creatorY-helperY))
	if steps == 0 {
		steps = 1
	}
	transitMinutes := steps * 10.0
	arrivalTime := time.Now().UTC().Add(time.Duration(transitMinutes) * time.Minute)

	_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers - $1, mechs = mechs - $2 WHERE encampment_id = $3", conSoldiers, conMechs, helperCampID)

	queryJointMember := `
		INSERT INTO raid_coop_members (raid_id, encampment_id, soldiers_contributed, mechs_contributed, state, arrival_time)
		VALUES ($1, $2, $3, $4, 'marching_to_ally', $5)`
	_, err = tx.ExecContext(ctx, queryJointMember, raidID, helperCampID, conSoldiers, conMechs, arrivalTime)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing helper contribution details."})
	}

	var creatorUserID int64
	var targetOutpostName string
	_ = tx.QueryRowContext(ctx, `
		SELECT e.user_id, COALESCE(ed.name, 'Rogue Drone Nest') 
		FROM raids r 
		JOIN encampments e ON e.id = r.attacker_id 
		LEFT JOIN encampments ed ON ed.id = r.defender_id
		WHERE r.id = $1`, raidID).Scan(&creatorUserID, &targetOutpostName)

	alertCreatorMsg := fmt.Sprintf(
		"🤝 CO-OP LOBBY UPDATE: Allied Commander @%s has joined your raid forces!\n"+
			"They have departed on Leg 1 and are rallying to your base. Once they arrive, they will be stationed for battle.",
		sender.Username,
	)
	_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", creatorUserID, alertCreatorMsg)

	rowsHelpers, errHelpers := tx.QueryContext(ctx, "SELECT e.user_id FROM raid_coop_members rcm JOIN encampments e ON e.id = rcm.encampment_id WHERE rcm.raid_id = $1 AND rcm.encampment_id != $2", raidID, helperCampID)
	if errHelpers == nil {
		defer rowsHelpers.Close()
		for rowsHelpers.Next() {
			var hUserID int64
			if err := rowsHelpers.Scan(&hUserID); err == nil {
				alertMsg := fmt.Sprintf("🤝 CO-OP LOBBY UPDATE: Allied Commander @%s has joined the raid forces! They are rallying to the lead base.", sender.Username)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", hUserID, alertMsg)
			}
		}
		rowsHelpers.Close()
	}

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: "🤝 Joint forces rally initiated! marching to lead base."})
	return h.HandleExpeditionRadar(c)
}

func (h *CombatHandler) HandleLaunchRaidCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	defenderCampID := c.Args()[0]

	var myCampID string
	var myRegion string
	var myX, myY int
	queryMe := `
		SELECT e.id, c.region, c.x, c.y 
		FROM encampments e 
		JOIN coordinates c ON c.id = e.coordinate_id 
		WHERE e.user_id = $1`
	err := h.DB.QueryRowContext(ctx, queryMe, sender.ID).Scan(&myCampID, &myRegion, &myX, &myY)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Outpost core not found. Choose faction first."})
	}

	_, _ = h.DB.ExecContext(ctx, `
		INSERT INTO campaign_drafts (user_id, target_id) 
		VALUES ($1, $2) 
		ON CONFLICT (user_id) DO UPDATE SET target_id = $2, soldiers = 0, mechs = 0, buggies = 0, ships = 0, jets = 0, nukes = 0, destroyers = 0, bombers = 0, battlecruisers = 0, deathstars = 0`, 
		sender.ID, defenderCampID,
	)

	return h.renderDraftCustomizerHUD(c, sender.ID, defenderCampID, myRegion)
}

func (h *CombatHandler) renderDraftCustomizerHUD(c telebot.Context, userID int64, targetCampID string, _ string) error {
	ctx := context.Background()

	var myCampID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", userID).Scan(&myCampID)

	var availSoldiers, availMechs, availBuggies, availShips, availJets, availNukes, availDestroyers, availBombers, availBC, availDS int
	queryInv := `SELECT COALESCE(soldiers, 0), COALESCE(mechs, 0), COALESCE(buggies, 0), COALESCE(ships, 0), COALESCE(jets, 0), COALESCE(nukes, 0), COALESCE(destroyers, 0), COALESCE(bombers, 0), COALESCE(battlecruisers, 0), COALESCE(deathstars, 0) FROM workshop_inventory WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, queryInv, myCampID).Scan(&availSoldiers, &availMechs, &availBuggies, &availShips, &availJets, &availNukes, &availDestroyers, &availBombers, &availBC, &availDS)

	var dSols, dMechs, dBuggies, dShips, dJets, dNukes, dDestroyers, dBombers, dBC, dDS int
	queryDraft := `SELECT soldiers, mechs, buggies, ships, jets, nukes, COALESCE(destroyers,0), COALESCE(bombers,0), COALESCE(battlecruisers,0), COALESCE(deathstars,0) FROM campaign_drafts WHERE user_id = $1`
	_ = h.DB.QueryRowContext(ctx, queryDraft, userID).Scan(&dSols, &dMechs, &dBuggies, &dShips, &dJets, &dNukes, &dDestroyers, &dBombers, &dBC, &dDS)

	panelText := fmt.Sprintf(
		"🎖️━━━━━━━━━━━━━━━━━━━━━━🎖️\n"+
			"✈️ HANGAR CUSTOM CAMPAIGN DRAFT BOARD ✈️\n"+
			"🎖️━━━━━━━━━━━━━━━━━━━━━━🎖️\n"+
			"Select the exact military quantities you want to mobilize:\n\n"+
			"📋 DRAFTED FORCES STOCKPILES:\n"+
			"🪖 Soldiers: %d / %d active\n"+
			"🤖 Mechs: %d / %d active\n"+
			"💥 Destroyers: %d / %d active\n"+
			"🛩️ Bombers: %d / %d active\n"+
			"🚢👑 Battlecruisers: %d / %d active\n"+
			"🌑💀 Doomsday Rigs: %d / %d active\n"+
			"🚗 Buggies: %d / %d active\n"+
			"⛵ Clipper Ships: %d / %d active\n"+
			"✈️ Cargo Jets: %d / %d active\n"+
			"☢️ Nukes: %d / %d active\n\n"+
			"🗺️ TACTICAL ROUTING PATHS:\n"+
			"🚀 [Direct Route] — Base travel speed. Alerts defenders.\n"+
			"🛡️ [Safe Route] — Costs 1.5x Fuel. Travels fast (0.7x duration).\n"+
			"🛰️ [Stealth Route] — Slow travel (1.5x duration). BYPASSES ALL RADAR WARNINGS!\n"+
			"🎖️━━━━━━━━━━━━━━━━━━━━━━🎖️",
		dSols, availSoldiers, dMechs, availMechs, dDestroyers, availDestroyers, dBombers, availBombers, dBC, availBC, dDS, availDS, dBuggies, availBuggies, dShips, availShips, dJets, availJets, dNukes, availNukes,
	)

	selector := &telebot.ReplyMarkup{}

	btnPlusSol := selector.Data("🪖 +Soldier", "adjust_draft", "soldier", "inc")
	btnMinusSol := selector.Data("🪖 -Soldier", "adjust_draft", "soldier", "dec")
	btnPlusMech := selector.Data("🤖 +Mech", "adjust_draft", "mech", "inc")
	btnMinusMech := selector.Data("🤖 -Mech", "adjust_draft", "mech", "dec")
	btnPlusDestroyer := selector.Data("💥 +Destroyer", "adjust_draft", "destroyer", "inc")
	btnMinusDestroyer := selector.Data("💥 -Destroyer", "adjust_draft", "destroyer", "dec")
	btnPlusBomber := selector.Data("🛩️ +Bomber", "adjust_draft", "bomber", "inc")
	btnMinusBomber := selector.Data("🛩️ -Bomber", "adjust_draft", "bomber", "dec")
	btnPlusBC := selector.Data("🚢👑 +Battlecruiser", "adjust_draft", "battlecruiser", "inc")
	btnMinusBC := selector.Data("🚢👑 -Battlecruiser", "adjust_draft", "battlecruiser", "dec")
	btnPlusDS := selector.Data("🌑💀 +Doomsday Rig", "adjust_draft", "deathstar", "inc")
	btnMinusDS := selector.Data("🌑💀 -Doomsday Rig", "adjust_draft", "deathstar", "dec")
	btnPlusBuggy := selector.Data("🚗 +Buggy", "adjust_draft", "buggy", "inc")
	btnMinusBuggy := selector.Data("🚗 -Buggy", "adjust_draft", "buggy", "dec")

	btnPlusShip := selector.Data("⛵ +Ship", "adjust_draft", "ship", "inc")
	btnMinusShip := selector.Data("⛵ -Ship", "adjust_draft", "ship", "dec")
	btnPlusJet := selector.Data("✈️ +Jet", "adjust_draft", "jet", "inc")
	btnMinusJet := selector.Data("✈️ -Jet", "adjust_draft", "jet", "dec")
	btnPlusNuke := selector.Data("☢️ +Nuke", "adjust_draft", "nuke", "inc")
	btnMinusNuke := selector.Data("☢️ -Nuke", "adjust_draft", "nuke", "dec")

	btnConfirmDirect := selector.Data("🚀 Launch Direct", "confirm_launch", targetCampID, "direct")
	btnConfirmStealth := selector.Data("🛰️ Launch Stealth", "confirm_launch", targetCampID, "stealth")
	btnConfirmSafe := selector.Data("🛡️ Launch Safe", "confirm_launch", targetCampID, "safe")

	selector.Inline(
		selector.Row(btnPlusSol, btnMinusSol),
		selector.Row(btnPlusMech, btnMinusMech),
		selector.Row(btnPlusBC, btnMinusBC),
		selector.Row(btnPlusDS, btnMinusDS),
		selector.Row(btnPlusDestroyer, btnMinusDestroyer),
		selector.Row(btnPlusBomber, btnMinusBomber),
		selector.Row(btnPlusBuggy, btnMinusBuggy),
		selector.Row(btnPlusShip, btnMinusShip),
		selector.Row(btnPlusJet, btnMinusJet),
		selector.Row(btnPlusNuke, btnMinusNuke),
		selector.Row(btnConfirmDirect),
		selector.Row(btnConfirmStealth, btnConfirmSafe),
	)

	if c.Callback() != nil {
		return c.Edit(panelText, selector)
	}
	return c.Send(panelText, selector)
}

func (h *CombatHandler) HandleAdjustDraftCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	unitType := c.Args()[0]
	action := c.Args()[1]

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Adjustment failed."})
	}
	defer tx.Rollback()

	var campID string
	var myRegion string

	queryMe := `
		SELECT e.id, c.region 
		FROM encampments e 
		JOIN coordinates c ON c.id = e.coordinate_id 
		WHERE e.user_id = $1`
	err = tx.QueryRowContext(ctx, queryMe, sender.ID).Scan(&campID, &myRegion)
	if err != nil {
		log.Printf("Failed to scan encampment core attributes: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to resolve outpost profile assets."})
	}

	var availSoldiers, availMechs, availBuggies, availShips, availJets, availNukes, availDestroyers, availBombers, availBC, availDS int
	queryInv := `SELECT COALESCE(soldiers, 0), COALESCE(mechs, 0), COALESCE(buggies, 0), COALESCE(ships, 0), COALESCE(jets, 0), COALESCE(nukes, 0), COALESCE(destroyers, 0), COALESCE(bombers, 0), COALESCE(battlecruisers, 0), COALESCE(deathstars, 0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE`
	err = tx.QueryRowContext(ctx, queryInv, campID).Scan(&availSoldiers, &availMechs, &availBuggies, &availShips, &availJets, &availNukes, &availDestroyers, &availBombers, &availBC, &availDS)
	if err != nil {
		log.Printf("Failed query warehouse profile values: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Inventory records inaccessible."})
	}

	var dSols, dMechs, dBuggies, dShips, dJets, dNukes, dDestroyers, dBombers, dBC, dDS int
	var targetCampID string
	queryDraft := `SELECT soldiers, mechs, buggies, ships, jets, nukes, COALESCE(destroyers,0), COALESCE(bombers,0), COALESCE(battlecruisers,0), COALESCE(deathstars,0), target_id FROM campaign_drafts WHERE user_id = $1 FOR UPDATE`
	err = tx.QueryRowContext(ctx, queryDraft, sender.ID).Scan(&dSols, &dMechs, &dBuggies, &dShips, &dJets, &dNukes, &dDestroyers, &dBombers, &dBC, &dDS, &targetCampID)
	if err != nil {
		log.Printf("Draft session select failure: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ No active campaign parameters found."})
	}

	var currentVal, maxVal int
	var dbColumn string

	switch unitType {
	case "soldier":
		currentVal = dSols
		maxVal = availSoldiers
		dbColumn = "soldiers"
	case "mech":
		currentVal = dMechs
		maxVal = availMechs
		dbColumn = "mechs"
	case "destroyer":
		currentVal = dDestroyers
		maxVal = availDestroyers
		dbColumn = "destroyers"
	case "bomber":
		currentVal = dBombers
		maxVal = availBombers
		dbColumn = "bombers"
	case "battlecruiser":
		currentVal = dBC
		maxVal = availBC
		dbColumn = "battlecruisers"
	case "deathstar":
		currentVal = dDS
		maxVal = availDS
		dbColumn = "deathstars"
	case "buggy":
		currentVal = dBuggies
		maxVal = availBuggies
		dbColumn = "buggies"
	case "ship":
		currentVal = dShips
		maxVal = availShips
		dbColumn = "ships"
	case "jet":
		currentVal = dJets
		maxVal = availJets
		dbColumn = "jets"
	case "nuke":
		currentVal = dNukes
		maxVal = availNukes
		dbColumn = "nukes"
	}

	newVal := currentVal
	if action == "inc" {
		if currentVal >= maxVal {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Warehouse Stock: Deploy limit reached!"})
		}
		newVal++
	} else {
		if currentVal <= 0 {
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Invalid Value: Already at 0."})
		}
		newVal--
	}

	queryUpdate := fmt.Sprintf("UPDATE campaign_drafts SET %s = $1 WHERE user_id = $2", dbColumn)
	_, err = tx.ExecContext(ctx, queryUpdate, newVal, sender.ID)
	if err != nil {
		log.Printf("Draft update execution failed: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Configuration persist error."})
	}

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: "⚙️ Fleet draft configuration modified."})

	return h.renderDraftCustomizerHUD(c, sender.ID, targetCampID, myRegion)
}

func (h *CombatHandler) HandleConfirmHangarLaunchCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()

	defenderCampID := c.Args()[0]
	routeType := c.Args()[1]

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Launch failed."})
	}
	defer tx.Rollback()

	var myCampID string
	var myRegion string
	var myX, myY int
	queryMe := `
		SELECT e.id, c.region, c.x, c.y 
		FROM encampments e 
		JOIN coordinates c ON c.id = e.coordinate_id 
		WHERE e.user_id = $1 FOR UPDATE`
	_ = tx.QueryRowContext(ctx, queryMe, sender.ID).Scan(&myCampID, &myRegion, &myX, &myY)

	var activeWeather string
	_ = tx.QueryRowContext(ctx, "SELECT active_weather FROM world_state WHERE id = 1").Scan(&activeWeather)

	var heroID sql.NullString
	_ = tx.QueryRowContext(ctx, "SELECT id FROM heroes WHERE encampment_id = $1", myCampID).Scan(&heroID)

	var mobSoldiers, mobMechs, mobBuggies, mobShips, mobJets, mobNukes, mobDestroyers, mobBombers, mobBC, mobDS int
	queryDraft := `SELECT soldiers, mechs, buggies, ships, jets, nukes, COALESCE(destroyers,0), COALESCE(bombers,0), COALESCE(battlecruisers,0), COALESCE(deathstars,0) FROM campaign_drafts WHERE user_id = $1`
	err = tx.QueryRowContext(ctx, queryDraft, sender.ID).Scan(&mobSoldiers, &mobMechs, &mobBuggies, &mobShips, &mobJets, &mobNukes, &mobDestroyers, &mobBombers, &mobBC, &mobDS)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Staging Timeout: No active draft session located."})
	}

	totMobilized := mobSoldiers + mobMechs + mobBuggies + mobShips + mobJets + mobNukes + mobDestroyers + mobBombers + mobBC + mobDS
	if totMobilized <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Hangar Staging Empty: Allocate at least 1 unit to deploy!"})
	}

	var defenderName string
	var defenderUserID int64
	var defX, defY int
	var defRegion string

	var isAI bool = defenderCampID == "ai_drone_nest"
	if !isAI {
		err = tx.QueryRowContext(ctx, "SELECT e.name, e.user_id, c.x, c.y, c.region FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", defenderCampID).Scan(&defenderName, &defenderUserID, &defX, &defY, &defRegion)
		if err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Target coordinates mismatch."})
		}
	} else {
		defenderName = "Rogue Drone Nest"
		defX = 1
		defY = 1
		defRegion = myRegion
	}

	var marchingMinutes float64

	if defRegion != myRegion {
		if mobJets <= 0 && mobShips <= 0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Ocean Block: You must include at least 1 Clipper Ship or 1 Cargo Jet in your campaign draft to deploy across continents!"})
		}
		if mobJets > 0 {
			marchingMinutes = 120.0
		} else {
			marchingMinutes = 720.0
		}
	} else {
		steps := math.Abs(float64(defX-myX)) + math.Abs(float64(defY-myY))
		if steps == 0 {
			steps = 1
		}
		marchingMinutes = steps * 10.0
		if mobBuggies > 0 {
			marchingMinutes *= 0.75
		}
	}

	switch routeType {
	case "safe":
		marchingMinutes *= 0.7
	case "stealth":
		marchingMinutes *= 1.5
	}

	switch activeWeather {
	case "radiation_storm":
		marchingMinutes *= 1.5
	case "solar_flare":
		marchingMinutes *= 0.7
	case "acid_rain":
		marchingMinutes *= 2.0
	}

	var attackerSpeedTechLvl int = 1
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(speed_tech_lvl, 1) FROM research_states WHERE encampment_id = $1", myCampID).Scan(&attackerSpeedTechLvl)
	speedBonus := math.Min(float64(attackerSpeedTechLvl-1)*0.04, 0.60)
	marchingMinutes *= (1.0 - speedBonus)
	if marchingMinutes < 1.0 {
		marchingMinutes = 1.0
	}

	var electricity float64
	_ = tx.QueryRowContext(ctx, "SELECT electricity FROM resources WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&electricity)

	fuelCost := 30.0
	if routeType == "safe" {
		fuelCost = 45.0
	}

	if electricity < fuelCost {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Electricity: Required %.1f cells.", fuelCost)})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET electricity = electricity - $1 WHERE encampment_id = $2", fuelCost, myCampID)
	_, _ = tx.ExecContext(ctx, `
		UPDATE workshop_inventory 
		SET soldiers = soldiers - $1, mechs = mechs - $2, buggies = buggies - $3, ships = ships - $4, jets = jets - $5, nukes = nukes - $6,
		    destroyers = destroyers - $8, bombers = bombers - $9, battlecruisers = battlecruisers - $10, deathstars = deathstars - $11
		WHERE encampment_id = $7`, 
		mobSoldiers, mobMechs, mobBuggies, mobShips, mobJets, mobNukes, myCampID, mobDestroyers, mobBombers, mobBC, mobDS,
	)

	_, _ = tx.ExecContext(ctx, "DELETE FROM campaign_drafts WHERE user_id = $1", sender.ID)

	marchDuration := time.Duration(marchingMinutes) * time.Minute
	resolveTime := time.Now().UTC().Add(marchDuration)

	var raidID string
	var insertRaid string
	if isAI {
		insertRaid = `
			INSERT INTO raids (attacker_id, defender_id, state, resolve_time) 
			VALUES ($1, NULL, 'marching', $2)
			RETURNING id`
		_ = tx.QueryRowContext(ctx, insertRaid, myCampID, resolveTime).Scan(&raidID)
	} else {
		insertRaid = `
			INSERT INTO raids (attacker_id, defender_id, state, resolve_time) 
			VALUES ($1, $2, 'marching', $3)
			RETURNING id`
		_ = tx.QueryRowContext(ctx, insertRaid, myCampID, defenderCampID, resolveTime).Scan(&raidID)
	}

	_, _ = tx.ExecContext(ctx, "INSERT INTO raid_forces (raid_id, hero_id, soldiers_mobilized, mechs_mobilized, buggies_mobilized, route_type, destroyers_mobilized, bombers_mobilized, battlecruisers_mobilized, deathstars_mobilized) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)", raidID, heroID, mobSoldiers, mobMechs, mobBuggies, routeType, mobDestroyers, mobBombers, mobBC, mobDS)

	newsHeadline := fmt.Sprintf("🚀 MILITARY DEPLOYMENT: Outpost [%s] has deployed marching forces towards Outpost [%s] over [%s Route].", sender.FirstName, defenderName, routeType)
	_, _ = tx.ExecContext(ctx, "INSERT INTO world_news (headline) VALUES ($1)", newsHeadline)

	_ = tx.Commit()

	msg, errAnim := c.Bot().Send(c.Recipient(), "📡 INITIATING SECTOR MARCH TELEMETRY...")
	if errAnim == nil {
		time.Sleep(350 * time.Millisecond)
		_, _ = c.Bot().Edit(msg, "📡 CONNECTING ENGINE SYSTEMS...\n[▰▱▱▱▱▱▱▱▱▱] 10%\n⚡ Thrust vector buffer allocated.")
		time.Sleep(350 * time.Millisecond)
		
		weatherStatus := "Baseline parameters nominal."
		switch activeWeather {
case "radiation_storm":
			weatherStatus = "⚠️ Radiation fallout warnings active over sector grids."
		case "solar_flare":
			weatherStatus = "⚡ Electromagnetic solar interference warning. Accuracy variance applied."
		case "acid_rain":
			weatherStatus = "🌧️ Corrosive precipitation active. Mechs structure structural integrity hazard."
		}
		
		_, _ = c.Bot().Edit(msg, fmt.Sprintf("📡 ANALYSIS ENGINE: WEATHER VECTORS...\n[▰▰▰▰▱▱▱▱▱▱] 40%%\n🌍 Weather Status: %s", weatherStatus))
		time.Sleep(350 * time.Millisecond)
		
		fleetStatus := "Default land speed"
		if mobBuggies > 0 {
			fleetStatus = "🚗 Buggy logistics speed multiplier applied (+25%% speed)."
		}
		if mobJets > 0 {
			fleetStatus = "✈️ Cargo Jet air transit systems fully engaged."
		} else if mobShips > 0 {
			fleetStatus = "⛵ Clipper Ship sea transit structures secured."
		}
		
		_, _ = c.Bot().Edit(msg, fmt.Sprintf("📡 TELEMETRY ENGINE: FLEET ALIGNMENT...\n[▰▰▰▰▰▰▰▰▱▱] 80%%\n⚙️ Engine Sync: %s", fleetStatus))
		time.Sleep(350 * time.Millisecond)
		_, _ = c.Bot().Edit(msg, "📡 SECURE TRANSMISSION ESTABLISHED...\n[▰▰▰▰▰▰▰▰▰▰] 100%\n🚀 Handshake complete! Campaign deployed. Coordinates locked on targeting scanners.")
		time.Sleep(350 * time.Millisecond)
		_ = c.Bot().Delete(msg)
	}

	if !isAI && routeType != "stealth" {
		defenderAlert := fmt.Sprintf(
			"🚨 RADAR ALERT: HOSTILE RAID INBOUND!\n\n"+
				"Our sensors have detected a hostile staged raid marching from Outpost [%s] in %s.\n"+
				"Estimated Arrival Time: %s.",
			sender.FirstName, myRegion, resolveTime.UTC().Format("15:04:05"),
		)
		targetUser := &telebot.User{ID: defenderUserID}
		_, _ = c.Bot().Send(targetUser, defenderAlert)
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, TRUE)", defenderUserID, defenderAlert)
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🚀 Expedition deployed!"})
	return c.Send(fmt.Sprintf("🚀 Raiders deployed! Deployed: %d Soldiers, %d Mechs over [%s Route]. Check Expedition Radar for travel progress.", mobSoldiers, mobMechs, routeType), keyboards.MainNavigation())
}

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

	if state != "marching" && state != "returning" && state != "engaged" && state != "staged" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Already concluded."})
	}

	switch action {
	case "speed":
		if state == "staged" || state == "engaged" {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Action Blocked: Active engagements or staged lobbies cannot be speed-boosted."})
		}
		var scrap, dollars float64
		_ = tx.QueryRowContext(ctx, "SELECT scrap, dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", attackerID).Scan(&scrap, &dollars)
		if scrap < 500.0 || dollars < 100.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Assets: Speed up costs 500 Scrap and $100 Cash."})
		}

		diff := resolveTime.UTC().Sub(time.Now().UTC())
		if diff <= 30*time.Second {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Action Blocked: Campaign is already arriving!"})
		}

		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - 500.0, dollars = dollars - 100.0 WHERE encampment_id = $1", attackerID)
		newResolve := resolveTime.UTC().Add(-30 * time.Minute)
		if time.Until(newResolve) < 0 {
			newResolve = time.Now().UTC().Add(5 * time.Second)
		}

		_, _ = tx.ExecContext(ctx, "UPDATE raids SET resolve_time = $1 WHERE id = $2", newResolve, raidID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "⚡ Speed boosted! Arrival time advanced by 30 minutes."})
		resolveTime = newResolve

	case "abort":
		if state == "staged" {
			var creatorSols, creatorMechs int
			_ = tx.QueryRowContext(ctx, "SELECT soldiers_mobilized, mechs_mobilized FROM raid_forces WHERE raid_id = $1", raidID).Scan(&creatorSols, &creatorMechs)
			
			_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2 WHERE encampment_id = $3", creatorSols, creatorMechs, attackerID)

			rowsCoop, errCoop := tx.QueryContext(ctx, "SELECT encampment_id, soldiers_contributed, mechs_contributed FROM raid_coop_members WHERE raid_id = $1", raidID)
			if errCoop == nil {
				type helperRefund struct {
					campID   string
					userID   int64
					soldiers int
					mechs    int
				}
				var refunds []helperRefund
				for rowsCoop.Next() {
					var rVal helperRefund
					_ = rowsCoop.Scan(&rVal.campID, &rVal.soldiers, &rVal.mechs)
					_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", rVal.campID).Scan(&rVal.userID)
					refunds = append(refunds, rVal)
				}
				rowsCoop.Close()

				for _, rVal := range refunds {
					_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2 WHERE encampment_id = $3", rVal.soldiers, rVal.mechs, rVal.campID)

					cancelAlert := "↩️ CO-OP STAGE CANCELLED: The lobby creator has dismantled the campaign. Your contributed forces have returned safely to base."
					_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", rVal.userID, cancelAlert)
				}
			}

			_, _ = tx.ExecContext(ctx, "DELETE FROM raids WHERE id = $1", raidID)
			
			_ = tx.Commit()
			_ = c.Respond(&telebot.CallbackResponse{Text: "🤝 Co-Op lobby cancelled! Mobilized forces refunded."})
			return c.Send("↩️ Co-Op lobby dismantled. All draft troops and helper contributions have returned safely.", keyboards.MainNavigation())
		}

		var createdAt time.Time
		_ = tx.QueryRowContext(ctx, "SELECT created_at FROM raids WHERE id = $1", raidID).Scan(&createdAt)

		if state == "engaged" {
			var soldiersMob, mechsMob int
			_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers_mobilized, 0), COALESCE(mechs_mobilized, 0) FROM raid_forces WHERE raid_id = $1 FOR UPDATE", raidID).Scan(&soldiersMob, &mechsMob)

			lostSoldiers := int(float64(soldiersMob) * 0.15)
			lostMechs := int(float64(mechsMob) * 0.15)

			survSoldiers := soldiersMob - lostSoldiers
			survMechs := mechsMob - lostMechs

			if survSoldiers < 0 {
				survSoldiers = 0
			}
			if survMechs < 0 {
				survMechs = 0
			}

			_, _ = tx.ExecContext(ctx, "UPDATE raid_forces SET soldiers_mobilized = $1, mechs_mobilized = $2 WHERE raid_id = $3", survSoldiers, survMechs, raidID)
		}

		rowsCoop, errCoop := tx.QueryContext(ctx, "SELECT encampment_id, soldiers_contributed, mechs_contributed FROM raid_coop_members WHERE raid_id = $1", raidID)
		if errCoop == nil {
			type helperRefund struct {
				campID   string
				userID   int64
				soldiers int
				mechs    int
			}
			var refunds []helperRefund
			for rowsCoop.Next() {
				var rVal helperRefund
				_ = rowsCoop.Scan(&rVal.campID, &rVal.soldiers, &rVal.mechs)
				_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", rVal.campID).Scan(&rVal.userID)
				refunds = append(refunds, rVal)
			}
			rowsCoop.Close()

			for _, rVal := range refunds {
				_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1, mechs = mechs + $2 WHERE encampment_id = $3", rVal.soldiers, rVal.mechs, rVal.campID)

				retreatAlert := "↩️ CO-OP MISSION ABORTED: The raid creator has ordered a strategic retreat. Your contributed survivors have returned safely to base."
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", rVal.userID, retreatAlert)
			}
			_, _ = tx.ExecContext(ctx, "DELETE FROM raid_coop_members WHERE raid_id = $1", raidID)
		}

		var scrap float64
		_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", attackerID).Scan(&scrap)

		penalty := scrap * 0.20
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", penalty, attackerID)

		elapsed := time.Since(createdAt.UTC())
		if elapsed < 10*time.Second {
			elapsed = 10 * time.Second
		}

		returnResolveTime := time.Now().UTC().Add(elapsed)

		_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'returning', resolve_time = $1 WHERE id = $2", returnResolveTime, raidID)

		var attackerName string
		_ = tx.QueryRowContext(ctx, "SELECT name FROM encampments WHERE id = $1", attackerID).Scan(&attackerName)
		newsHeadline := fmt.Sprintf("↩️ TACTICAL RETREAT: Outpost [%s] has ordered a strategic retreat. Survivors are returning back to hangar.", attackerName)
		_, _ = tx.ExecContext(ctx, "INSERT INTO world_news (headline) VALUES ($1)", newsHeadline)

		_ = c.Respond(&telebot.CallbackResponse{Text: "↩️ Tactical retreat engaged! Return march started."})
		_ = tx.Commit()
		return c.Send("↩️ Tactical retreat ordered. Remaining forces are marching back to base.", keyboards.MainNavigation())
	}

	_ = tx.Commit()

	var attackerName string
	_ = h.DB.QueryRowContext(ctx, "SELECT name FROM encampments WHERE id = $1", attackerID).Scan(&attackerName)

	return h.renderExpeditionPanel(c, raidID, attackerName, resolveTime)
}

func (h *CombatHandler) renderExpeditionPanel(c telebot.Context, raidID, attackerName string, resolveTime time.Time) error {
	diff := resolveTime.UTC().Sub(time.Now().UTC())
	timeLeft := int(diff.Seconds())
	if timeLeft < 0 {
		timeLeft = 0
	}

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🛰️ EXPEDITION COMMAND PANEL\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Attacker: %s\n"+
			"Estimated Arrival: %s (%ds remaining)\n\n"+
			"Use the action buttons to speed up or abort the current expedition.\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		attackerName,
		resolveTime.UTC().Format("15:04:05"),
		timeLeft,
	)

	selector := &telebot.ReplyMarkup{}
	btnSpeed := selector.Data("⚡ Speed Up", "exp_action", "speed", raidID)
	btnAbort := selector.Data("↩️ Abort", "exp_action", "abort", raidID)
	selector.Inline(selector.Row(btnSpeed, btnAbort))

	return c.Send(panelText, selector)
}