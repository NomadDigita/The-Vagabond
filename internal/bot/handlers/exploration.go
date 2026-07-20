package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/worldintel"
	"gopkg.in/telebot.v3"
)

type ExplorationHandler struct {
	DB *sql.DB
}

func NewExplorationHandler(db *sql.DB) *ExplorationHandler {
	return &ExplorationHandler{DB: db}
}

const (
	explorationDispatchRationsCost = 30.0
	explorationDispatchMetalCost   = 15.0
	explorationMinTravelMinutes    = 20
	explorationMaxTravelMinutes    = 45
)

// rewardEmoji gives the display icon for an exploration site's reward
// currency, matching the icons already used for these resources
// throughout the rest of the game (Metal 🔩, Crystal 💎, Electricity ⚡).
func rewardEmoji(rewardType string) string {
	switch rewardType {
	case "metal":
		return "🔩"
	case "crystal":
		return "💎"
	case "ether":
		return "🔮"
	case "dollars":
		return "💵"
	default:
		return "📦"
	}
}

// HandleExplorePanel (/explore) shows the unclaimed exploration sites
// currently active in the player's own continent, plus the status of
// any expedition they already have en route.
func (h *ExplorationHandler) HandleExplorePanel(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	var campID, campRegion string
	var scouts int
	err := h.DB.QueryRowContext(ctx,
		"SELECT e.id, c.region, COALESCE(w.scouts, 0) FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id LEFT JOIN workshop_inventory w ON w.encampment_id = e.id WHERE e.user_id = $1", sender.ID).
		Scan(&campID, &campRegion, &scouts)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.MainNavigation())
	}

	// Show any expedition this outpost already has in flight first -
	// only one at a time, matching the one-dispatch-per-outpost-at-once
	// design (keeps this from being spammable).
	var siteName, siteRewardType string
	var siteRewardAmount float64
	var resolveTime time.Time
	err = h.DB.QueryRowContext(ctx, `
		SELECT s.site_name, s.reward_type, s.reward_amount, d.resolve_time
		FROM exploration_dispatches d
		JOIN exploration_sites s ON s.id = d.site_id
		WHERE d.encampment_id = $1`, campID).Scan(&siteName, &siteRewardType, &siteRewardAmount, &resolveTime)
	if err == nil {
		remaining := time.Until(resolveTime.UTC())
		if remaining < 0 {
			remaining = 0
		}
		panelText := fmt.Sprintf(
			"🧭━━━━━━━━━━━━━━━━━━━━━━🧭\n"+
				"🧭 WORLD EXPLORATION: EXPEDITION EN ROUTE\n"+
				"🧭━━━━━━━━━━━━━━━━━━━━━━🧭\n"+
				"Target: %s\n"+
				"Expected reward: %s %.0f %s\n"+
				"ETA: %d min\n"+
				"🧭━━━━━━━━━━━━━━━━━━━━━━🧭",
			siteName, rewardEmoji(siteRewardType), siteRewardAmount, siteRewardType,
			int(remaining.Minutes())+1,
		)
		return c.Send(panelText, keyboards.CombatNavigation())
	}

	rows, err := h.DB.QueryContext(ctx, `
		SELECT id, site_name, site_type, reward_type, reward_amount, expires_at
		FROM exploration_sites
		WHERE continent = $1 AND claimed_by IS NULL AND expires_at > CURRENT_TIMESTAMP
		ORDER BY expires_at ASC`, campRegion)
	if err != nil {
		return c.Send("⚠️ Error scanning exploration sector.")
	}
	defer rows.Close()

	panelText := fmt.Sprintf(
		"🧭━━━━━━━━━━━━━━━━━━━━━━🧭\n"+
			"🧭 WORLD EXPLORATION: %s SECTOR\n"+
			"🧭━━━━━━━━━━━━━━━━━━━━━━🧭\n"+
			"Dispatch an expedition to an undiscovered site before a rival\n"+
			"outpost claims it first. Cost: %.0f Rations, %.0f Metal.\n"+
			"Recon capability: %d Scout Walker(s) | New-contact chance: %.0f%%.\n\n",
		campRegion, explorationDispatchRationsCost, explorationDispatchMetalCost,
		scouts, worldintel.ExplorationDiscoveryChance(scouts)*100,
	)

	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row
	found := false
	for rows.Next() {
		var id, name, siteType, rewardType string
		var rewardAmount float64
		var expiresAt time.Time
		if err := rows.Scan(&id, &name, &siteType, &rewardType, &rewardAmount, &expiresAt); err != nil {
			continue
		}
		found = true
		panelText += fmt.Sprintf("📍 %s (%s)\n   Reward: %s %.0f %s\n\n", name, siteType, rewardEmoji(rewardType), rewardAmount, rewardType)
		btn := selector.Data(fmt.Sprintf("🧭 Dispatch to %s", name), "explore_dispatch", id)
		buttons = append(buttons, selector.Row(btn))
	}

	if !found {
		panelText += "No undiscovered sites currently detected in this sector. Check back later.\n"
	}
	panelText += "🧭━━━━━━━━━━━━━━━━━━━━━━🧭"

	selector.Inline(buttons...)
	return sendPanelWithNav(c, navCaptionCombat, keyboards.CombatNavigation(), panelText, selector)
}

// HandleDispatchExpeditionCallback fires when a player taps "Dispatch"
// on an undiscovered site. Locks in the claim attempt immediately
// (exploration_dispatches.site_id is UNIQUE, so a second dispatch to
// the same site simply fails the insert) rather than waiting until
// resolution, so two players racing for the same site can't both spend
// the cost only to have one refunded later.
func (h *ExplorationHandler) HandleDispatchExpeditionCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	siteID := c.Args()[0]

	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error resolving Outpost."})
	}

	var existingDispatch bool
	_ = h.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM exploration_dispatches WHERE encampment_id = $1)", campID).Scan(&existingDispatch)
	if existingDispatch {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ You already have an expedition en route. Wait for it to resolve first."})
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transaction failed."})
	}
	defer tx.Rollback()

	var claimed sql.NullString
	var expiresAt time.Time
	err = tx.QueryRowContext(ctx, "SELECT claimed_by, expires_at FROM exploration_sites WHERE id = $1 FOR UPDATE", siteID).Scan(&claimed, &expiresAt)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Site no longer exists."})
	}
	if claimed.Valid || time.Now().UTC().After(expiresAt.UTC()) {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Too late! Another outpost already claimed this site, or it has expired."})
	}

	var rations, metal float64
	_ = tx.QueryRowContext(ctx, "SELECT rations, metal FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&rations, &metal)
	if rations < explorationDispatchRationsCost || metal < explorationDispatchMetalCost {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient supplies! Need %.0f Rations, %.0f Metal.", explorationDispatchRationsCost, explorationDispatchMetalCost)})
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO exploration_dispatches (site_id, encampment_id, user_id, resolve_time) VALUES ($1, $2, $3, $4)",
		siteID, campID, sender.ID, time.Now().UTC().Add(time.Duration(explorationMinTravelMinutes+rand.Intn(explorationMaxTravelMinutes-explorationMinTravelMinutes+1))*time.Minute))
	if err != nil {
		// UNIQUE(site_id) violation means someone else's dispatch beat this one to the transaction.
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Too late! Another outpost's expedition beat you to this site."})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET rations = rations - $1, metal = metal - $2 WHERE encampment_id = $3",
		explorationDispatchRationsCost, explorationDispatchMetalCost, campID)

	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error dispatching expedition."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🧭 Expedition dispatched! Check /explore for its ETA."})
	return h.HandleExplorePanel(c)
}
