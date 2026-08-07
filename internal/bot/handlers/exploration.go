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
// throughout the rest of the game.
func rewardEmoji(rewardType string) string {
	switch rewardType {
	case "scrap":
		return "⚙️"
	case "metal":
		return "🔩"
	case "rations":
		return "🍞"
	case "electricity":
		return "⚡"
	case "hydrogen":
		return "💧"
	case "neuro_cores":
		return "🧠"
	case "crystal":
		return "🔮"
	case "ether":
		return "✨"
	case "dollars":
		return "💵"
	default:
		return "📦"
	}
}

// explorationSiteTemplate is one entry in the reward pool a personal
// exploration expedition rolls from at dispatch time - a flavor name/type
// paired with a reward currency, amount range, and a relative weight.
//
// Rebuilt 2026-07-26 to fix a real scarcity bug: exploration used to be a
// SINGLE shared, contested site per continent (one player claims it, every
// other concurrent player in that continent gets nothing until the next
// site randomly spawns - which itself only had a 15%-per-tick chance).
// With many concurrent players sharing one continent, most players could
// go a very long time without ever successfully claiming a site at all -
// which meant they never even reached the discovery roll below, no matter
// how many times they tried to dispatch. Exploration is now personal and
// always available (subject only to one-dispatch-at-a-time per outpost,
// same as before): every dispatch generates its own site, so there's
// nothing to race for and nobody else's activity can starve you out.
//
// Reward variety was also expanded from 4 currencies (metal/crystal/ether/
// dollars) to all 9 real resources, weighted so everyday materiel is
// common and Crystal/Ether stay rare finds - not the ONLY things
// exploration ever turns up.
type explorationSiteTemplate struct {
	siteType   string
	namePrefix string
	rewardType string
	minAmount  float64
	maxAmount  float64
	weight     int
}

var explorationTemplates = []explorationSiteTemplate{
	{"Scrapyard", "Scrap Yard", "scrap", 150, 400, 20},
	{"Cache", "Supply Cache", "metal", 100, 300, 18},
	{"Depot", "Ration Depot", "rations", 80, 200, 16},
	{"Generator", "Power Cell Cluster", "electricity", 60, 150, 14},
	{"Reserve", "Fuel Reserve", "hydrogen", 40, 100, 10},
	{"Beacon", "Signal Beacon", "dollars", 300, 800, 10},
	{"Artifact", "Tech Artifact", "neuro_cores", 10, 30, 6},
	{"Ruins", "Ancient Ruins", "ether", 15, 40, 4},
	{"Vein", "Crystal Vein", "crystal", 5, 15, 2},
}

// rollExplorationTemplate picks a reward template weighted by rarity.
func rollExplorationTemplate() explorationSiteTemplate {
	total := 0
	for _, t := range explorationTemplates {
		total += t.weight
	}
	roll := rand.Intn(total)
	for _, t := range explorationTemplates {
		if roll < t.weight {
			return t
		}
		roll -= t.weight
	}
	return explorationTemplates[0]
}

// HandleExplorePanel (/explore) shows the status of any expedition the
// player already has en route, or lets them launch a new one.
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
		// 2026-08-06: the reward line used to spell out the exact
		// amount and resource in plain text the instant a player
		// dispatched - which flatly contradicted the panel's own
		// pitch two screens back ("has a real chance of making first
		// contact... every expedition returns with a resource haul")
		// by resolving all the suspense before the expedition had even
		// left. Now hidden behind Telegram's tap-to-reveal spoiler
		// (htmlSpoiler, render.go) while the expedition's still en
		// route - the actual haul lands, unhidden, in the completion
		// notification once it's genuinely earned (see engine.go's
		// resolveExplorationDispatches). A curious player can still
		// tap to peek early; this isn't a hard lock, just a real
		// choice between spoiling it themselves or waiting for the
		// reveal - which is what a spoiler tag is actually for.
		panelText := fmt.Sprintf(
			"🧭━━━━━━━━━━━━━━━━━━━━━━🧭\n"+
				"🧭 WORLD EXPLORATION: EXPEDITION EN ROUTE\n"+
				"🧭━━━━━━━━━━━━━━━━━━━━━━🧭\n"+
				"Target: %s\n"+
				"Expected reward: %s\n"+
				"ETA: %d min\n"+
				"🧭━━━━━━━━━━━━━━━━━━━━━━🧭",
			htmlEscape(siteName), htmlSpoiler(fmt.Sprintf("%s %.0f %s", rewardEmoji(siteRewardType), siteRewardAmount, htmlEscape(siteRewardType))),
			int(remaining.Minutes())+1,
		)
		return c.Send(panelText, telebot.ModeHTML, keyboards.CombatNavigation())
	}

	panelText := fmt.Sprintf(
		"🧭━━━━━━━━━━━━━━━━━━━━━━🧭\n"+
			"🧭 WORLD EXPLORATION: %s SECTOR\n"+
			"🧭━━━━━━━━━━━━━━━━━━━━━━🧭\n"+
			"Launch a personal survey expedition into your sector. Cost: %.0f Rations, %.0f Metal.\n"+
			"Recon capability: %d Scout Walker(s) | New-contact chance: %.0f%%.\n\n"+
			"Every expedition returns with a resource haul, and has a real chance of\n"+
			"making first contact with another outpost or AI faction along the way -\n"+
			"more Scout Walkers improve that chance.\n"+
			"🧭━━━━━━━━━━━━━━━━━━━━━━🧭",
		campRegion, explorationDispatchRationsCost, explorationDispatchMetalCost,
		scouts, worldintel.ExplorationDiscoveryChance(scouts)*100,
	)

	selector := &telebot.ReplyMarkup{}
	btn := selector.Data("🧭 Launch Expedition", "explore_dispatch")
	selector.Inline(selector.Row(btn))
	return sendPanelWithNav(c, navCaptionCombat, keyboards.CombatNavigation(), panelText, selector)
}

// HandleDispatchExpeditionCallback fires when a player taps "Launch
// Expedition". Generates a personal exploration site on the spot (see the
// explorationSiteTemplate doc comment above for why this is no longer a
// shared, contested resource) and immediately dispatches to it - nobody
// else's activity can block or race this outpost's own expedition.
func (h *ExplorationHandler) HandleDispatchExpeditionCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	var campID, campRegion string
	err := h.DB.QueryRowContext(ctx, "SELECT e.id, c.region FROM encampments e JOIN coordinates c ON c.id = e.coordinate_id WHERE e.user_id = $1", sender.ID).Scan(&campID, &campRegion)
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

	var rations, metal float64
	_ = tx.QueryRowContext(ctx, "SELECT rations, metal FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&rations, &metal)
	if rations < explorationDispatchRationsCost || metal < explorationDispatchMetalCost {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient supplies! Need %.0f Rations, %.0f Metal.", explorationDispatchRationsCost, explorationDispatchMetalCost)})
	}

	// Generate this outpost's own personal site - nothing shared, nothing
	// to race another player for. See the explorationSiteTemplate doc
	// comment for why this replaced the old shared-site-pool design.
	tmpl := rollExplorationTemplate()
	sector := rand.Intn(99) + 1
	siteName := fmt.Sprintf("%s (Sector %d)", tmpl.namePrefix, sector)
	rewardAmount := tmpl.minAmount + rand.Float64()*(tmpl.maxAmount-tmpl.minAmount)
	expiresAt := time.Now().UTC().Add(24 * time.Hour) // generous window; this site only ever belongs to this dispatch

	var siteID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO exploration_sites (continent, site_name, site_type, reward_type, reward_amount, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`, campRegion, siteName, tmpl.siteType, tmpl.rewardType, rewardAmount, expiresAt).Scan(&siteID); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error generating expedition site."})
	}

	_, err = tx.ExecContext(ctx, "INSERT INTO exploration_dispatches (site_id, encampment_id, user_id, resolve_time) VALUES ($1, $2, $3, $4)",
		siteID, campID, sender.ID, time.Now().UTC().Add(time.Duration(explorationMinTravelMinutes+rand.Intn(explorationMaxTravelMinutes-explorationMinTravelMinutes+1))*time.Minute))
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error dispatching expedition."})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET rations = rations - $1, metal = metal - $2 WHERE encampment_id = $3",
		explorationDispatchRationsCost, explorationDispatchMetalCost, campID)

	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error dispatching expedition."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🧭 Expedition dispatched! Check /explore for its ETA."})
	return h.HandleExplorePanel(c)
}
