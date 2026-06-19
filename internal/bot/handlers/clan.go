package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

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

	// Fetch current clan status
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
	
	// --- CLAN MANAGEMENT MODULES (Phase 3 Additions) ---
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

// HandleClanManageCallback lists members with administrative kick commands (Phase 3)
func (h *ClanHandler) HandleClanManageCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	clanID := c.Args()[0]

	var myRole string
	_ = h.DB.QueryRowContext(ctx, "SELECT role FROM user_clans WHERE user_id = $1", sender.ID).Scan(&myRole)

	rows, err := h.DB.QueryContext(ctx, "SELECT uc.user_id, u.first_name, uc.role FROM user_clans uc JOIN users u ON u.telegram_id = uc.user_id WHERE uc.clan_id = $1 LIMIT 15", clanID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error reading database registers."})
	}
	defer rows.Close()

	panelText := "━━━━━━━━━━━━━━━━━━━━━━\n" +
		"👥 ALLIANCE ROSTER DIRECTORY\n" +
		"━━━━━━━━━━━━━━━━━━━━━━\n"

	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	index := 1
	for rows.Next() {
		var memberID int64
		var mName, mRole string
		if err := rows.Scan(&memberID, &mName, &mRole); err == nil {
			panelText += fmt.Sprintf("[%d] %s (%s)\n", index, mName, mRole)
			if myRole == "Leader" && memberID != sender.ID {
				btnKick := selector.Data(fmt.Sprintf("❌ Kick %s", mName), "kick_member", clanID, strconv.FormatInt(memberID, 10))
				buttons = append(buttons, selector.Row(btnKick))
			}
			index++
		}
	}
	rows.Close()
	panelText += "━━━━━━━━━━━━━━━━━━━━━━"

	btnBack := selector.Data("⬅️ Back to Alliance", "back_to_clan")
	buttons = append(buttons, selector.Row(btnBack))

	selector.Inline(buttons...)
	return c.Send(panelText, selector)
}

// HandleKickMemberCallback processes kicking a player from the alliance (Phase 3)
func (h *ClanHandler) HandleKickMemberCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	clanID := c.Args()[0]
	targetUserIDStr := c.Args()[1]

	targetUserID, err := strconv.ParseInt(targetUserIDStr, 10, 64)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ ID resolution error."})
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transaction failed."})
	}
	defer tx.Rollback()

	// Verify sender is leader
	var role string
	_ = tx.QueryRowContext(ctx, "SELECT role FROM user_clans WHERE user_id = $1 AND clan_id = $2", sender.ID, clanID).Scan(&role)
	if role != "Leader" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Action Forbidden: Leader clearance required."})
	}

	_, _ = tx.ExecContext(ctx, "DELETE FROM user_clans WHERE user_id = $1 AND clan_id = $2", targetUserID, clanID)

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: "❌ Commander dismissed from alliance."})

	return h.HandleClanPanel(c)
}

// HandleClanStatsCallback aggregates combined alliance metrics (Phase 3)
func (h *ClanHandler) HandleClanStatsCallback(c telebot.Context) error {
	ctx := context.Background()
	clanID := c.Args()[0]

	var membersCount int
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_clans WHERE clan_id = $1", clanID).Scan(&membersCount)

	// Aggregate combined resources of all members
	var totalScrap float64
	queryScrap := `
		SELECT COALESCE(SUM(r.scrap), 0.0) 
		FROM resources r 
		JOIN encampments e ON e.id = r.encampment_id 
		WHERE e.user_id IN (SELECT user_id FROM user_clans WHERE clan_id = $1)`
	_ = h.DB.QueryRowContext(ctx, queryScrap, clanID).Scan(&totalScrap)

	// Aggregate combined active soldiers in barracks
	var totalSoldiers int
	queryMilitary := `
		SELECT COALESCE(SUM(w.soldiers), 0) 
		FROM workshop_inventory w 
		JOIN encampments e ON e.id = w.encampment_id 
		WHERE e.user_id IN (SELECT user_id FROM user_clans WHERE clan_id = $1)`
	_ = h.DB.QueryRowContext(ctx, queryMilitary, clanID).Scan(&totalSoldiers)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"📊 SECTOR ALLIANCE METRICS\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"COMBINED ALLIANCE BALANCES:\n"+
			"👥 Enlisted commanders: %d / 15\n"+
			"⚙️ Accumulated Scrap: %.1f Scrap\n"+
			"🪖 Standing Barracks Troops: %d Soldiers\n\n"+
			"Alliance updates are calculated instantly during global ticks.\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		membersCount, totalScrap, totalSoldiers,
	)

	selector := &telebot.ReplyMarkup{}
	btnBack := selector.Data("⬅️ Back to Alliance", "back_to_clan")
	selector.Inline(selector.Row(btnBack))

	return c.Send(panelText, selector)
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