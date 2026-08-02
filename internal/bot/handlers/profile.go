package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
	"gopkg.in/telebot.v3"
)

type ProfileHandler struct {
	DB       *sql.DB
	AdminIDs []int64

	// Mirrors admin.go's pendingMu/pending pattern (see its own doc
	// comment for the full reasoning): tracks which player is mid-flow
	// on the "💬 Send Feedback" button, so their next free-text message
	// is captured as the feedback body instead of falling through to
	// normal NLP parsing. In-memory rather than DB-backed like
	// onboarding's naming flow - losing this on a restart just means
	// the player taps the button again, low enough stakes not to need
	// persistence.
	feedbackPendingMu sync.Mutex
	feedbackPending   map[int64]bool
}

func NewProfileHandler(db *sql.DB, adminIDs []int64) *ProfileHandler {
	return &ProfileHandler{DB: db, AdminIDs: adminIDs, feedbackPending: make(map[int64]bool)}
}

func (h *ProfileHandler) IsAdmin(senderID int64) bool {
	for _, id := range h.AdminIDs {
		if id == senderID {
			return true
		}
	}
	return false
}

// HandleProfilePanel is the "📊 Player Profile" mother-keyboard entry
// point - plants the ProfileNavigation child keyboard. Every button on
// it fires its own report/panel directly (same leaf-action shape as
// Jobs/Advisors).
func (h *ProfileHandler) HandleProfilePanel(c telebot.Context) error {
	panelText := "📊━━━━━━━━━━━━━━━━━━━━━━📊\n" +
		htmlBold("COMMANDER DASHBOARD") + "\n" +
		"📊━━━━━━━━━━━━━━━━━━━━━━📊\n\n" +
		"Everything about your standing and settings - pick a report below.\n\n" +
		"📈 " + htmlBold("Server Stats") + " - world-wide totals.\n" +
		"🪖 " + htmlBold("My Units") + " - your full garrison breakdown.\n" +
		"📜 " + htmlBold("My Missions") + " - active raids/expeditions at a glance.\n" +
		"🗺️ " + htmlBold("My Destinations") + " - saved raid targets.\n" +
		"📰 " + htmlBold("Event Log") + " - recent sector news.\n" +
		"⚙️ " + htmlBold("Settings") + " - alert toggles, incl. muting routine route-status pings.\n" +
		"📖 " + htmlBold("Player Guide") + " - how to play, from the top.\n\n" +
		"📊━━━━━━━━━━━━━━━━━━━━━━📊"

	return sendPanelWithNavHTML(c, "📊 Loading Commander Dashboard...", keyboards.ProfileNavigation(), panelText, &telebot.ReplyMarkup{})
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
			current = "<i>(none set)</i>"
		} else {
			current = htmlQuote(htmlEscape(current))
		}
		return c.Send(fmt.Sprintf("📝 %s\n%s\n\n%s", htmlBold("YOUR DESCRIPTION"), current, htmlItalic("Usage: /description [text] (max 200 characters)")), telebot.ModeHTML)
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

	var muteRouteStatus bool
	_ = h.DB.QueryRowContext(ctx, "SELECT mute_route_status FROM notification_preferences WHERE user_id = $1", sender.ID).Scan(&muteRouteStatus)

	panelText := fmt.Sprintf(
		"⚙️━━━━━━━━━━━━━━━━━━━━━━⚙️\n"+
			"🎛️ ADVANCED GAMEPLAY SETTINGS 🎛️\n"+
			"⚙️━━━━━━━━━━━━━━━━━━━━━━⚙️\n\n"+
			"🚨 Incoming Raid Alerts: %s\n"+
			"📦 Storage Full Alerts: %s\n"+
			"🛣️ Route Status Pings (peaceful road passes,\n"+
			"   weather clears, convoy arrivals): %s\n"+
			"⚙️━━━━━━━━━━━━━━━━━━━━━━⚙️",
		onOff(notifyRaid), onOff(notifyStorage), onOff(!muteRouteStatus),
	)

	selector := &telebot.ReplyMarkup{}
	btnRaid := selector.Data("🚨 Toggle Raid Alerts", "settings_toggle", "raid")
	btnStorage := selector.Data("📦 Toggle Storage Alerts", "settings_toggle", "storage")
	btnRouteStatus := selector.Data("🛣️ Toggle Route Status Pings", "settings_toggle", "route_status")
	selector.Inline(selector.Row(btnRaid), selector.Row(btnStorage), selector.Row(btnRouteStatus))

	return sendPanelWithNav(c, navCaptionMain, keyboards.MainNavigation(), panelText, selector)
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

	// route_status lives in notification_preferences, not users - see
	// MMO_WORLD_EVOLUTION_PLAN.md Phase 7 milestone 2 / internal/engine/
	// notifications/preferences.go's MutableCategories. This is
	// deliberately the ONLY notification category a player can mute here;
	// combat, discovery, and supply-loss alerts stay on users.notify_on_*
	// or aren't gated at all, by design.
	if setting == "route_status" {
		_, err := h.DB.ExecContext(ctx, `
			INSERT INTO notification_preferences (user_id, mute_route_status, updated_at)
			VALUES ($1, TRUE, CURRENT_TIMESTAMP)
			ON CONFLICT (user_id) DO UPDATE SET mute_route_status = NOT notification_preferences.mute_route_status, updated_at = CURRENT_TIMESTAMP`,
			sender.ID)
		if err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error updating setting."})
		}
		_ = c.Respond(&telebot.CallbackResponse{Text: "✅ Setting updated!"})
		return h.HandleSettings(c)
	}

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

	// Codes are normally set the moment a player picks a faction, but
	// fall back to generating one here (e.g. a player runs /refer before
	// ever finishing onboarding). Same deterministic, collision-free
	// generator either way, so the value is identical regardless of
	// which path sets it first.
	var code string
	err := h.DB.QueryRowContext(ctx, "SELECT COALESCE(referral_code, '') FROM users WHERE telegram_id = $1", sender.ID).Scan(&code)
	if err != nil || code == "" {
		code = generateReferralCode(sender.ID)
		_, _ = h.DB.ExecContext(ctx, "UPDATE users SET referral_code = $1 WHERE telegram_id = $2", code, sender.ID)
	}

	var referralCount, tierClaimed int
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE referred_by = $1", sender.ID).Scan(&referralCount)
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(referral_tier_claimed, 0) FROM users WHERE telegram_id = $1", sender.ID).Scan(&tierClaimed)

	botUsername := ""
	if c.Bot() != nil && c.Bot().Me != nil {
		botUsername = c.Bot().Me.Username
	}
	referralLink := fmt.Sprintf("https://t.me/%s?start=%s", botUsername, code)

	var b strings.Builder
	b.WriteString("🎁━━━━━━━━━━━━━━━━━━━━━━🎁\n")
	b.WriteString("👥 " + ai.HTMLBold("REFER YOUR FRIENDS") + " 👥\n")
	b.WriteString("🎁━━━━━━━━━━━━━━━━━━━━━━🎁\n\n")
	b.WriteString("Send your link below — anyone who taps it and starts the bot is automatically marked as referred by you. No code to type!\n\n")
	fmt.Fprintf(&b, "🔗 Your Referral Link:\n%s\n\n", ai.HTMLCode(ai.HTMLEscape(referralLink)))
	fmt.Fprintf(&b, "🔑 Your Code: %s\n", ai.HTMLCode(ai.HTMLEscape(code)))
	fmt.Fprintf(&b, "👥 Friends Referred: %s\n", ai.HTMLBold(fmt.Sprintf("%d", referralCount)))
	b.WriteString("🎁 Reward per referral: 25,000 of every resource for both of you\n\n")

	milestoneRows := make([][]string, 0, len(referralMilestones))
	for _, m := range referralMilestones {
		status := fmt.Sprintf("%d/%d", referralCount, m.Count)
		if tierClaimed >= m.Count {
			status = "✅ Done"
		}
		milestoneRows = append(milestoneRows, []string{
			fmt.Sprintf("%d", m.Count),
			fmt.Sprintf("+%.0f", m.Metal),
			fmt.Sprintf("+%.0f", m.Crystal),
			fmt.Sprintf("+%.0f", m.Neuro),
			status,
		})
	}
	b.WriteString("🏆 " + ai.HTMLBold("MILESTONE BONUSES") + "\n")
	b.WriteString(ai.HTMLTable([]string{"Refs", "Metal", "🔮", "Neuro", "Status"}, milestoneRows) + "\n\n")

	rows, lbErr := h.DB.QueryContext(ctx, `
		SELECT COALESCE(ref.first_name, 'Commander'), COUNT(*) AS cnt
		FROM users child
		JOIN users ref ON ref.telegram_id = child.referred_by
		GROUP BY ref.telegram_id, ref.first_name
		ORDER BY cnt DESC
		LIMIT 5`)
	if lbErr == nil {
		var topRows [][]string
		rank := 1
		for rows.Next() {
			var name string
			var cnt int
			if scanErr := rows.Scan(&name, &cnt); scanErr == nil {
				// Telegram first names are free-form user input and can
				// legitimately be long - clip the RAW name to a safe
				// rune length before escaping, not after (escaping
				// first and letting htmlTable's own clipping cut the
				// result could slice through an entity like "&amp;"
				// and produce invalid HTML Telegram would reject the
				// whole message for - see ai.HTMLTable's doc comment).
				if r := []rune(name); len(r) > 18 {
					name = string(r[:17]) + "…"
				}
				topRows = append(topRows, []string{
					medalFor(rank) + " " + fmt.Sprintf("%d", rank),
					ai.HTMLEscape(name),
					fmt.Sprintf("%d", cnt),
				})
				rank++
			}
		}
		rows.Close()
		if len(topRows) > 0 {
			b.WriteString("🏅 " + ai.HTMLBold("TOP REFERRERS") + "\n")
			b.WriteString(ai.HTMLTable([]string{"#", "Name", "Refs"}, topRows) + "\n\n")
		}
	}

	b.WriteString("🎁━━━━━━━━━━━━━━━━━━━━━━🎁")

	return c.Send(b.String(), telebot.ModeHTML)
}

// feedbackSenderLabel builds "First Name (@username)" for the admin
// alert, matching clan.go's broadcast label convention. Named
// distinctly from ranking.go's displayName (a different signature, same
// package) to avoid any confusion between the two.
func feedbackSenderLabel(sender *telebot.User) string {
	if sender.Username == "" {
		return sender.FirstName
	}
	return fmt.Sprintf("%s (@%s)", sender.FirstName, sender.Username)
}

// ── /feedback ─────────────────────────────────────────────────────────

// doSubmitFeedback is the testable core shared by the slash command and
// the button-driven pending-input flow below, matching this session's
// established doX convention (doSetTaxRate, doGhostProtocol, etc.):
// stores the submission, then - the actual gap this closes, per
// FEEDBACK_CHANGELOG_NLP_PLAN.md milestone 1 - immediately notifies
// every admin, rather than leaving feedback sitting in a table nobody
// thinks to query. Category "general" (non-mutable): an admin muting
// routine pings should never cost them real player feedback.
func (h *ProfileHandler) doSubmitFeedback(ctx context.Context, userID int64, displayName, message string) error {
	if _, err := h.DB.ExecContext(ctx, "INSERT INTO feedback_submissions (user_id, message) VALUES ($1, $2)", userID, message); err != nil {
		return err
	}

	alert := "📨 " + htmlBold("NEW PLAYER FEEDBACK") + "\n" + divider + "\n" +
		fmt.Sprintf("From: %s\n\n%s", htmlEscape(displayName), htmlEscape(message)) + "\n" + divider
	for _, adminID := range h.AdminIDs {
		_ = notifications.Queue(ctx, h.DB, adminID, alert, "general")
	}
	return nil
}

func (h *ProfileHandler) HandleFeedback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	msg := strings.TrimSpace(c.Message().Payload)
	if msg == "" {
		return c.Send("⚠️ Usage: /feedback [your message]\n\nYour feedback goes straight to the development team. Or tap 💬 Send Feedback to be prompted instead.")
	}

	if err := h.doSubmitFeedback(ctx, sender.ID, feedbackSenderLabel(sender), msg); err != nil {
		return c.Send("⚠️ Error submitting feedback.")
	}
	return c.Send("📨 Feedback received - thank you for helping improve The Vagabond!")
}

// HandleFeedbackButton starts the pending-input flow: the player's next
// text message is captured as their feedback instead of being routed
// through nlp.HandleTextMessage - see HandleFeedbackPendingInput below,
// wired ahead of nlp.HandleTextMessage in main.go exactly like admin.go
// and onboarding.go's own pending-input flows.
func (h *ProfileHandler) HandleFeedbackButton(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	h.feedbackPendingMu.Lock()
	h.feedbackPending[sender.ID] = true
	h.feedbackPendingMu.Unlock()

	return c.Send("💬 "+htmlBold("SEND FEEDBACK")+"\n"+divider+"\n"+
		htmlItalic("What's on your mind? Bug reports, feature ideas, anything - type your message and send it, it goes straight to the development team.")+"\n"+divider,
		telebot.ModeHTML)
}

// HandleFeedbackPendingInput consumes a player's next free-text message
// if (and only if) they're mid-flow from HandleFeedbackButton above.
// Returns handled=false immediately for anyone with nothing pending, so
// normal NLP text parsing continues completely unaffected - see
// main.go's OnText registration for how this chains alongside admin.go
// and onboarding.go's equivalents.
func (h *ProfileHandler) HandleFeedbackPendingInput(c telebot.Context) (handled bool, err error) {
	sender := c.Sender()
	if sender == nil {
		return false, nil
	}

	h.feedbackPendingMu.Lock()
	pending := h.feedbackPending[sender.ID]
	if pending {
		delete(h.feedbackPending, sender.ID)
	}
	h.feedbackPendingMu.Unlock()

	if !pending {
		return false, nil
	}

	msg := strings.TrimSpace(c.Text())
	if msg == "" {
		return true, c.Send("⚠️ Empty message - feedback not submitted. Tap 💬 Send Feedback to try again.")
	}

	ctx := context.Background()
	if err := h.doSubmitFeedback(ctx, sender.ID, feedbackSenderLabel(sender), msg); err != nil {
		return true, c.Send("⚠️ Error submitting feedback.")
	}
	return true, c.Send("📨 Feedback received - thank you for helping improve The Vagabond!")
}

// feedbackInboxPageSize matches changelog's "at least 5" convention
// from FEEDBACK_CHANGELOG_NLP_PLAN.md, applied here too for the same
// reason: a short, skimmable page rather than a wall of history.
const feedbackInboxPageSize = 5

// HandleFeedbackInbox (/feedback_inbox, admin-only) lists the most
// recent submissions - so a submission is never visible only in a chat
// log if every admin happened to be offline when it was delivered.
func (h *ProfileHandler) HandleFeedbackInbox(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil || !h.IsAdmin(sender.ID) {
		return c.Send("⛔ Admin access required.")
	}

	ctx := context.Background()
	rows, err := h.DB.QueryContext(ctx, `
		SELECT COALESCE(u.first_name, ''), COALESCE(u.username, ''), f.message, f.created_at
		FROM feedback_submissions f
		JOIN users u ON u.telegram_id = f.user_id
		ORDER BY f.created_at DESC
		LIMIT $1`, feedbackInboxPageSize)
	if err != nil {
		return c.Send("⚠️ Error loading feedback inbox.")
	}
	defer rows.Close()

	text := "📨 " + htmlBold("FEEDBACK INBOX") + "\n" + divider + "\n"
	any := false
	for rows.Next() {
		var fName, username, message string
		var createdAt time.Time
		if scanErr := rows.Scan(&fName, &username, &message, &createdAt); scanErr == nil {
			any = true
			who := htmlEscape(fName)
			if username != "" {
				who += " (@" + htmlEscape(username) + ")"
			}
			text += fmt.Sprintf("\n👤 %s — %s\n%s\n", who, htmlCode(createdAt.UTC().Format("Jan 2 15:04")), htmlEscape(message))
		}
	}
	if !any {
		text += "\n" + htmlItalic("No feedback submitted yet.")
	}
	text += "\n" + divider
	return c.Send(text, telebot.ModeHTML)
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

	alertMsg := fmt.Sprintf("💬 %s %s:\n\n%s", htmlBold("MESSAGE from"), htmlCode(htmlEscape(sender.FirstName)), htmlEscape(messageText))
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
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users WHERE state != 'ai_faction'").Scan(&totalPlayers)
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM clans").Scan(&totalClans)
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM federations").Scan(&totalFederations)
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM raids").Scan(&totalRaids)

	var totalMetal, totalCrystal, totalScrap float64
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(SUM(metal),0), COALESCE(SUM(crystal),0), COALESCE(SUM(scrap),0) FROM resources").Scan(&totalMetal, &totalCrystal, &totalScrap)

	panelText := fmt.Sprintf(
		"🌍 %s 🌍\n"+divider+"\n\n"+
			"👥 Total Survivors: %s\n"+
			"🏴 Total Clans: %s\n"+
			"🌐 Total Federations: %s\n"+
			"⚔️ Total Raids Launched: %s\n\n"+
			"%s\n"+
			"🔩 Metal in circulation: %s\n"+
			"🔮 Crystal in circulation: %s\n"+
			"⚙️ Scrap in circulation: %s\n"+divider,
		htmlBold("GLOBAL WASTELAND STATISTICS"),
		htmlCode(fmt.Sprintf("%d", totalPlayers)),
		htmlCode(fmt.Sprintf("%d", totalClans)),
		htmlCode(fmt.Sprintf("%d", totalFederations)),
		htmlCode(fmt.Sprintf("%d", totalRaids)),
		htmlBold("🌎 ECONOMY-WIDE TOTALS"),
		htmlCode(fmt.Sprintf("%.0f", totalMetal)),
		htmlCode(fmt.Sprintf("%.0f", totalCrystal)),
		htmlCode(fmt.Sprintf("%.0f", totalScrap)),
	)

	return c.Send(panelText, telebot.ModeHTML)
}

// ── /missions ─────────────────────────────────────────────────────────

// HandleMissions consolidates every active operation the player has in
// flight: raids, World Boss engagements, and active mining queues.
func (h *ProfileHandler) HandleMissions(c telebot.Context) error {
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

	panelText := "🚀 " + htmlBold("YOUR ACTIVE MISSIONS") + " 🚀\n" + divider + "\n\n"

	any := false

	raidRows, err := h.DB.QueryContext(ctx, `
		SELECT COALESCE(ed.name, 'Rogue Drone Nest'), r.state, r.resolve_time
		FROM raids r
		LEFT JOIN encampments ed ON ed.id = r.defender_id
		WHERE r.attacker_id = $1 AND r.state IN ('marching', 'engaged', 'returning')
		ORDER BY r.resolve_time ASC`, campID)
	if err == nil {
		for raidRows.Next() {
			var target, state string
			var resolveTime interface{}
			if raidRows.Scan(&target, &state, &resolveTime) == nil {
				any = true
				panelText += fmt.Sprintf("⚔️ Raid ➜ %s %s\n", htmlEscape(target), htmlCode("["+state+"]"))
			}
		}
		raidRows.Close()
	}

	bossRows, err := h.DB.QueryContext(ctx, `
		SELECT b.name, a.state
		FROM world_boss_attacks a
		JOIN world_bosses b ON b.id = a.boss_id
		WHERE a.encampment_id = $1`, campID)
	if err == nil {
		for bossRows.Next() {
			var bossName, state string
			if bossRows.Scan(&bossName, &state) == nil {
				any = true
				panelText += fmt.Sprintf("👹 Boss Strike ➜ %s %s\n", htmlEscape(bossName), htmlCode("["+state+"]"))
			}
		}
		bossRows.Close()
	}

	miningRows, err := h.DB.QueryContext(ctx, `
		SELECT resource_type, miners_assigned, ready_at 
		FROM active_mining_queues 
		WHERE encampment_id = $1 AND is_completed = FALSE`, campID)
	if err == nil {
		for miningRows.Next() {
			var resType string
			var miners int
			var readyAt interface{}
			if miningRows.Scan(&resType, &miners, &readyAt) == nil {
				any = true
				panelText += fmt.Sprintf("⛏️ Mining %s %s\n", htmlEscape(resType), htmlCode(fmt.Sprintf("(%d miners)", miners)))
			}
		}
		miningRows.Close()
	}

	if !any {
		panelText += htmlItalic("No active missions. Launch a raid, attack a boss, or start mining!") + "\n"
	}

	panelText += divider
	return c.Send(panelText, telebot.ModeHTML)
}

// ── /destinations ─────────────────────────────────────────────────────

// HandleDestinations lists every rival outpost the player has previously
// scouted (via /scout or /autoscan), plus the always-available Rogue
// Drone Nest - matching SpaceHunt's 'map of discovered planets', where
// only previously-discovered targets show up.
func (h *ProfileHandler) HandleDestinations(c telebot.Context) error {
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

	panelText := "🗺️━━━━━━━━━━━━━━━━━━━━━━🗺️\n" +
		"🌍 DISCOVERED DESTINATIONS 🌍\n" +
		"🗺️━━━━━━━━━━━━━━━━━━━━━━🗺️\n\n" +
		"🤖 [AI] Rogue Drone Nest - always available (use /recon_ai to scout)\n\n" +
		"👁️ PREVIOUSLY SCOUTED RIVALS:\n"

	rows, err := h.DB.QueryContext(ctx, `
		SELECT DISTINCT ed.name, c.x, c.y
		FROM spy_missions sm
		JOIN encampments ed ON ed.id = sm.target_id
		JOIN coordinates c ON c.id = ed.coordinate_id
		WHERE sm.spy_id = $1
		LIMIT 15`, campID)
	if err == nil {
		any := false
		for rows.Next() {
			var name string
			var x, y int
			if rows.Scan(&name, &x, &y) == nil {
				any = true
				panelText += fmt.Sprintf("📍 %s [%d, %d]\n", name, x, y)
			}
		}
		rows.Close()
		if !any {
			panelText += "None yet - use /scout [username] to discover rival outposts!\n"
		}
	}

	panelText += "🗺️━━━━━━━━━━━━━━━━━━━━━━🗺️"
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
	var liberators, wraiths, observers, guardians, piercingMissiles, cargoMk1, cargoMk2, cargoMk3 int
	query := `SELECT COALESCE(soldiers,0), COALESCE(drones,0), COALESCE(mechs,0), COALESCE(nukes,0), 
	          COALESCE(buggies,0), COALESCE(ships,0), COALESCE(jets,0), 
	          COALESCE(haulers,0), COALESCE(tankers,0), COALESCE(rigs,0),
	          COALESCE(destroyers,0), COALESCE(bombers,0), COALESCE(scouts,0), COALESCE(battlecruisers,0), COALESCE(deathstars,0),
	          COALESCE(liberators,0), COALESCE(wraiths,0), COALESCE(observers,0), COALESCE(guardians,0), COALESCE(piercing_missiles,0),
	          COALESCE(cargo_mk1,0), COALESCE(cargo_mk2,0), COALESCE(cargo_mk3,0)
	          FROM workshop_inventory WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, query, campID).Scan(&soldiers, &drones, &mechs, &nukes, &buggies, &ships, &jets, &haulers, &tankers, &rigs, &destroyers, &bombers, &scouts, &battlecruisers, &deathstars,
		&liberators, &wraiths, &observers, &guardians, &piercingMissiles, &cargoMk1, &cargoMk2, &cargoMk3)

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
			"🦅 Liberators: %d | 👻 Wraiths: %d\n"+
			"👁️ Observers: %d | 🛡️🤖 Guardians: %d\n"+
			"🎯☢️ Piercing Missiles: %d\n"+
			"🚗 Buggies: %d | ⛵ Ships: %d | ✈️ Jets: %d\n"+
			"🚛 Haulers: %d | 🛡️ Tankers: %d | 🔧 Rigs: %d\n"+
			"🚚 Cargo Mk I: %d | 🚚🚚 Mk II: %d | 🚚🚚🚚 Mk III: %d\n\n"+
			"🚀 ON ACTIVE MISSIONS: %d fleet(s) deployed (check /missions)\n"+
			"🪖━━━━━━━━━━━━━━━━━━━━━━🪖",
		soldiers, drones, mechs, nukes, destroyers, bombers, scouts, battlecruisers, deathstars,
		liberators, wraiths, observers, guardians, piercingMissiles,
		buggies, ships, jets, haulers, tankers, rigs, cargoMk1, cargoMk2, cargoMk3, marchingCount,
	)

	return c.Send(panelText)
}
