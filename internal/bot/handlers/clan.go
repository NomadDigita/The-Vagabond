package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/scoring"
	"gopkg.in/telebot.v3"
)

type ClanHandler struct {
	DB *sql.DB
}

func NewClanHandler(db *sql.DB) *ClanHandler {
	return &ClanHandler{DB: db}
}

const clanRenameCost = 800.0 // Crystal

// HandleClanPanel renders the player's own clan HUD, or the "unaligned" screen.
func (h *ClanHandler) HandleClanPanel(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	ctx := context.Background()

	var campID string
	var campLvl int
	err := h.DB.QueryRowContext(ctx, "SELECT id, level FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID, &campLvl)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.MainNavigation())
	}

	if campLvl < 5 {
		return c.Send("❌ Alliance Access Locked: Reach Outpost Core Level 5 to access Clan Networks.", keyboards.MainNavigation())
	}

	var clanID sql.NullString
	var clanName sql.NullString
	var role sql.NullString

	queryUserClan := `
		SELECT c.id, c.name, uc.role 
		FROM user_clans uc
		JOIN clans c ON c.id = uc.clan_id
		WHERE uc.user_id = $1`

	err = h.DB.QueryRowContext(ctx, queryUserClan, sender.ID).Scan(&clanID, &clanName, &role)

	selector := &telebot.ReplyMarkup{}

	if errors.Is(err, sql.ErrNoRows) || !clanID.Valid {
		var pendingCount int
		_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM clan_applications WHERE user_id = $1 AND status = 'pending'", sender.ID).Scan(&pendingCount)

		panelText := "🛡️━━━━━━━━━━━━━━━━━━━━━━🛡️\n" +
			"🏴 SECTOR ALLIANCE NETWORK 🏴\n" +
			"🛡️━━━━━━━━━━━━━━━━━━━━━━🛡️\n\n" +
			"Clans unite up to 15 commanders under joint defense grids and war decks.\n\n" +
			"You are currently unaligned.\n"

		if pendingCount > 0 {
			panelText += fmt.Sprintf("\n⏳ You have %d pending application(s) awaiting Leader approval.\n", pendingCount)
		}

		panelText += "\n🔍 Browse existing clans with /clans, or found your own:\n" +
			"⚒️ /clan_create [name]\n" +
			"🛡️━━━━━━━━━━━━━━━━━━━━━━🛡️"

		btnBrowse := selector.Data("🔍 Browse Clans", "browse_clans", "0")
		selector.Inline(selector.Row(btnBrowse))

		return c.Send(panelText, selector, keyboards.EconomyNavigation())
	}

	var members int
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_clans WHERE clan_id = $1", clanID.String).Scan(&members)

	var pendingApps int
	if role.String == "Leader" {
		_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM clan_applications WHERE clan_id = $1 AND status = 'pending'", clanID.String).Scan(&pendingApps)
	}

	var activeWarOpponent string
	var warScoreMine, warScoreTheirs float64
	var inWar bool
	warQuery := `
		SELECT CASE WHEN w.clan_a_id = $1 THEN cb.name ELSE ca.name END,
		       CASE WHEN w.clan_a_id = $1 THEN w.score_a ELSE w.score_b END,
		       CASE WHEN w.clan_a_id = $1 THEN w.score_b ELSE w.score_a END
		FROM clan_wars w
		JOIN clans ca ON ca.id = w.clan_a_id
		JOIN clans cb ON cb.id = w.clan_b_id
		WHERE (w.clan_a_id = $1 OR w.clan_b_id = $1) AND w.status = 'active'
		LIMIT 1`
	err = h.DB.QueryRowContext(ctx, warQuery, clanID.String).Scan(&activeWarOpponent, &warScoreMine, &warScoreTheirs)
	if err == nil {
		inWar = true
	}

	panelText := fmt.Sprintf(
		"🛡️━━━━━━━━━━━━━━━━━━━━━━🛡️\n"+
			"🏴 CLAN HUD: %s 🏴\n"+
			"🛡️━━━━━━━━━━━━━━━━━━━━━━🛡️\n"+
			"👥 Commanders Enlisted: %d / 15\n"+
			"🎖️ Your Rank: %s\n",
		clanName.String, members, role.String,
	)

	if inWar {
		panelText += fmt.Sprintf("\n⚔️ AT WAR with %s!\n📊 Score: %.0f - %.0f\n", activeWarOpponent, warScoreMine, warScoreTheirs)
	}
	if pendingApps > 0 {
		panelText += fmt.Sprintf("\n📬 %d pending application(s) - review them below!\n", pendingApps)
	}
	panelText += "🛡️━━━━━━━━━━━━━━━━━━━━━━🛡️"

	var buttons []telebot.Row

	btnManage := selector.Data("👥 Manage Members", "clan_manage", clanID.String)
	btnStats := selector.Data("📊 Alliance Stats", "clan_stats", clanID.String)
	buttons = append(buttons, selector.Row(btnManage, btnStats))

	if role.String == "Leader" {
		btnApps := selector.Data(fmt.Sprintf("📬 Applications (%d)", pendingApps), "clan_apps", clanID.String)
		buttons = append(buttons, selector.Row(btnApps))

		if !inWar {
			btnDeclare := selector.Data("⚔️ Declare War", "declare_clan_war", clanID.String)
			buttons = append(buttons, selector.Row(btnDeclare))
		}
	}
	btnLeave := selector.Data("🚪 Leave Clan", "leave_clan", clanID.String)
	buttons = append(buttons, selector.Row(btnLeave))

	selector.Inline(buttons...)
	return c.Send(panelText, selector, keyboards.EconomyNavigation())
}

// HandleBrowseClansCallback / HandleBrowseClansCommand lists all clans,
// ranked by combined member score, each with an Apply button.
func (h *ClanHandler) HandleBrowseClans(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	ctx := context.Background()

	panelText := "🔍━━━━━━━━━━━━━━━━━━━━━━🔍\n" +
		"🏴 BROWSE CLANS 🏴\n" +
		"🔍━━━━━━━━━━━━━━━━━━━━━━🔍\n\n"

	query := fmt.Sprintf(`
		SELECT c.id, c.name, COUNT(uc.user_id) as members, COALESCE(SUM(%s), 0) as total_score
		FROM clans c
		LEFT JOIN user_clans uc ON uc.clan_id = c.id
		LEFT JOIN encampments e ON e.user_id = uc.user_id
		GROUP BY c.id, c.name
		ORDER BY total_score DESC
		LIMIT 15`, scoring.ScoreExpr)

	rows, err := h.DB.QueryContext(ctx, query)
	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	if err == nil {
		rank := 1
		any := false
		for rows.Next() {
			var clanID, name string
			var members int
			var score float64
			if scanErr := rows.Scan(&clanID, &name, &members, &score); scanErr == nil {
				any = true
				panelText += fmt.Sprintf("%s %d. 🏴 %s (%d/15) — 🏅 %.0f pts\n", medalFor(rank), rank, name, members, score)
				if members < 15 {
					btn := selector.Data(fmt.Sprintf("📨 Apply to %s", name), "clan_apply", clanID)
					buttons = append(buttons, selector.Row(btn))
				}
				rank++
			}
		}
		rows.Close()
		if !any {
			panelText += "No Clans exist yet. Found the first one with /clan_create [name]!\n"
		}
	}

	panelText += "\n🔍━━━━━━━━━━━━━━━━━━━━━━🔍"
	selector.Inline(buttons...)
	return c.Send(panelText, selector, keyboards.EconomyNavigation())
}

// HandleApplyToClanCallback sends a join application, notifying the Leader.
func (h *ClanHandler) HandleApplyToClanCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	clanID := c.Args()[0]

	var alreadyIn bool
	_ = h.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM user_clans WHERE user_id = $1)", sender.ID).Scan(&alreadyIn)
	if alreadyIn {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ You're already in a Clan. Leave it first."})
	}

	var clanName string
	var leaderID int64
	err := h.DB.QueryRowContext(ctx, "SELECT name, leader_id FROM clans WHERE id = $1", clanID).Scan(&clanName, &leaderID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ That Clan no longer exists."})
	}

	_, err = h.DB.ExecContext(ctx, "INSERT INTO clan_applications (clan_id, user_id, status) VALUES ($1, $2, 'pending') ON CONFLICT (clan_id, user_id) DO UPDATE SET status = 'pending'", clanID, sender.ID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error submitting application."})
	}

	senderName := sender.FirstName
	alertMsg := fmt.Sprintf("📬 NEW CLAN APPLICATION: %s wants to join %s! Review it via /clan.", senderName, clanName)
	_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", leaderID, alertMsg)

	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("📨 Application sent to %s! Awaiting Leader approval.", clanName)})
	return nil
}

// HandleApplicationsInboxCallback shows the Leader their pending applications.
func (h *ClanHandler) HandleApplicationsInboxCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	clanID := c.Args()[0]

	var role string
	_ = h.DB.QueryRowContext(ctx, "SELECT role FROM user_clans WHERE user_id = $1 AND clan_id = $2", sender.ID, clanID).Scan(&role)
	if role != "Leader" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Access Denied: Leaders only."})
	}

	rows, err := h.DB.QueryContext(ctx, `
		SELECT ca.user_id, u.first_name, u.username 
		FROM clan_applications ca
		JOIN users u ON u.telegram_id = ca.user_id
		WHERE ca.clan_id = $1 AND ca.status = 'pending'`, clanID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to load applications."})
	}
	defer rows.Close()

	panelText := "📬━━━━━━━━━━━━━━━━━━━━━━📬\n" +
		"PENDING CLAN APPLICATIONS\n" +
		"📬━━━━━━━━━━━━━━━━━━━━━━📬\n\n"

	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row
	any := false

	for rows.Next() {
		var userID int64
		var fName, username string
		if scanErr := rows.Scan(&userID, &fName, &username); scanErr == nil {
			any = true
			panelText += fmt.Sprintf("👤 %s (@%s)\n", fName, username)
			btnAccept := selector.Data("✅ Accept", "clan_app_accept", strconv.FormatInt(userID, 10), clanID)
			btnReject := selector.Data("❌ Reject", "clan_app_reject", strconv.FormatInt(userID, 10), clanID)
			buttons = append(buttons, selector.Row(btnAccept, btnReject))
		}
	}

	if !any {
		panelText += "No pending applications.\n"
	}
	panelText += "📬━━━━━━━━━━━━━━━━━━━━━━📬"
	selector.Inline(buttons...)
	return c.Send(panelText, selector)
}

// HandleApplicationDecisionCallback processes Accept/Reject on an application.
func (h *ClanHandler) HandleApplicationDecisionCallback(c telebot.Context, accept bool) error {
	ctx := context.Background()
	sender := c.Sender()
	args := c.Args()
	if len(args) < 2 {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Invalid application reference."})
	}
	targetID, _ := strconv.ParseInt(args[0], 10, 64)
	clanID := args[1]

	var role string
	_ = h.DB.QueryRowContext(ctx, "SELECT role FROM user_clans WHERE user_id = $1 AND clan_id = $2", sender.ID, clanID).Scan(&role)
	if role != "Leader" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Access Denied: Leaders only."})
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Action failed."})
	}
	defer tx.Rollback()

	if accept {
		var members int
		_ = tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_clans WHERE clan_id = $1", clanID).Scan(&members)
		if members >= 15 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Clan Full: 15/15 members already."})
		}

		var alreadyIn bool
		_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM user_clans WHERE user_id = $1)", targetID).Scan(&alreadyIn)
		if alreadyIn {
			_, _ = tx.ExecContext(ctx, "DELETE FROM clan_applications WHERE clan_id = $1 AND user_id = $2", clanID, targetID)
			_ = tx.Commit()
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ That applicant already joined another Clan."})
		}

		_, _ = tx.ExecContext(ctx, "INSERT INTO user_clans (user_id, clan_id, role) VALUES ($1, $2, 'Soldier')", targetID, clanID)
		_, _ = tx.ExecContext(ctx, "DELETE FROM clan_applications WHERE clan_id = $1 AND user_id = $2", clanID, targetID)
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", targetID, "✅ APPLICATION ACCEPTED: Welcome to your new Clan!")
		_ = c.Respond(&telebot.CallbackResponse{Text: "✅ Applicant accepted!"})
	} else {
		_, _ = tx.ExecContext(ctx, "UPDATE clan_applications SET status = 'rejected' WHERE clan_id = $1 AND user_id = $2", clanID, targetID)
		_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", targetID, "❌ Your Clan application was declined.")
		_ = c.Respond(&telebot.CallbackResponse{Text: "❌ Applicant rejected."})
	}

	_ = tx.Commit()
	return h.HandleApplicationsInboxCallback(c)
}

func (h *ClanHandler) HandleAcceptApplicationCallback(c telebot.Context) error {
	return h.HandleApplicationDecisionCallback(c, true)
}

func (h *ClanHandler) HandleRejectApplicationCallback(c telebot.Context) error {
	return h.HandleApplicationDecisionCallback(c, false)
}

// HandleManageMembersCallback renders the roster management page with Promote & Kick inline buttons
func (h *ClanHandler) HandleManageMembersCallback(c telebot.Context) error {
	ctx := context.Background()
	clanID := c.Args()[0]
	sender := c.Sender()

	var senderRole string
	_ = h.DB.QueryRowContext(ctx, "SELECT role FROM user_clans WHERE user_id = $1 AND clan_id = $2", sender.ID, clanID).Scan(&senderRole)

	rows, err := h.DB.QueryContext(ctx, "SELECT u.telegram_id, u.first_name, u.username, uc.role FROM user_clans uc JOIN users u ON u.telegram_id = uc.user_id WHERE uc.clan_id = $1", clanID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Failed to fetch roster."})
	}
	defer rows.Close()

	rosterText := "👥━━━━━━━━━━━━━━━━━━━━━━👥\n" +
		"ALLIANCE ROSTER LISTING\n" +
		"👥━━━━━━━━━━━━━━━━━━━━━━👥\n" +
		"Manage your allied commanders below:\n\n"

	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	index := 1
	for rows.Next() {
		var memberID int64
		var fName, username, role string
		if err := rows.Scan(&memberID, &fName, &username, &role); err == nil {
			rosterText += fmt.Sprintf("[%d] %s (@%s)\n    Rank: %s\n\n", index, fName, username, role)

			if senderRole == "Leader" && memberID != sender.ID {
				btnKick := selector.Data(fmt.Sprintf("❌ Kick [%d]", index), "clan_kick", strconv.FormatInt(memberID, 10))
				btnPromote := selector.Data(fmt.Sprintf("🛡️ Promote [%d]", index), "clan_promote", strconv.FormatInt(memberID, 10))
				buttons = append(buttons, selector.Row(btnPromote, btnKick))
			}
			index++
		}
	}

	rosterText += "👥━━━━━━━━━━━━━━━━━━━━━━👥"
	selector.Inline(buttons...)
	return c.Send(rosterText, selector)
}

// HandleKickMemberCallback processes kicking members from the alliance
func (h *ClanHandler) HandleKickMemberCallback(c telebot.Context) error {
	ctx := context.Background()
	targetIDStr := c.Args()[0]
	targetID, _ := strconv.ParseInt(targetIDStr, 10, 64)
	sender := c.Sender()

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Action failed."})
	}
	defer tx.Rollback()

	var leaderRole string
	_ = tx.QueryRowContext(ctx, "SELECT role FROM user_clans WHERE user_id = $1", sender.ID).Scan(&leaderRole)
	if leaderRole != "Leader" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Access Denied: Leaders only."})
	}

	_, _ = tx.ExecContext(ctx, "DELETE FROM user_clans WHERE user_id = $1", targetID)

	alertMsg := "🚪 ALLIANCE NOTICE: You have been removed from the alliance roster by the Clan Leader."
	_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", targetID, alertMsg)

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: "❌ Allied commander removed."})
	return h.HandleClanPanel(c)
}

// HandlePromoteMemberCallback processes promoting members to Co-Leaders
func (h *ClanHandler) HandlePromoteMemberCallback(c telebot.Context) error {
	ctx := context.Background()
	targetIDStr := c.Args()[0]
	targetID, _ := strconv.ParseInt(targetIDStr, 10, 64)
	sender := c.Sender()

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Action failed."})
	}
	defer tx.Rollback()

	var leaderRole string
	_ = tx.QueryRowContext(ctx, "SELECT role FROM user_clans WHERE user_id = $1", sender.ID).Scan(&leaderRole)
	if leaderRole != "Leader" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Access Denied: Leaders only."})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE user_clans SET role = 'Co-Leader' WHERE user_id = $1", targetID)

	alertMsg := "🛡️ CONGRATULATIONS: You have been promoted to Co-Leader within your alliance!"
	_, _ = tx.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", targetID, alertMsg)

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: "🛡️ Member promoted to Co-Leader!"})
	return h.HandleClanPanel(c)
}

// HandleAllianceStatsCallback calculates the accumulated strength metrics
func (h *ClanHandler) HandleAllianceStatsCallback(c telebot.Context) error {
	ctx := context.Background()
	clanID := c.Args()[0]

	var totalLevel int
	var totalSoldiers int
	var totalMechs int

	queryStats := `
		SELECT COALESCE(SUM(e.level), 0), COALESCE(SUM(w.soldiers), 0), COALESCE(SUM(w.mechs), 0)
		FROM user_clans uc
		JOIN encampments e ON e.user_id = uc.user_id
		JOIN workshop_inventory w ON w.encampment_id = e.id
		WHERE uc.clan_id = $1`

	err := h.DB.QueryRowContext(ctx, queryStats, clanID).Scan(&totalLevel, &totalSoldiers, &totalMechs)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error loading statistics."})
	}

	alliancePower := (totalSoldiers * 10) + (totalMechs * 150)

	report := fmt.Sprintf(
		"📊━━━━━━━━━━━━━━━━━━━━━━📊\n"+
			"ALLIANCE STRENGTH SUMMARY\n"+
			"📊━━━━━━━━━━━━━━━━━━━━━━📊\n\n"+
			"🏆 Collective Outpost Level: Level %d\n"+
			"⚔️ Accumulated Military Power: %d Power Rating\n\n"+
			"MILITARY ASSET STOCKPILES:\n"+
			"🪖 Combined Infantry: %d Soldiers\n"+
			"🤖 Combined Armored Core: %d Mechs\n"+
			"📊━━━━━━━━━━━━━━━━━━━━━━📊",
		totalLevel, alliancePower, totalSoldiers, totalMechs,
	)

	return c.Send(report, keyboards.EconomyNavigation())
}

// randomAnimalIcons is the pool /guild_icon draws from, matching
// SpaceHunt's "change your guild icon to another random animal" feature.
var randomAnimalIcons = []string{"🦅", "🐺", "🐻", "🦁", "🐯", "🦂", "🐍", "🦇", "🦉", "🐗", "🦖", "🦍", "🐉", "🦌", "🦊"}

// HandleGuildMissions (/guild_missions) shows recent raids and transfers
// involving any member of the caller's clan.
func (h *ClanHandler) HandleGuildMissions(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	var clanID, clanName string
	err := h.DB.QueryRowContext(ctx, "SELECT c.id, c.name FROM clans c JOIN user_clans uc ON uc.clan_id = c.id WHERE uc.user_id = $1", sender.ID).Scan(&clanID, &clanName)
	if err != nil {
		return c.Send("⚠️ You're not in a Clan. Use /clans to browse or /clan_create [name] to found one.")
	}

	panelText := fmt.Sprintf(
		"📜━━━━━━━━━━━━━━━━━━━━━━📜\n"+
			"🏴 %s: RAIDS & TRANSFERS 🏴\n"+
			"📜━━━━━━━━━━━━━━━━━━━━━━📜\n\n",
		clanName,
	)

	rows, err := h.DB.QueryContext(ctx, `
		SELECT ea.name, COALESCE(ed.name, 'Rogue Drone Nest'), r.state, r.stolen_scrap, r.stolen_metal, r.stolen_crystal
		FROM raids r
		JOIN encampments ea ON ea.id = r.attacker_id
		LEFT JOIN encampments ed ON ed.id = r.defender_id
		WHERE ea.user_id IN (SELECT user_id FROM user_clans WHERE clan_id = $1)
		   OR ed.user_id IN (SELECT user_id FROM user_clans WHERE clan_id = $1)
		ORDER BY r.id DESC
		LIMIT 12`, clanID)
	if err == nil {
		any := false
		for rows.Next() {
			var attName, defName, state string
			var stolenScrap, stolenMetal, stolenCrystal float64
			if scanErr := rows.Scan(&attName, &defName, &state, &stolenScrap, &stolenMetal, &stolenCrystal); scanErr == nil {
				any = true
				panelText += fmt.Sprintf("⚔️ %s ➜ %s [%s]\n   Loot: ⚙️%.0f 🔩%.0f 💎%.0f\n\n", attName, defName, state, stolenScrap, stolenMetal, stolenCrystal)
			}
		}
		rows.Close()
		if !any {
			panelText += "No recent Clan activity.\n"
		}
	}

	panelText += "📜━━━━━━━━━━━━━━━━━━━━━━📜"
	return c.Send(panelText)
}

// HandleGuildMsg (/guildmsg [message]) broadcasts to every clan member.
func (h *ClanHandler) HandleGuildMsg(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	msg := c.Message().Payload
	if msg == "" {
		return c.Send("⚠️ Usage: /guildmsg [message]")
	}

	var clanID, clanName string
	err := h.DB.QueryRowContext(ctx, "SELECT c.id, c.name FROM clans c JOIN user_clans uc ON uc.clan_id = c.id WHERE uc.user_id = $1", sender.ID).Scan(&clanID, &clanName)
	if err != nil {
		return c.Send("⚠️ You're not in a Clan.")
	}

	broadcast := fmt.Sprintf("📢 %s [%s]:\n\n%s", clanName, sender.FirstName, msg)

	rows, err := h.DB.QueryContext(ctx, "SELECT user_id FROM user_clans WHERE clan_id = $1 AND user_id != $2", clanID, sender.ID)
	if err != nil {
		return c.Send("⚠️ Error broadcasting message.")
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var uid int64
		if rows.Scan(&uid) == nil {
			_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", uid, broadcast)
			count++
		}
	}

	return c.Send(fmt.Sprintf("📢 Message broadcast to %d Clan member(s)!", count))
}

// HandleGuildIcon (/guild_icon) randomly changes the clan's icon.
func (h *ClanHandler) HandleGuildIcon(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	var clanID, role string
	err := h.DB.QueryRowContext(ctx, "SELECT c.id, uc.role FROM clans c JOIN user_clans uc ON uc.clan_id = c.id WHERE uc.user_id = $1", sender.ID).Scan(&clanID, &role)
	if err != nil {
		return c.Send("⚠️ You're not in a Clan.")
	}
	if role != "Leader" {
		return c.Send("❌ Access Denied: Only the Clan Leader can change the icon.")
	}

	newIcon := randomAnimalIcons[int(sender.ID+time.Now().Unix())%len(randomAnimalIcons)]
	_, err = h.DB.ExecContext(ctx, "UPDATE clans SET icon = $1 WHERE id = $2", newIcon, clanID)
	if err != nil {
		return c.Send("⚠️ Error updating icon.")
	}

	return c.Send(fmt.Sprintf("%s Your Clan's icon is now %s!", newIcon, newIcon))
}

// HandleGuildDescription (/guild_description [text]) sets the clan's
// description, shown on the recruitment /board.
func (h *ClanHandler) HandleGuildDescription(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	desc := c.Message().Payload
	if desc == "" {
		return c.Send("⚠️ Usage: /guild_description [text] (max 200 characters)")
	}
	if len(desc) > 200 {
		return c.Send("❌ Too Long: Max 200 characters.")
	}

	var clanID, role string
	err := h.DB.QueryRowContext(ctx, "SELECT c.id, uc.role FROM clans c JOIN user_clans uc ON uc.clan_id = c.id WHERE uc.user_id = $1", sender.ID).Scan(&clanID, &role)
	if err != nil {
		return c.Send("⚠️ You're not in a Clan.")
	}
	if role != "Leader" {
		return c.Send("❌ Access Denied: Only the Clan Leader can set the description.")
	}

	_, err = h.DB.ExecContext(ctx, "UPDATE clans SET description = $1 WHERE id = $2", desc, clanID)
	if err != nil {
		return c.Send("⚠️ Error updating description.")
	}

	return c.Send("✅ Clan description updated!")
}

// HandleBoard (/board) is the recruitment post board - clans currently
// open to recruiting, with their icon and description, distinct from
// /clans (which is the pure ranking list).
func (h *ClanHandler) HandleBoard(c telebot.Context) error {
	ctx := context.Background()

	panelText := "📋━━━━━━━━━━━━━━━━━━━━━━📋\n" +
		"🏴 CLAN RECRUITMENT BOARD 🏴\n" +
		"📋━━━━━━━━━━━━━━━━━━━━━━📋\n\n"

	rows, err := h.DB.QueryContext(ctx, `
		SELECT c.id, c.name, c.icon, c.description, COUNT(uc.user_id) as members
		FROM clans c
		LEFT JOIN user_clans uc ON uc.clan_id = c.id
		WHERE c.recruiting = TRUE
		GROUP BY c.id, c.name, c.icon, c.description
		HAVING COUNT(uc.user_id) < 15
		ORDER BY RANDOM()
		LIMIT 10`)

	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	if err == nil {
		any := false
		for rows.Next() {
			var clanID, name, icon, desc string
			var members int
			if scanErr := rows.Scan(&clanID, &name, &icon, &desc, &members); scanErr == nil {
				any = true
				if desc == "" {
					desc = "(no description set)"
				}
				panelText += fmt.Sprintf("%s %s (%d/15)\n📜 %s\n\n", icon, name, members, desc)
				btn := selector.Data(fmt.Sprintf("📨 Apply to %s", name), "clan_apply", clanID)
				buttons = append(buttons, selector.Row(btn))
			}
		}
		rows.Close()
		if !any {
			panelText += "No Clans are actively recruiting right now. Check /clans for the full list.\n"
		}
	}

	panelText += "\n📋━━━━━━━━━━━━━━━━━━━━━━📋"
	selector.Inline(buttons...)
	return c.Send(panelText, selector, keyboards.EconomyNavigation())
}

// HandleCreateClanCommand establishes a clan with a REAL custom name
// (payload), matching SpaceHunt's naming freedom instead of an
// auto-generated placeholder.
func (h *ClanHandler) HandleCreateClanCommand(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	name := c.Message().Payload
	if name == "" {
		return c.Send("⚠️ Usage: /clan_create [name]\n📏 3-24 characters.")
	}
	if len(name) < 3 || len(name) > 24 {
		return c.Send("❌ Invalid Length: Clan name must be 3-24 characters.")
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ Alliance transaction failed.")
	}
	defer tx.Rollback()

	var exists bool
	_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM user_clans WHERE user_id = $1)", sender.ID).Scan(&exists)
	if exists {
		return c.Send("❌ Already in an active Clan! Leave it first with /clan.")
	}

	var clanID string
	err = tx.QueryRowContext(ctx, "INSERT INTO clans (name, leader_id) VALUES ($1, $2) RETURNING id", name, sender.ID).Scan(&clanID)
	if err != nil {
		return c.Send("❌ Name Taken: A Clan with that name already exists.")
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO user_clans (user_id, clan_id, role) VALUES ($1, $2, 'Leader')", sender.ID, clanID)
	if err != nil {
		return c.Send("⚠️ Error writing alliance membership.")
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Failed committing clan creation: %v", err)
		return c.Send("⚠️ Error establishing Clan.")
	}

	return c.Send(fmt.Sprintf("🛡️🎉 CLAN ESTABLISHED: \"%s\"! You are its Leader. Use /clans to see it listed, or /clan for your HUD.", name))
}

// HandleRenameClanCommand lets a Leader rename their clan for a real cost.
func (h *ClanHandler) HandleRenameClanCommand(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	newName := c.Message().Payload
	if newName == "" {
		return c.Send(fmt.Sprintf("⚠️ Usage: /clan_rename [new name]\n💰 Cost: %.0f Crystal\n📏 3-24 characters.", clanRenameCost))
	}
	if len(newName) < 3 || len(newName) > 24 {
		return c.Send("❌ Invalid Length: Clan name must be 3-24 characters.")
	}

	var clanID, role string
	err := h.DB.QueryRowContext(ctx, "SELECT c.id, uc.role FROM clans c JOIN user_clans uc ON uc.clan_id = c.id WHERE uc.user_id = $1", sender.ID).Scan(&clanID, &role)
	if err != nil {
		return c.Send("⚠️ You're not in a Clan.")
	}
	if role != "Leader" {
		return c.Send("❌ Access Denied: Only the Clan Leader can rename it.")
	}

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ Rename transaction failed.")
	}
	defer tx.Rollback()

	var crystal float64
	_ = tx.QueryRowContext(ctx, "SELECT crystal FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&crystal)
	if crystal < clanRenameCost {
		return c.Send(fmt.Sprintf("❌ Insufficient Crystal: Need %.0f, you have %.0f.", clanRenameCost, crystal))
	}

	_, err = tx.ExecContext(ctx, "UPDATE clans SET name = $1 WHERE id = $2", newName, clanID)
	if err != nil {
		return c.Send("❌ Name Taken: Another Clan already uses that name.")
	}
	_, _ = tx.ExecContext(ctx, "UPDATE resources SET crystal = crystal - $1 WHERE encampment_id = $2", clanRenameCost, campID)

	if err := tx.Commit(); err != nil {
		return c.Send("⚠️ Error saving new name.")
	}

	return c.Send(fmt.Sprintf("✅ CLAN RENAMED: Now known as \"%s\"!", newName))
}

// HandleLeaveClanCallback removes the member (or dissolves if Leader)
func (h *ClanHandler) HandleLeaveClanCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	clanID := c.Args()[0]

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transaction error."})
	}
	defer tx.Rollback()

	var role string
	_ = tx.QueryRowContext(ctx, "SELECT role FROM user_clans WHERE user_id = $1 AND clan_id = $2", sender.ID, clanID).Scan(&role)

	if role == "Leader" {
		_, _ = tx.ExecContext(ctx, "DELETE FROM clans WHERE id = $1", clanID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "💥 Alliance dissolved!"})
	} else {
		_, _ = tx.ExecContext(ctx, "DELETE FROM user_clans WHERE user_id = $1", sender.ID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "🚪 Left alliance."})
	}

	_ = tx.Commit()
	return h.HandleClanPanel(c)
}

// HandleDeclareClanWarCallback creates a REAL tracked war (48h duration,
// live score accumulated from actual raid outcomes between the two clans'
// members, resolved by the tick engine) - replacing the old stub that just
// sent a one-time notification with zero mechanical effect.
func (h *ClanHandler) HandleDeclareClanWarCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	clanID := c.Args()[0]

	var role string
	_ = h.DB.QueryRowContext(ctx, "SELECT role FROM user_clans WHERE user_id = $1 AND clan_id = $2", sender.ID, clanID).Scan(&role)
	if role != "Leader" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Access Denied: Leaders only."})
	}

	var alreadyAtWar bool
	_ = h.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM clan_wars WHERE (clan_a_id = $1 OR clan_b_id = $1) AND status = 'active')", clanID).Scan(&alreadyAtWar)
	if alreadyAtWar {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Your Clan is already at war! Only one war at a time."})
	}

	var enemyID, enemyName string
	err := h.DB.QueryRowContext(ctx, `
		SELECT id, name FROM clans 
		WHERE id != $1 
		AND id NOT IN (SELECT clan_a_id FROM clan_wars WHERE status = 'active' UNION SELECT clan_b_id FROM clan_wars WHERE status = 'active')
		ORDER BY RANDOM() LIMIT 1`, clanID).Scan(&enemyID, &enemyName)
	if errors.Is(err, sql.ErrNoRows) {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ No available rival Clans to declare war on right now."})
	}

	endsAt := time.Now().UTC().Add(48 * time.Hour)
	_, err = h.DB.ExecContext(ctx, "INSERT INTO clan_wars (clan_a_id, clan_b_id, ends_at) VALUES ($1, $2, $3)", clanID, enemyID, endsAt)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error declaring war."})
	}

	alert := fmt.Sprintf(
		"🚨⚔️ CLAN WAR DECLARED! ⚔️🚨\n\n"+
			"War has begun against [%s]! Duration: 48 hours.\n"+
			"📊 Every successful raid your Clan members win against enemy Clan members earns War Score.\n"+
			"🏆 The Clan with the highest score when the war ends claims victory and a shared spoils reward!",
		enemyName,
	)
	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("⚔️ WAR DECLARED on %s! 48h battle begins now.", enemyName)})

	// Notify all members of both clans
	rows, _ := h.DB.QueryContext(ctx, "SELECT user_id FROM user_clans WHERE clan_id IN ($1, $2)", clanID, enemyID)
	if rows != nil {
		for rows.Next() {
			var uid int64
			if rows.Scan(&uid) == nil {
				_, _ = h.DB.ExecContext(ctx, "INSERT INTO notifications (user_id, message, is_sent) VALUES ($1, $2, FALSE)", uid, alert)
			}
		}
		rows.Close()
	}

	return nil
}
