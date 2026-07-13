package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/telebot.v3"
)

type ProfileHandler struct {
	DB *sql.DB
}

func NewProfileHandler(db *sql.DB) *ProfileHandler {
	return &ProfileHandler{DB: db}
}

// ── /description ─────────────────────────────────────────────────────

func (h *ProfileHandler) HandleDescription(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	desc := strings.TrimSpace(c.Message().Payload)
	if desc == "" {
		var current string
		_ = h.DB.QueryRowContext(ctx, "SELECT description FROM users WHERE telegram_id = $1", sender.ID).Scan(&current)
		if current == "" {
			current = "(none set)"
		}
		return c.Send(fmt.Sprintf("📝 YOUR DESCRIPTION:\n\"%s\"\n\nUsage: /description [text] (max 200 characters)", current))
	}

	if len(desc) > 200 {
		return c.Send("❌ Too Long: Description must be 200 characters or fewer.")
	}

	_, err := h.DB.ExecContext(ctx, "UPDATE users SET description = $1 WHERE telegram_id = $2", desc, sender.ID)
	if err != nil {
		return c.Send("⚠️ Error saving description.")
	}
	return c.Send("✅ Description updated!")
}

// ── /settings ─────────────────────────────────────────────────────────

func (h *ProfileHandler) HandleSettings(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	var notifyRaid, notifyStorage bool
	_ = h.DB.QueryRowContext(ctx, "SELECT notify_on_raid, notify_on_storage_full FROM users WHERE telegram_id = $1", sender.ID).Scan(&notifyRaid, &notifyStorage)

	panelText := fmt.Sprintf(
		"⚙️━━━━━━━━━━━━━━━━━━━━━━⚙️\n"+
			"🎛️ ADVANCED GAMEPLAY SETTINGS 🎛️\n"+
			"⚙️━━━━━━━━━━━━━━━━━━━━━━⚙️\n\n"+
			"🚨 Incoming Raid Alerts: %s\n"+
			"📦 Storage Full Alerts: %s\n"+
			"⚙️━━━━━━━━━━━━━━━━━━━━━━⚙️",
		onOff(notifyRaid), onOff(notifyStorage),
	)

	selector := &telebot.ReplyMarkup{}
	btnRaid := selector.Data("🚨 Toggle Raid Alerts", "settings_toggle", "raid")
	btnStorage := selector.Data("📦 Toggle Storage Alerts", "settings_toggle", "storage")
	selector.Inline(selector.Row(btnRaid), selector.Row(btnStorage))

	return c.Send(panelText, selector)
}

func onOff(b bool) string {
	if b {
		return "✅ ON"
	}
	return "❌ OFF"
}

func (h *ProfileHandler) HandleSettingsToggleCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	setting := c.Args()[0]

	var column string
	switch setting {
	case "raid":
		column = "notify_on_raid"
	case "storage":
		column = "notify_on_storage_full"
	default:
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Unknown setting."})
	}

	query := fmt.Sprintf("UPDATE users SET %s = NOT %s WHERE telegram_id = $1", column, column)
	_, err := h.DB.ExecContext(ctx, query, sender.ID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error updating setting."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "✅ Setting updated!"})
	return h.HandleSettings(c)
}

// ── /refer ────────────────────────────────────────────────────────────

func (h *ProfileHandler) HandleRefer(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	var code string
	err := h.DB.QueryRowContext(ctx, "SELECT referral_code FROM users WHERE telegram_id = $1", sender.ID).Scan(&code)
	if err != nil || code == "" {
		code = fmt.Sprintf("REF%d", sender.ID%1000000)
		_, _ = h.DB.ExecContext(ctx, "UPDATE users SET referral_code = $1 WHERE telegram_id = $2", code, sender.ID)
	}

	var referralCount int
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE referred_by = $1", sender.ID).Scan(&referralCount)

	panelText := fmt.Sprintf(
		"🎁━━━━━━━━━━━━━━━━━━━━━━🎁\n"+
			"👥 REFER YOUR FRIENDS 👥\n"+
			"🎁━━━━━━━━━━━━━━━━━━━━━━🎁\n\n"+
			"Share your code with friends. When they start with /start %s, you both earn rewards!\n\n"+
			"🔑 Your Referral Code: %s\n"+
			"👥 Friends Referred: %d\n"+
			"🎁 Reward per referral: 500 Metal + 200 Crystal + 100 Neuro Cores\n"+
			"🎁━━━━━━━━━━━━━━━━━━━━━━🎁",
		code, code, referralCount,
	)

	return c.Send(panelText)
}

// ── /feedback ─────────────────────────────────────────────────────────

func (h *ProfileHandler) HandleFeedback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	msg := strings.TrimSpace(c.Message().Payload)
	if msg == "" {
		return c.Send("⚠️ Usage: /feedback [your message]\n\nYour feedback goes straight to the development team.")
	}

	_, err := h.DB.ExecContext(ctx, "INSERT INTO feedback_submissions (user_id, message) VALUES ($1, $2)", sender.ID, msg)
	if err != nil {
		return c.Send("⚠️ Error submitting feedback.")
	}

	return c.Send("📨 Feedback received - thank you for helping improve The Vagabond!")
}

// ── /msg ──────────────────────────────────────────────────────────────

func (h *ProfileHandler) HandleMsg(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	parts := strings.SplitN(strings.TrimSpace(c.Message().Payload), " ", 2)
	if len(parts) < 2 {
		return c.Send("⚠️ Usage: /msg [username] [message]")
	}
	targetUsername := strings.TrimPrefix(parts[0], "@")
	messageText := parts[1]

	var targetID int64
	err := h.DB.QueryRowContext(ctx, "SELECT telegram_id FROM users WHERE LOWER(username) = LOWER($1)", targetUsername).Scan(&targetID)
	if err != nil {
		return c.Send("❌ Player not found.")
	}

	var isMuted bool
	_ = h.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM user_mutes WHERE muter_id = $1 AND muted_id = $2)", targetID, sender.ID).Scan(&isMuted)
	if isMuted {
		return c.Send("🔇 This player has muted you - your message wasn't delivered.")
	}

	alertMsg := fmt.Sprintf("💬 MESSAGE from %s:\n\n%s", sender.FirstName, messageText)
	_, err = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", targetID, alertMsg)
	if err != nil {
		return c.Send("⚠️ Error sending message.")
	}

	return c.Send(fmt.Sprintf("✅ Message sent to %s!", targetUsername))
}

// ── /mute, /unmute, /mutes ──────────────────────────────────────────────

func (h *ProfileHandler) HandleMute(c telebot.Context) error {
	return h.muteAction(c, true)
}

func (h *ProfileHandler) HandleUnmute(c telebot.Context) error {
	return h.muteAction(c, false)
}

func (h *ProfileHandler) muteAction(c telebot.Context, mute bool) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	targetUsername := strings.TrimPrefix(strings.TrimSpace(c.Message().Payload), "@")
	if targetUsername == "" {
		if mute {
			return c.Send("⚠️ Usage: /mute [username]")
		}
		return c.Send("⚠️ Usage: /unmute [username]")
	}

	var targetID int64
	err := h.DB.QueryRowContext(ctx, "SELECT telegram_id FROM users WHERE LOWER(username) = LOWER($1)", targetUsername).Scan(&targetID)
	if err != nil {
		return c.Send("❌ Player not found.")
	}

	if mute {
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO user_mutes (muter_id, muted_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", sender.ID, targetID)
		return c.Send(fmt.Sprintf("🔇 %s has been muted. Their messages will no longer reach you.", targetUsername))
	}

	_, _ = h.DB.ExecContext(ctx, "DELETE FROM user_mutes WHERE muter_id = $1 AND muted_id = $2", sender.ID, targetID)
	return c.Send(fmt.Sprintf("🔊 %s has been unmuted.", targetUsername))
}

func (h *ProfileHandler) HandleMutesList(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	rows, err := h.DB.QueryContext(ctx, "SELECT u.username FROM user_mutes um JOIN users u ON u.telegram_id = um.muted_id WHERE um.muter_id = $1", sender.ID)
	if err != nil {
		return c.Send("⚠️ Error loading muted players.")
	}
	defer rows.Close()

	panelText := "🔇 MUTED PLAYERS:\n\n"
	any := false
	for rows.Next() {
		var username string
		if rows.Scan(&username) == nil {
			any = true
			panelText += fmt.Sprintf("🔇 @%s\n", username)
		}
	}
	if !any {
		panelText += "(none)"
	}

	return c.Send(panelText)
}

// ── /log ──────────────────────────────────────────────────────────────

func (h *ProfileHandler) HandleLog(c telebot.Context) error {
	ctx := context.Background()

	rows, err := h.DB.QueryContext(ctx, "SELECT message, created_at FROM event_log ORDER BY created_at DESC LIMIT 15")
	if err != nil {
		return c.Send("⚠️ Unable to reach the event log.")
	}
	defer rows.Close()

	panelText := "📜━━━━━━━━━━━━━━━━━━━━━━📜\n" +
		"🗞️ LATEST WASTELAND EVENTS 🗞️\n" +
		"📜━━━━━━━━━━━━━━━━━━━━━━📜\n\n"

	any := false
	for rows.Next() {
		var msg string
		var createdAt sql.NullTime
		if rows.Scan(&msg, &createdAt) == nil {
			any = true
			panelText += fmt.Sprintf("• %s\n", msg)
		}
	}
	if !any {
		panelText += "No major events logged recently.\n"
	}
	panelText += "📜━━━━━━━━━━━━━━━━━━━━━━📜"

	return c.Send(panelText)
}

// ── /stats ────────────────────────────────────────────────────────────

func (h *ProfileHandler) HandleStats(c telebot.Context) error {
	ctx := context.Background()

	var totalPlayers, totalClans, totalFederations int
	var totalRaids int
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&totalPlayers)
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM clans").Scan(&totalClans)
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM federations").Scan(&totalFederations)
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM raids").Scan(&totalRaids)

	var totalMetal, totalCrystal, totalScrap float64
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(SUM(metal),0), COALESCE(SUM(crystal),0), COALESCE(SUM(scrap),0) FROM resources").Scan(&totalMetal, &totalCrystal, &totalScrap)

	panelText := fmt.Sprintf(
		"📊━━━━━━━━━━━━━━━━━━━━━━📊\n"+
			"🌍 GLOBAL WASTELAND STATISTICS 🌍\n"+
			"📊━━━━━━━━━━━━━━━━━━━━━━📊\n\n"+
			"👥 Total Survivors: %d\n"+
			"🏴 Total Clans: %d\n"+
			"🌐 Total Federations: %d\n"+
			"⚔️ Total Raids Launched: %d\n\n"+
			"🌎 ECONOMY-WIDE TOTALS:\n"+
			"🔩 Metal in circulation: %.0f\n"+
			"💎 Crystal in circulation: %.0f\n"+
			"⚙️ Scrap in circulation: %.0f\n"+
			"📊━━━━━━━━━━━━━━━━━━━━━━📊",
		totalPlayers, totalClans, totalFederations, totalRaids, totalMetal, totalCrystal, totalScrap,
	)

	return c.Send(panelText)
}

// ── /units ────────────────────────────────────────────────────────────

func (h *ProfileHandler) HandleUnits(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	var soldiers, drones, mechs, nukes, buggies, ships, jets, haulers, tankers, rigs, destroyers, bombers, scouts, battlecruisers, deathstars int
	query := `SELECT COALESCE(soldiers,0), COALESCE(drones,0), COALESCE(mechs,0), COALESCE(nukes,0), 
	          COALESCE(buggies,0), COALESCE(ships,0), COALESCE(jets,0), 
	          COALESCE(haulers,0), COALESCE(tankers,0), COALESCE(rigs,0),
	          COALESCE(destroyers,0), COALESCE(bombers,0), COALESCE(scouts,0), COALESCE(battlecruisers,0), COALESCE(deathstars,0)
	          FROM workshop_inventory WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, query, campID).Scan(&soldiers, &drones, &mechs, &nukes, &buggies, &ships, &jets, &haulers, &tankers, &rigs, &destroyers, &bombers, &scouts, &battlecruisers, &deathstars)

	var marchingCount int
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM raids WHERE attacker_id = $1 AND state IN ('marching', 'engaged', 'returning')", campID).Scan(&marchingCount)

	panelText := fmt.Sprintf(
		"🪖━━━━━━━━━━━━━━━━━━━━━━🪖\n"+
			"📋 YOUR UNITS & STATUS 📋\n"+
			"🪖━━━━━━━━━━━━━━━━━━━━━━🪖\n\n"+
			"🏠 GARRISONED (at base):\n"+
			"🪖 Soldiers: %d\n"+
			"🛰️ Tactical Drones: %d\n"+
			"🤖 Mechs: %d\n"+
			"☢️ Nukes: %d\n"+
			"💥 Destroyers: %d\n"+
			"🛩️ Bombers: %d\n"+
			"🛵 Scouts: %d\n"+
			"🚢👑 Battlecruisers: %d\n"+
			"🌑💀 Doomsday Rigs: %d\n"+
			"🚗 Buggies: %d | ⛵ Ships: %d | ✈️ Jets: %d\n"+
			"🚛 Haulers: %d | 🛡️ Tankers: %d | 🔧 Rigs: %d\n\n"+
			"🚀 ON ACTIVE MISSIONS: %d fleet(s) deployed (check /missions)\n"+
			"🪖━━━━━━━━━━━━━━━━━━━━━━🪖",
		soldiers, drones, mechs, nukes, destroyers, bombers, scouts, battlecruisers, deathstars,
		buggies, ships, jets, haulers, tankers, rigs, marchingCount,
	)

	return c.Send(panelText)
}
