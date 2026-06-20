package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strconv" // Added missing strconv package to resolve compiler errors (Fixes undefined: strconv)

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type ClanHandler struct {
	DB *sql.DB
}

func NewClanHandler(db *sql.DB) *ClanHandler {
	return &ClanHandler{DB: db}
}

// HandleClanPanel renders alliance alignments, active wars, and target declarations (Clean split HUD)
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
		panelText := "━━━━━━━━━━━━━━━━━━━━━━\n" +
			"🛡️ SECTOR ALLIANCE NETWORK\n" +
			"━━━━━━━━━━━━━━━━━━━━━━\n" +
			"Alliances unite up to 15 commanders. Joint defensive grids and war decks active.\n\n" +
			"You are currently unaligned.\n" +
			"━━━━━━━━━━━━━━━━━━━━━━"

		btnCreate := selector.Data("🛡️ Establish New Clan", "create_clan", campID)
		selector.Inline(selector.Row(btnCreate))

		return c.Send(panelText, selector)
	}

	var members int
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_clans WHERE clan_id = $1", clanID.String).Scan(&members)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🛡️ CLAN HUD: %s\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Commanders Enlisted: %d / 15 members\n"+
			"Your Rank: %s\n\n"+
			"CLAN WAR MATRIX:\n"+
			"Matchmaking detects equivalent level alliances. Declarations cannot be avoided.\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		clanName.String, members, role.String,
	)

	var buttons []telebot.Row

	btnManage := selector.Data("👥 Manage Members", "clan_manage", clanID.String)
	btnStats := selector.Data("📊 Alliance Stats", "clan_stats", clanID.String)
	buttons = append(buttons, selector.Row(btnManage, btnStats))

	if role.String == "Leader" {
		btnDeclare := selector.Data("⚔️ Declare Alliance War", "declare_clan_war", clanID.String)
		buttons = append(buttons, selector.Row(btnDeclare))
	}
	btnLeave := selector.Data("🚪 Leave Alliance", "leave_clan", clanID.String)
	buttons = append(buttons, selector.Row(btnLeave))

	selector.Inline(buttons...)
	return c.Send(panelText, selector)
}

// HandleManageMembersCallback renders the roster management page with Promote & Kick inline buttons (Phase 3 Complete)
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

	rosterText := "━━━━━━━━━━━━━━━━━━━━━━\n" +
		"👥 ALLIANCE ROSTER LISTING\n" +
		"━━━━━━━━━━━━━━━━━━━━━━\n" +
		"Manage your allied commanders below:\n\n"

	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	index := 1
	for rows.Next() {
		var memberID int64
		var fName, username, role string
		if err := rows.Scan(&memberID, &fName, &username, &role); err == nil {
			rosterText += fmt.Sprintf("[%d] %s (@%s)\n    Rank: %s\n\n", index, fName, username, role)

			// Show administrative actions for leaders on non-leader members
			if senderRole == "Leader" && memberID != sender.ID {
				btnKick := selector.Data(fmt.Sprintf("❌ Kick [%d]", index), "clan_kick", strconv.FormatInt(memberID, 10))
				btnPromote := selector.Data(fmt.Sprintf("🛡️ Promote [%d]", index), "clan_promote", strconv.FormatInt(memberID, 10))
				buttons = append(buttons, selector.Row(btnPromote, btnKick))
			}
			index++
		}
	}

	rosterText += "━━━━━━━━━━━━━━━━━━━━━━"
	selector.Inline(buttons...)
	return c.Send(rosterText, selector)
}

// HandleKickMemberCallback processes kicking members from the alliance (Phase 3 Complete)
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

// HandlePromoteMemberCallback processes promoting members to Co-Leaders (Phase 3 Complete)
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

// HandleAllianceStatsCallback calculates the accumulated strength metrics (Phase 3 Complete)
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
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"📊 ALLIANCE STRENGTH SUMMARY\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n\n"+
			"🏆 Collective Outpost Level: Level %d\n"+
			"⚔️ Accumulated Military Power: %d Power Rating\n\n"+
			"MILITARY ASSET STOCKPILES:\n"+
			"🪖 Combined Infantry: %d Soldiers\n"+
			"🤖 Combined Armored Core: %d Mechs\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		totalLevel, alliancePower, totalSoldiers, totalMechs,
	)

	return c.Send(report, keyboards.EconomyNavigation())
}

// HandleCreateClanCallback establishes an alliance with the player as Leader
func (h *ClanHandler) HandleCreateClanCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Alliance transaction failed."})
	}
	defer tx.Rollback()

	var exists bool
	_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM user_clans WHERE user_id = $1)", sender.ID).Scan(&exists)
	if exists {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Already in an active alliance!"})
	}

	clanName := fmt.Sprintf("Clan-%d", sender.ID%100)

	var clanID string
	err = tx.QueryRowContext(ctx, "INSERT INTO clans (name, leader_id) VALUES ($1, $2) RETURNING id", clanName, sender.ID).Scan(&clanID)
	if err != nil {
		log.Printf("Failed writing clan: %v", err)
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing clan registry."})
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO user_clans (user_id, clan_id, role) VALUES ($1, $2, 'Leader')", sender.ID, clanID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing alliance membership."})
	}

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("🛡️ %s established!", clanName)})
	return h.HandleClanPanel(c)
}

// HandleLeaveClanCallback removes the member
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

// HandleDeclareClanWarCallback triggers clan war declarations
func (h *ClanHandler) HandleDeclareClanWarCallback(c telebot.Context) error {
	ctx := context.Background()
	clanID := c.Args()[0]

	var enemyClanID string
	var enemyClanName string
	queryEnemy := `
		SELECT id, name 
		FROM clans 
		WHERE id != $1 
		ORDER BY RANDOM() 
		LIMIT 1`

	err := h.DB.QueryRowContext(ctx, queryEnemy, clanID).Scan(&enemyClanID, &enemyClanName)
	if errors.Is(err, sql.ErrNoRows) {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Scanning: No equivalent rivals detected."})
	}

	alert := fmt.Sprintf(
		"🚨 ALLIANCE WAR DECLARED!\n\n"+
			"Your clan has launched war metrics on [%s]. Combat declarations cannot be avoided.",
		enemyClanName,
	)
	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("⚔️ War metrics loaded on %s!", enemyClanName)})

	return c.Send(alert)
}
