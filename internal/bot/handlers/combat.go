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
				// Timezone-Normalized HUD Timers: Compute countdown strictly inside UTC boundaries
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
		// Timezone-Normalized HUD Timers: Compute countdown strictly inside UTC boundaries
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
		SELECT s.id, ea.name, ed.name, s.created_at, (s.spy_id = $1) as is_outbound
		FROM spy_missions s
		JOIN encampments ea ON ea.id = s.spy_id
		JOIN encampments ed ON ed.id = s.target_id
		WHERE s.is_intercepted = FALSE AND (s.spy_id = $1 OR s.target_id = $1) AND s.resolved = FALSE`

	rowsSpies, err := h.DB.QueryContext(ctx, querySpies, campID)
	spyText := ""
	if err == nil {
		defer rowsSpies.Close()
		for rowsSpies.Next() {
			var spyID, eaName, edName string
			var createdAt time.Time
			var isOutbound bool
			if err := rowsSpies.Scan(&spyID, &eaName, &edName, &createdAt, &isOutbound); err == nil {
				timeLeft := 30 - int(time.Since(createdAt.UTC()).Seconds())
				if timeLeft < 0 {
					timeLeft = 0
				}
				if isOutbound {
					spyText += fmt.Sprintf("🛰️ ACTIVE OUTBOUND SCAN: Scanning %s\n   Uplink Status: Decrypting (%ds remaining)\n\n", edName, timeLeft)
				} else {
					spyText += fmt.Sprintf("📡 INCOMING ESPIONAGE BREACH: Rival %s\n   Uplink Status: Intercept Window (%ds remaining)\n\n", eaName, timeLeft)
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

	var energy float64
	_ = tx.QueryRowContext(ctx, "SELECT energy FROM resources WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&energy)

	if energy < 30.0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Energy: Satellite scans require 30.0 Energy Cells."})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET energy = energy - 30.0 WHERE encampment_id = $1", myCampID)
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

	defenderAlert := fmt.Sprintf(
		"🛰️ ESPIONAGE INTRUSION DETECTED!\n\n"+
			"A hostile Spy Satellite launched by Outpost [%s] has breached your wireless perimeter and is transmitting warehouse telemetry!\n\n"+
			"⚠️ Intercept Window: 30 seconds. Spend 10.0 Energy Cells to vaporize the uplink.",
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

	var isIntercepted bool
	var resolved bool
	var attackerCampID string
	var createdAt time.Time
	err = tx.QueryRowContext(ctx, "SELECT is_intercepted, resolved, spy_id, created_at FROM spy_missions WHERE id = $1 FOR UPDATE", spyID).Scan(&isIntercepted, &resolved, &attackerCampID, &createdAt)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Connection Closed: This satellite has already returned to orbit."})
	}

	if isIntercepted {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Already Neutralized."})
	}

	var energy float64
	_ = tx.QueryRowContext(ctx, "SELECT energy FROM resources WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&energy)

	if energy < 10.0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Energy: Drones require 10.0 Energy Cells."})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET energy = energy - 10.0 WHERE encampment_id = $1", myCampID)

	var attackerUserID int64
	_ = tx.QueryRowContext(ctx, "SELECT user_id FROM encampments WHERE id = $1", attackerCampID).Scan(&attackerUserID)

	if resolved {
		rSource := rand.NewSource(time.Now().UnixNano() + sender.ID)
		rGen := rand.New(rSource)
		if rGen.Float64() < 0.60 {
			_, _ = tx.ExecContext(ctx, "UPDATE spy_missions SET is_intercepted = TRUE, resolved = TRUE WHERE id = $1", spyID)
			_ = tx.Commit()
			_ = c.Respond(&telebot.CallbackResponse{Text: "🛡️ Interceptor Drone chased down and destroyed the returning spy satellite!"})
			
			attackerUser := &telebot.User{ID: attackerUserID}
			_, _ = c.Bot().Send(attackerUser, "💥 CHASE INTERCEPT: Your returning spy satellite was chased down and vaporized by defender Interceptor Drones! Intel was lost.")
			return c.Send("🛡️ CHASE SUCCESS: Your Interceptor Drone chased down the spy satellite and vaporized the decrypted intel!")
		} else {
			_ = tx.Commit()
			return c.Send("❌ CHASE FAILED: The spy satellite was too fast! Your Interceptor Drone failed to catch it before it exited communications range. Interceptor lost.")
		}
	}

	_, _ = tx.ExecContext(ctx, "UPDATE spy_missions SET is_intercepted = TRUE, resolved = TRUE WHERE id = $1", spyID)
	_ = tx.Commit()

	_ = c.Respond(&telebot.CallbackResponse{Text: "🛡️ Interceptor Drone launched! Tracking satellite..."})

	attackerUser := &telebot.User{ID: attackerUserID}
	_, _ = c.Bot().Send(attackerUser, "💥 SPY INTERCEPTED: Your returning Spy Satellite was intercepted and destroyed by hostile air defense systems! Telemetry lost.")

	return c.Send("🛡️ INTERCEPT SUCCESS: Your Interceptor Drone destroyed the spy satellite! Telemetry was vaporized.")
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

	_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers - $1, mechs = mechs - $2 WHERE encampment_id = $3", conSoldiers, conMechs, helperCampID)

	queryJointMember := `
		INSERT INTO raid_coop_members (raid_id, encampment_id, soldiers_contributed, mechs_contributed)
		VALUES ($1, $2, $3, $4)`
	_, err = tx.ExecContext(ctx, queryJointMember, raidID, helperCampID, conSoldiers, conMechs)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing helper contribution details."})
	}

	var primSoldiers, primMechs int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(soldiers, 0), COALESCE(mechs, 0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", attackerCampID).Scan(&primSoldiers, &primMechs)

	_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = 0, mechs = 0 WHERE encampment_id = $1", attackerCampID)

	_, _ = tx.ExecContext(ctx, `
		INSERT INTO raid_forces (raid_id, soldiers_mobilized, mechs_mobilized, buggies_mobilized, route_type) 
		VALUES ($1, $2, $3, 0, 'direct')
		ON CONFLICT (raid_id) DO UPDATE SET soldiers_mobilized = $2, mechs_mobilized = $3`, 
		raidID, primSoldiers, primMechs,
	)

	_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'marching', resolve_time = $1 WHERE id = $2", time.Now().UTC().Add(15*time.Minute), raidID)

	var creatorUserID int64
	var targetOutpostName string
	_ = tx.QueryRowContext(ctx, `
		SELECT e.user_id, COALESCE(ed.name, 'Rogue Drone Nest') 
		FROM raids r 
		JOIN encampments e ON e.id = r.attacker_id 
		LEFT JOIN encampments ed ON ed.id = r.defender_id
		WHERE r.id = $1`, raidID).Scan(&creatorUserID, &targetOutpostName)

	alertCreatorMsg := fmt.Sprintf(
		"🤝 CO-OP LOBBY DEPARTURE: Allied Commander @%s has successfully contributed units and joined your staged raid against Outpost [%s]!\n"+
			"Your joint military forces have permanently departed and are marching towards coordinates.",
		sender.Username, targetOutpostName,
	)
	_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", creatorUserID, alertCreatorMsg)

	rowsHelpers, errHelpers := tx.QueryContext(ctx, "SELECT e.user_id FROM raid_coop_members rcm JOIN encampments e ON e.id = rcm.encampment_id WHERE rcm.raid_id = $1 AND rcm.encampment_id != $2", raidID, helperCampID)
	if errHelpers == nil {
		defer rowsHelpers.Close()
		for rowsHelpers.Next() {
			var hUserID int64
			if err := rowsHelpers.Scan(&hUserID); err == nil {
				alertMsg := fmt.Sprintf("🤝 CO-OP LOBBY UPDATE: Allied Commander @%s has joined your raid forces! Campaign has departed marching towards coordinates.", sender.Username)
				_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", hUserID, alertMsg)
			}
		}
		rowsHelpers.Close()
	}

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: "🤝 Joint forces bound! Marching towards coordinates."})
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

	var soldiers, mechs, buggies int
	queryInv := `SELECT COALESCE(soldiers, 0), COALESCE(mechs, 0), COALESCE(buggies, 0) FROM workshop_inventory WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, queryInv, myCampID).Scan(&soldiers, &mechs, &buggies)

	totForce := soldiers + mechs + buggies
	if totForce <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Hangar Empty: Recruit soldiers or craft vehicles first!"})
	}

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"✈️ HANGAR COMMAND EXPEDITION DECK\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Select an offensive force deployment size and route coordinates:\n\n"+
			"BARRACKS STOCKPILES:\n"+
			"🪖 Soldiers: %d | 🤖 Mechs: %d | 🚗 Buggies: %d\n\n"+
			"TACTICAL ROUTING DECK:\n"+
			"🚀 [Direct Route] — Base travel speed. Alerts defenders.\n"+
			"🛡️ [Safe Route] — Costs 1.5x Fuel. Travels fast (0.7x duration).\n"+
			"🛰️ [Stealth Route] — Slow travel (1.5x duration). BYPASSES ALL RADAR WARNINGS!\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		soldiers, mechs, buggies,
	)

	selector := &telebot.ReplyMarkup{}

	btnDirect25 := selector.Data("🚀 Deploy 25% [Direct]", "confirm_launch", defenderCampID, "25", "direct")
	btnDirect50 := selector.Data("🚀 Deploy 50% [Direct]", "confirm_launch", defenderCampID, "50", "direct")
	btnDirect100 := selector.Data("🚀 Deploy 100% [Direct]", "confirm_launch", defenderCampID, "100", "direct")
	btnStealth100 := selector.Data("🛰️ Deploy 100% [Stealth]", "confirm_launch", defenderCampID, "100", "stealth")
	btnSafe100 := selector.Data("🛡️ Deploy 100% [Safe]", "confirm_launch", defenderCampID, "100", "safe")

	selector.Inline(
		selector.Row(btnDirect25, btnDirect50),
		selector.Row(btnDirect100),
		selector.Row(btnStealth100, btnSafe100),
	)

	return c.Send(panelText, selector)
}

func (h *CombatHandler) HandleConfirmHangarLaunchCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()

	defenderCampID := c.Args()[0]
	weightPctStr := c.Args()[1]
	routeType := c.Args()[2]

	weightPct, _ := strconv.Atoi(weightPctStr)

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

	var soldiers, mechs, buggies int
	queryInv := `SELECT COALESCE(soldiers, 0), COALESCE(mechs, 0), COALESCE(buggies, 0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE`
	_ = tx.QueryRowContext(ctx, queryInv, myCampID).Scan(&soldiers, &mechs, &buggies)

	mobRatio := float64(weightPct) / 100.0
	mobSoldiers := int(float64(soldiers) * mobRatio)
	mobMechs := int(float64(mechs) * mobRatio)
	mobBuggies := int(float64(buggies) * mobRatio)

	if mobSoldiers <= 0 && mobMechs <= 0 {
		if soldiers > 0 {
			mobSoldiers = 1
		} else if mechs > 0 {
			mobMechs = 1
		} else {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient troops to allocate."})
		}
	}

	var energy float64
	_ = tx.QueryRowContext(ctx, "SELECT energy FROM resources WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&energy)

	fuelCost := 30.0
	if routeType == "safe" {
		fuelCost = 45.0
	}

	if energy < fuelCost {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Energy: Required %.1f cells.", fuelCost)})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET energy = energy - $1 WHERE encampment_id = $2", fuelCost, myCampID)
	_, _ = tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers - $1, mechs = mechs - $2, buggies = buggies - $3 WHERE encampment_id = $4", mobSoldiers, mobMechs, mobBuggies, myCampID)

	var marchingMinutes float64

	if defenderCampID == "ai_drone_nest" {
		steps := math.Abs(float64(1-myX)) + math.Abs(float64(1-myY))
		if steps == 0 {
			steps = 1
		}
		marchingMinutes = steps * 10.0
		if marchingMinutes < 15.0 {
			marchingMinutes = 15.0
		}

		resolveTime := time.Now().UTC().Add(time.Duration(marchingMinutes) * time.Minute)

		insertRaid := `
			INSERT INTO raids (attacker_id, defender_id, state, resolve_time) 
			VALUES ($1, NULL, 'marching', $2)
			RETURNING id`
		var raidID string
		_ = tx.QueryRowContext(ctx, insertRaid, myCampID, resolveTime).Scan(&raidID)

		_, _ = tx.ExecContext(ctx, "INSERT INTO raid_forces (raid_id, hero_id, soldiers_mobilized, mechs_mobilized, buggies_mobilized, route_type) VALUES ($1, $2, $3, $4, $5, $6)", raidID, heroID, mobSoldiers, mobMechs, mobBuggies, routeType)

		_ = tx.Commit()
		_ = c.Respond(&telebot.CallbackResponse{Text: "🤖 Skirmish forces deployed!"})
		return c.Send(fmt.Sprintf("🤖 Skirmish launched! Your army of %d Soldiers and %d Mechs is marching on Rogue Drone Nest...", mobSoldiers, mobMechs), keyboards.MainNavigation())
	}

	var defenderName string
	var defenderUserID int64
	var defX, defY int
	var defRegion string
	_ = tx.QueryRowContext(ctx, "SELECT e.name, e.user_id, c.x, c.y, c.region FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.id = $1", defenderCampID).Scan(&defenderName, &defenderUserID, &defX, &defY, &defRegion)

	if defRegion != myRegion {
		var jets, ships int
		_ = tx.QueryRowContext(ctx, "SELECT COALESCE(jets, 0), COALESCE(ships, 0) FROM workshop_inventory WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&jets, &ships)
		if jets <= 0 && ships <= 0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Ocean Block: Build a Clipper Ship or Cargo Jet first to deploy across continents!"})
		}
		if jets > 0 {
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
	}

	marchDuration := time.Duration(marchingMinutes) * time.Minute
	resolveTime := time.Now().UTC().Add(marchDuration)

	var raidID string
	insertRaid := `
		INSERT INTO raids (attacker_id, defender_id, state, resolve_time) 
		VALUES ($1, $2, 'marching', $3)
		RETURNING id`
	_ = tx.QueryRowContext(ctx, insertRaid, myCampID, defenderCampID, resolveTime).Scan(&raidID)

	_, _ = tx.ExecContext(ctx, "INSERT INTO raid_forces (raid_id, hero_id, soldiers_mobilized, mechs_mobilized, buggies_mobilized, route_type) VALUES ($1, $2, $3, $4, $5, $6)", raidID, heroID, mobSoldiers, mobMechs, mobBuggies, routeType)

	newsHeadline := fmt.Sprintf("🚀 MILITARY DEPLOYMENT: Outpost [%s] has deployed marching forces towards Outpost [%s] over [%s Route].", sender.FirstName, defenderName, routeType)
	_, _ = tx.ExecContext(ctx, "INSERT INTO world_news (headline) VALUES ($1)", newsHeadline)

	_ = tx.Commit()

	if routeType != "stealth" {
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

	if state != "marching" && state != "returning" && state != "engaged" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Already concluded."})
	}

	switch action {
	case "speed":
		if state == "engaged" {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Action Blocked: Active engagements cannot be speed-boosted."})
		}
		var scrap, dollars float64
		_ = tx.QueryRowContext(ctx, "SELECT scrap, dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", attackerID).Scan(&scrap, &dollars)
		if scrap < 500.0 || dollars < 100.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Assets: Speed up costs 500 Scrap and $100 Cash."})
		}

		// Timezone-Normalized HUD Timers: Compute countdown strictly inside UTC boundaries
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
		var createdAt time.Time
		_ = tx.QueryRowContext(ctx, "SELECT created_at FROM raids WHERE id = $1", raidID).Scan(&createdAt)

		// Tactical Retreat cover casualties penalty (15%) when aborting under active combat fire
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

		// Secure Co-Op Helper Retreat & Refunds: Automatically refund all helper forces safely on retreat
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

		// Timezone-Normalized HUD Timers: Compute countdown strictly inside UTC boundaries
		returnResolveTime := time.Now().UTC().Add(elapsed)

		_, _ = tx.ExecContext(ctx, "UPDATE raids SET state = 'returning', resolve_time = $1 WHERE id = $2", returnResolveTime, raidID)

		// Broadcast retreat news to the live radio feed
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
	// Timezone-Normalized HUD Timers: Compute countdown strictly inside UTC boundaries
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