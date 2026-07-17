package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/scoring"
	"gopkg.in/telebot.v3"
)

type FederationHandler struct {
	DB *sql.DB
}

func NewFederationHandler(db *sql.DB) *FederationHandler {
	return &FederationHandler{DB: db}
}

const federationFoundCost = 5000.0 // Crystal - founding a Federation is a major, deliberate milestone

// getMyClan resolves the caller's clan membership and whether they're the
// King (leader), returning (clanID, isKing, error).
func (h *FederationHandler) getMyClan(ctx context.Context, userID int64) (string, bool, error) {
	var clanID string
	var leaderID int64
	err := h.DB.QueryRowContext(ctx, `
		SELECT c.id, c.leader_id 
		FROM clans c 
		JOIN user_clans uc ON uc.clan_id = c.id 
		WHERE uc.user_id = $1`, userID).Scan(&clanID, &leaderID)
	if err != nil {
		return "", false, err
	}
	return clanID, leaderID == userID, nil
}

// HandleFederationsPanel (/federations) lists all Federations ranked by
// combined score of every member clan's members.
func (h *FederationHandler) HandleFederationsPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	ctx := context.Background()

	panelText := "🌐━━━━━━━━━━━━━━━━━━━━━━🌐\n" +
		"🏛️ FEDERATIONS OF THE WASTELAND 🏛️\n" +
		"🌐━━━━━━━━━━━━━━━━━━━━━━🌐\n\n"

	query := fmt.Sprintf(`
		SELECT f.name, f.icon, COUNT(DISTINCT c.id) as clan_count, COUNT(DISTINCT uc.user_id) as member_count, COALESCE(SUM(%s), 0) as total_score
		FROM federations f
		JOIN clans c ON c.federation_id = f.id
		JOIN user_clans uc ON uc.clan_id = c.id
		JOIN encampments e ON e.user_id = uc.user_id
		GROUP BY f.name, f.icon
		ORDER BY total_score DESC
		LIMIT 15`, scoring.ScoreExpr)

	rows, err := h.DB.QueryContext(ctx, query)
	if err == nil {
		rank := 1
		any := false
		for rows.Next() {
			var name, icon string
			var clanCount, memberCount int
			var score float64
			if scanErr := rows.Scan(&name, &icon, &clanCount, &memberCount, &score); scanErr == nil {
				any = true
				panelText += fmt.Sprintf("%s %d. %s %s — 🏴 %d Clans, 👥 %d Members, 🏅 %.0f pts\n", medalFor(rank), rank, icon, name, clanCount, memberCount, score)
				rank++
			}
		}
		rows.Close()
		if !any {
			panelText += "No Federations have been founded yet. Be the first!\n"
		}
	}

	panelText += "\n💡 A Clan King can found a Federation with /fed_found [name], join one with /fed_join [name], or leave with /fed_leave.\n" +
		"🌐━━━━━━━━━━━━━━━━━━━━━━🌐"

	return c.Send(panelText, keyboards.EconomyNavigation())
}

// HandleMyFederationPanel (/federation) shows detailed info on the
// caller's own Federation.
func (h *FederationHandler) HandleMyFederationPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	ctx := context.Background()

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	clanID, _, err := h.getMyClan(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ You must be in a Clan first. Use /clan to create or join one.")
	}

	var fedID, fedName, fedIcon, fedDesc string
	err = h.DB.QueryRowContext(ctx, "SELECT f.id, f.name, f.icon, f.description FROM federations f JOIN clans c ON c.federation_id = f.id WHERE c.id = $1", clanID).Scan(&fedID, &fedName, &fedIcon, &fedDesc)
	if err != nil {
		return c.Send("⚠️ Your Clan isn't part of a Federation yet. A King can found one with /fed_found [name] or join one with /fed_join [name].")
	}

	panelText := fmt.Sprintf(
		"🌐━━━━━━━━━━━━━━━━━━━━━━🌐\n"+
			"%s %s\n"+
			"🌐━━━━━━━━━━━━━━━━━━━━━━🌐\n"+
			"📜 %s\n\n"+
			"🏴 MEMBER CLANS:\n",
		fedIcon, fedName, fedDesc,
	)

	rows, err := h.DB.QueryContext(ctx, `
		SELECT c.name, COUNT(uc.user_id) as members
		FROM clans c
		LEFT JOIN user_clans uc ON uc.clan_id = c.id
		WHERE c.federation_id = $1
		GROUP BY c.name
		ORDER BY members DESC`, fedID)
	if err == nil {
		for rows.Next() {
			var name string
			var members int
			if scanErr := rows.Scan(&name, &members); scanErr == nil {
				panelText += fmt.Sprintf("🏴 %s (%d members)\n", name, members)
			}
		}
		rows.Close()
	}

	panelText += "🌐━━━━━━━━━━━━━━━━━━━━━━🌐"

	return c.Send(panelText, keyboards.EconomyNavigation())
}

// HandleFoundFederation (/fed_found [name]) - a Clan King founds a new
// Federation, deliberately costly since it's a major milestone.
func (h *FederationHandler) HandleFoundFederation(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	name := c.Message().Payload
	if name == "" {
		return c.Send(fmt.Sprintf("⚠️ Usage: /fed_found [name]\n💰 Cost: %.0f Crystal\n🔒 Only your Clan's King can found a Federation.", federationFoundCost))
	}

	clanID, isKing, err := h.getMyClan(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ You must be in a Clan first. Use /clan to create or join one.")
	}
	if !isKing {
		return c.Send("❌ Only your Clan's King can found a Federation.")
	}

	var existingFed sql.NullString
	_ = h.DB.QueryRowContext(ctx, "SELECT federation_id FROM clans WHERE id = $1", clanID).Scan(&existingFed)
	if existingFed.Valid {
		return c.Send("❌ Your Clan is already part of a Federation. Use /fed_leave first.")
	}

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ Founding transaction failed.")
	}
	defer tx.Rollback()

	var crystal float64
	_ = tx.QueryRowContext(ctx, "SELECT crystal FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&crystal)
	if crystal < federationFoundCost {
		return c.Send(fmt.Sprintf("❌ Insufficient Crystal: Need %.0f, you have %.0f.", federationFoundCost, crystal))
	}

	var fedID string
	err = tx.QueryRowContext(ctx, "INSERT INTO federations (name, founder_clan_id) VALUES ($1, $2) RETURNING id", name, clanID).Scan(&fedID)
	if err != nil {
		return c.Send("❌ Name Taken: A Federation with that name already exists.")
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET crystal = crystal - $1 WHERE encampment_id = $2", federationFoundCost, campID)
	_, _ = tx.ExecContext(ctx, "UPDATE clans SET federation_id = $1 WHERE id = $2", fedID, clanID)

	if err := tx.Commit(); err != nil {
		return c.Send("⚠️ Error founding Federation.")
	}

	return c.Send(fmt.Sprintf("🌐🎉 FEDERATION FOUNDED: \"%s\"! Other Clan Kings can now join with /fed_join %s", name, name))
}

// HandleJoinFederation (/fed_join [name]) - a Clan King brings their whole
// Clan into an existing Federation.
func (h *FederationHandler) HandleJoinFederation(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	name := c.Message().Payload
	if name == "" {
		return c.Send("⚠️ Usage: /fed_join [federation name]")
	}

	clanID, isKing, err := h.getMyClan(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ You must be in a Clan first. Use /clan to create or join one.")
	}
	if !isKing {
		return c.Send("❌ Only your Clan's King can join a Federation.")
	}

	var fedID string
	err = h.DB.QueryRowContext(ctx, "SELECT id FROM federations WHERE LOWER(name) = LOWER($1)", name).Scan(&fedID)
	if err != nil {
		return c.Send("❌ No Federation found with that name. Check /federations for the list.")
	}

	_, err = h.DB.ExecContext(ctx, "UPDATE clans SET federation_id = $1 WHERE id = $2", fedID, clanID)
	if err != nil {
		return c.Send("⚠️ Error joining Federation.")
	}

	return c.Send(fmt.Sprintf("🌐✅ Your Clan has joined \"%s\"!", name))
}

// HandleLeaveFederation (/fed_leave) - a Clan King removes their Clan from
// its current Federation.
func (h *FederationHandler) HandleLeaveFederation(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	clanID, isKing, err := h.getMyClan(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ You must be in a Clan first.")
	}
	if !isKing {
		return c.Send("❌ Only your Clan's King can leave a Federation.")
	}

	_, err = h.DB.ExecContext(ctx, "UPDATE clans SET federation_id = NULL WHERE id = $1", clanID)
	if err != nil {
		return c.Send("⚠️ Error leaving Federation.")
	}

	return c.Send("🌐 Your Clan has left its Federation.")
}
