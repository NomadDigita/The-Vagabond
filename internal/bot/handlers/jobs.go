package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
	"gopkg.in/telebot.v3"
)

type JobsHandler struct {
	DB *sql.DB
}

func NewJobsHandler(db *sql.DB) *JobsHandler {
	return &JobsHandler{DB: db}
}

func (h *JobsHandler) myCamp(ctx context.Context, userID int64) (string, error) {
	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", userID).Scan(&campID)
	return campID, err
}

func sendConfirmCard(c telebot.Context, title, costLine, whyLine, actionKey string) error {
	cardText := title + "\n" + divider + "\n" +
		costLine + "\n\n" +
		htmlItalic(whyLine) + "\n" +
		divider

	selector := &telebot.ReplyMarkup{}
	btnConfirm := keyboards.Styled(selector.Data("✅ Confirm", actionKey+"_c"), keyboards.StyleSuccess)
	btnCancel := keyboards.Styled(selector.Data("❌ Cancel", actionKey+"_x"), keyboards.StyleDanger)
	return keyboards.SendStyled(c, cardText, [][]keyboards.StyledBtn{{btnCancel, btnConfirm}})
}

func sendCancelledCard(c telebot.Context, jobLabel string) error {
	_ = c.Edit(fmt.Sprintf("%s cancelled - nothing was charged, nothing changed.", jobLabel))
	return c.Respond()
}

func (h *JobsHandler) HandleJobsPanel(c telebot.Context) error {
	panelText := "🛠️━━━━━━━━━━━━━━━━━━━━━━🛠️\n" +
		htmlBold("ODD JOBS - FIELD OPERATIONS") + "\n" +
		"🛠️━━━━━━━━━━━━━━━━━━━━━━🛠️\n\n" +
		"One-off actions that don't fit anywhere else - pick one from the menu below.\n\n" +
		"🚀 " + htmlBold("HyperSpeed Mission") + " - 300 Electricity, halves your nearest active mission's remaining time.\n" +
		"🌍 " + htmlBold("Extend Planet") + " - Metal + Crystal (scales with level), +1000 storage capacity permanently.\n" +
		"🌀 " + htmlBold("Teleport Outpost") + " - 1000 Electricity, relocates you to a fresh coordinate. 24h cooldown.\n" +
		"👻 " + htmlBold("Ghost Protocol") + " - severe cost, 90-day cooldown. Relocates AND erases every locked-on scout/attacker's fix on your position. Not a substitute for Teleport - see /ghostprotocol for the full breakdown before using it.\n" +
		"🛰️ " + htmlBold("Orbital Maneuver") + " - 400 Electricity, a 2-hour tactical buff.\n" +
		"🔧 " + htmlBold("Repair Units") + " - 200 Scrap, restores 5 lost Soldiers.\n" +
		"🏚️ " + htmlBold("Repair Buildings") + " - 150 Scrap, clears structural damage.\n" +
		"☀️ " + htmlBold("Gather Sunlight") + " - free, +150 Electricity. 30-minute cooldown.\n\n" +
		"🛠️━━━━━━━━━━━━━━━━━━━━━━🛠️"

	return sendPanelWithNavHTML(c, "🛠️ Accessing Field Operations...", keyboards.JobsNavigation(), panelText, &telebot.ReplyMarkup{})
}

type hyperspeedTarget struct {
	kind        string
	id          string
	resolveTime time.Time
}

type hyperspeedQuerier interface {
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}

func findHyperSpeedTarget(ctx context.Context, q hyperspeedQuerier, campID string, forUpdate bool) (hyperspeedTarget, error) {
	raidLock, scoutLock := "", ""
	if forUpdate {
		raidLock, scoutLock = " FOR UPDATE", " FOR UPDATE"
	}

	var best hyperspeedTarget
	found := false

	var raidID string
	var raidResolve time.Time
	if err := q.QueryRowContext(ctx,
		"SELECT id, resolve_time FROM raids WHERE attacker_id = $1 AND state IN ('marching','engaged','returning') ORDER BY resolve_time ASC LIMIT 1"+raidLock,
		campID).Scan(&raidID, &raidResolve); err == nil {
		best = hyperspeedTarget{kind: "raid", id: raidID, resolveTime: raidResolve}
		found = true
	}

	var scoutID string
	var scoutResolve time.Time
	if err := q.QueryRowContext(ctx,
		"SELECT id, return_eta FROM scout_missions WHERE encampment_id = $1 AND phase = 'returning' ORDER BY return_eta ASC LIMIT 1"+scoutLock,
		campID).Scan(&scoutID, &scoutResolve); err == nil {
		if !found || scoutResolve.Before(best.resolveTime) {
			best = hyperspeedTarget{kind: "scout", id: scoutID, resolveTime: scoutResolve}
			found = true
		}
	}

	if !found {
		return hyperspeedTarget{}, sql.ErrNoRows
	}
	return best, nil
}

func (t hyperspeedTarget) label() string {
	if t.kind == "scout" {
		return "long-range scouting party"
	}
	return "raid/expedition"
}

func (h *JobsHandler) HandleHyperSpeed(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.JobsNavigation())
	}

	target, err := findHyperSpeedTarget(ctx, h.DB, campID, false)
	if err != nil {
		return c.Send("❌ No active missions to accelerate. Launch a raid or dispatch scouts first!", keyboards.JobsNavigation())
	}
	if remaining := time.Until(target.resolveTime); remaining < time.Minute {
		return c.Send("⚠️ That mission is about to resolve already - no need for HyperSpeed.", keyboards.JobsNavigation())
	}

	return sendConfirmCard(c, "🚀 "+htmlBold("CONFIRM HYPERSPEED"),
		fmt.Sprintf("Cost: %s Electricity. Cuts your nearest active mission's remaining time in half (currently your %s).", htmlCode("300"), target.label()),
		"This spends real Electricity for a one-time timing boost - confirming first avoids burning it on a mission you'd rather let resolve naturally.",
		"hyperspeed")
}

func (h *JobsHandler) HandleHyperSpeedConfirmCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid confirmation."})
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	message, err := h.doHyperSpeed(ctx, campID)
	if err != nil {
		return c.Edit(message)
	}
	return c.Edit(message, telebot.ModeHTML)
}

func (h *JobsHandler) HandleHyperSpeedCancelCallback(c telebot.Context) error {
	return sendCancelledCard(c, "🚀 HyperSpeed")
}

func (h *JobsHandler) doHyperSpeed(ctx context.Context, campID string) (string, error) {
	const cost = 300.0

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return "⚠️ HyperSpeed activation failed.", err
	}
	defer tx.Rollback()

	var electricity float64
	_ = tx.QueryRowContext(ctx, "SELECT electricity FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&electricity)
	if electricity < cost {
		return fmt.Sprintf("❌ %s Need %s, you have %s.", htmlBold("Insufficient Electricity!"), htmlCode(fmt.Sprintf("%.0f", cost)), htmlCode(fmt.Sprintf("%.0f", electricity))), errors.New("insufficient electricity")
	}

	target, err := findHyperSpeedTarget(ctx, tx, campID, true)
	if err != nil {
		return "❌ No active missions to accelerate. Launch a raid or dispatch scouts first!", err
	}

	remaining := time.Until(target.resolveTime)
	if remaining < time.Minute {
		return "⚠️ That mission is about to resolve already - no need for HyperSpeed.", errors.New("mission resolving imminently")
	}
	newResolve := target.resolveTime.Add(-remaining / 2)

	if _, err := tx.ExecContext(ctx, "UPDATE resources SET electricity = electricity - $1 WHERE encampment_id = $2", cost, campID); err != nil {
		return "⚠️ Error deducting HyperSpeed's cost.", err
	}

	switch target.kind {
	case "scout":
		if _, err := tx.ExecContext(ctx, "UPDATE scout_missions SET return_eta = $1 WHERE id = $2", newResolve, target.id); err != nil {
			return "⚠️ Error activating HyperSpeed.", err
		}
	default:
		if _, err := tx.ExecContext(ctx, "UPDATE raids SET resolve_time = $1 WHERE id = $2", newResolve, target.id); err != nil {
			return "⚠️ Error activating HyperSpeed.", err
		}
	}

	if err := tx.Commit(); err != nil {
		return "⚠️ Error activating HyperSpeed.", err
	}

	return fmt.Sprintf("🚀⚡ %s Your nearest %s's remaining time was cut in half. New ETA: %s", htmlBold("HYPERSPEED ENGAGED!"), target.label(), htmlCode(newResolve.UTC().Format("15:04 MST"))), nil
}

func (h *JobsHandler) HandleExtendPlanet(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.JobsNavigation())
	}

	var extensionLvl int
	_ = h.DB.QueryRowContext(ctx, "SELECT COALESCE(extension_lvl, 0) FROM encampments WHERE id = $1", campID).Scan(&extensionLvl)
	metalCost := float64(500 * (extensionLvl + 1))
	crystalCost := float64(100 * (extensionLvl + 1))

	return sendConfirmCard(c, "🌍 "+htmlBold("CONFIRM PLANET EXTENSION"),
		fmt.Sprintf("Cost: %s Metal, %s Crystal. +1000 storage capacity, permanently (extension level %d → %d).",
			htmlCode(fmt.Sprintf("%.0f", metalCost)), htmlCode(fmt.Sprintf("%.0f", crystalCost)), extensionLvl, extensionLvl+1),
		"This is a permanent, ever-growing investment - each level costs more than the last, so confirming first avoids spending on a level-up you weren't ready to commit to yet.",
		"extendplanet")
}

func (h *JobsHandler) HandleExtendPlanetConfirmCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid confirmation."})
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	message, err := h.doExtendPlanet(ctx, campID)
	if err != nil {
		return c.Edit(message)
	}
	return c.Edit(message, telebot.ModeHTML)
}

func (h *JobsHandler) HandleExtendPlanetCancelCallback(c telebot.Context) error {
	return sendCancelledCard(c, "🌍 Planet Extension")
}

func (h *JobsHandler) doExtendPlanet(ctx context.Context, campID string) (string, error) {
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return "⚠️ Extension failed.", err
	}
	defer tx.Rollback()

	var extensionLvl int
	_ = tx.QueryRowContext(ctx, "SELECT COALESCE(extension_lvl, 0) FROM encampments WHERE id = $1 FOR UPDATE", campID).Scan(&extensionLvl)

	metalCost := float64(500 * (extensionLvl + 1))
	crystalCost := float64(100 * (extensionLvl + 1))

	var metal, crystal float64
	_ = tx.QueryRowContext(ctx, "SELECT metal, crystal FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&metal, &crystal)
	if metal < metalCost || crystal < crystalCost {
		return fmt.Sprintf("❌ %s Need %s for extension level %d.", htmlBold("Insufficient Materials!"), htmlCode(fmt.Sprintf("%.0f Metal, %.0f Crystal", metalCost, crystalCost)), extensionLvl+1), errors.New("insufficient materials")
	}

	if _, err := tx.ExecContext(ctx, "UPDATE resources SET metal = metal - $1, crystal = crystal - $2 WHERE encampment_id = $3", metalCost, crystalCost, campID); err != nil {
		return "⚠️ Error deducting extension's cost.", err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE encampments SET extension_lvl = extension_lvl + 1 WHERE id = $1", campID); err != nil {
		return "⚠️ Error extending planet.", err
	}

	if err := tx.Commit(); err != nil {
		return "⚠️ Error extending planet.", err
	}

	return fmt.Sprintf("🌍✅ %s Storage capacity +1000 permanently (extension level %d). Next extension: %s.", htmlBold("PLANET EXTENDED!"), extensionLvl+1, htmlCode(fmt.Sprintf("%.0f Metal, %.0f Crystal", metalCost*2, crystalCost*2))), nil
}

const (
	teleportCooldown     = 24 * time.Hour
	teleportCostFraction = 0.15
)

func (h *JobsHandler) HandleTeleport(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	var lastTeleport sql.NullTime
	_ = h.DB.QueryRowContext(ctx, "SELECT last_teleport_at FROM encampments WHERE id = $1", campID).Scan(&lastTeleport)
	if lastTeleport.Valid && time.Since(lastTeleport.Time) < teleportCooldown {
		remaining := teleportCooldown - time.Since(lastTeleport.Time)
		return c.Send(fmt.Sprintf("⏳ Teleport is on cooldown for another %.1f hours.", remaining.Hours()))
	}

	var scrap, metal, crystal, electricity, dollars float64
	_ = h.DB.QueryRowContext(ctx, "SELECT scrap, metal, crystal, electricity, dollars FROM resources WHERE encampment_id = $1", campID).Scan(&scrap, &metal, &crystal, &electricity, &dollars)
	scrapCost, metalCost, crystalCost, electricityCost, dollarsCost :=
		scrap*teleportCostFraction, metal*teleportCostFraction, crystal*teleportCostFraction, electricity*teleportCostFraction, dollars*teleportCostFraction

	cardText := "🌀 " + htmlBold("CONFIRM TELEPORT") + "\n" + divider + "\n" +
		fmt.Sprintf("This will relocate your outpost to a fresh random coordinate and cost %s%% of your CURRENT holdings:\n", htmlCode(fmt.Sprintf("%.0f", teleportCostFraction*100))) +
		fmt.Sprintf("⚙️ %s Scrap  🔩 %s Metal  🔮 %s Crystal\n", htmlCode(fmt.Sprintf("%.0f", scrapCost)), htmlCode(fmt.Sprintf("%.0f", metalCost)), htmlCode(fmt.Sprintf("%.0f", crystalCost))) +
		fmt.Sprintf("⚡ %s Electricity  💵 $%s\n\n", htmlCode(fmt.Sprintf("%.0f", electricityCost)), htmlCode(fmt.Sprintf("%.0f", dollarsCost))) +
		htmlItalic("Why so steep: relocating abandons every scout/raider's existing fix on this outpost's position for free - a cheap Teleport would let anyone dodge an incoming raid or hide from a scout on a whim. The cost scales with your own holdings so it stays meaningful at any stage, not just early game.") + "\n" +
		divider

	selector := &telebot.ReplyMarkup{}
	btnConfirm := keyboards.Styled(selector.Data("✅ Confirm Teleport", "teleport_c"), keyboards.StyleSuccess)
	btnCancel := keyboards.Styled(selector.Data("❌ Cancel", "teleport_x"), keyboards.StyleDanger)
	return keyboards.SendStyled(c, cardText, [][]keyboards.StyledBtn{{btnCancel, btnConfirm}})
}

func (h *JobsHandler) HandleTeleportConfirmCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid confirmation."})
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	message, err := h.doTeleport(ctx, campID)
	if err != nil {
		return c.Edit(message)
	}
	return c.Edit(message, telebot.ModeHTML)
}

func (h *JobsHandler) HandleTeleportCancelCallback(c telebot.Context) error {
	_ = c.Edit("🌀 Teleport cancelled - nothing was charged, your outpost hasn't moved.")
	return c.Respond()
}

func (h *JobsHandler) doTeleport(ctx context.Context, campID string) (string, error) {
	var lastTeleport sql.NullTime
	_ = h.DB.QueryRowContext(ctx, "SELECT last_teleport_at FROM encampments WHERE id = $1", campID).Scan(&lastTeleport)
	if lastTeleport.Valid && time.Since(lastTeleport.Time) < teleportCooldown {
		remaining := teleportCooldown - time.Since(lastTeleport.Time)
		return fmt.Sprintf("⏳ Teleport is on cooldown for another %.1f hours.", remaining.Hours()), errors.New("on cooldown")
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return "⚠️ Teleport failed.", err
	}
	defer tx.Rollback()

	var scrap, metal, crystal, electricity, dollars float64
	if err := tx.QueryRowContext(ctx, "SELECT scrap, metal, crystal, electricity, dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap, &metal, &crystal, &electricity, &dollars); err != nil {
		return "⚠️ Error reading your resources.", err
	}
	scrapCost, metalCost, crystalCost, electricityCost, dollarsCost :=
		scrap*teleportCostFraction, metal*teleportCostFraction, crystal*teleportCostFraction, electricity*teleportCostFraction, dollars*teleportCostFraction

	newContinent := randomContinent(rand.New(rand.NewSource(time.Now().UnixNano())))
	newCoordID, newX, newY, err := allocateCoordinate(ctx, tx, time.Now().UnixNano(), newContinent)
	if err != nil {
		return "⚠️ Error finding new coordinates.", err
	}

	if _, err := tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1, metal = metal - $2, crystal = crystal - $3, electricity = electricity - $4, dollars = dollars - $5 WHERE encampment_id = $6",
		scrapCost, metalCost, crystalCost, electricityCost, dollarsCost, campID); err != nil {
		return "⚠️ Error deducting Teleport's cost.", err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE encampments SET coordinate_id = $1, last_teleport_at = CURRENT_TIMESTAMP WHERE id = $2", newCoordID, campID); err != nil {
		return "⚠️ Error relocating.", err
	}

	if err := tx.Commit(); err != nil {
		return "⚠️ Error completing teleport.", err
	}

	return fmt.Sprintf("🌀✨ %s Your outpost now stands near %s, at a cost of %s Scrap, %s Metal, %s Crystal, %s Electricity, and $%s.",
		htmlBold("TELEPORT COMPLETE!"), locationDescriptor(newX, newY, newContinent),
		htmlCode(fmt.Sprintf("%.0f", scrapCost)), htmlCode(fmt.Sprintf("%.0f", metalCost)), htmlCode(fmt.Sprintf("%.0f", crystalCost)),
		htmlCode(fmt.Sprintf("%.0f", electricityCost)), htmlCode(fmt.Sprintf("%.0f", dollarsCost))), nil
}

const (
	ghostProtocolCooldown     = 90 * 24 * time.Hour
	ghostProtocolCostFraction = 0.50
)

func (h *JobsHandler) HandleGhostProtocol(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	var lastGhost sql.NullTime
	_ = h.DB.QueryRowContext(ctx, "SELECT last_ghost_protocol_at FROM encampments WHERE id = $1", campID).Scan(&lastGhost)
	if lastGhost.Valid && time.Since(lastGhost.Time) < ghostProtocolCooldown {
		remaining := ghostProtocolCooldown - time.Since(lastGhost.Time)
		return c.Send(fmt.Sprintf("⏳ Ghost Protocol is on cooldown for another %.0f days.", remaining.Hours()/24))
	}

	var scrap, metal, crystal, dollars float64
	_ = h.DB.QueryRowContext(ctx, "SELECT scrap, metal, crystal, dollars FROM resources WHERE encampment_id = $1", campID).Scan(&scrap, &metal, &crystal, &dollars)
	scrapCost, metalCost, crystalCost, dollarsCost := scrap*ghostProtocolCostFraction, metal*ghostProtocolCostFraction, crystal*ghostProtocolCostFraction, dollars*ghostProtocolCostFraction

	cardText := "👻 " + htmlBold("CONFIRM GHOST PROTOCOL") + "\n" + divider + "\n" +
		fmt.Sprintf("This will relocate your outpost AND erase every scout/raider's lock on your current position - permanently, until they rediscover you. Cost: %s%% of your CURRENT holdings:\n", htmlCode(fmt.Sprintf("%.0f", ghostProtocolCostFraction*100))) +
		fmt.Sprintf("⚙️ %s Scrap  🔩 %s Metal  🔮 %s Crystal  💵 $%s\n\n", htmlCode(fmt.Sprintf("%.0f", scrapCost)), htmlCode(fmt.Sprintf("%.0f", metalCost)), htmlCode(fmt.Sprintf("%.0f", crystalCost)), htmlCode(fmt.Sprintf("%.0f", dollarsCost))) +
		htmlItalic("Why so severe: this is a 90-day-cooldown emergency measure, not a routine hop - it wipes out an attacker's entire scouting investment in you for free, so the cost has to genuinely hurt or it would trivialize raid-avoidance for anyone who can afford it.") + "\n" +
		divider

	selector := &telebot.ReplyMarkup{}
	btnConfirm := keyboards.Styled(selector.Data("✅ Confirm Ghost Protocol", "ghost_c"), keyboards.StyleSuccess)
	btnCancel := keyboards.Styled(selector.Data("❌ Cancel", "ghost_x"), keyboards.StyleDanger)
	return keyboards.SendStyled(c, cardText, [][]keyboards.StyledBtn{{btnCancel, btnConfirm}})
}

func (h *JobsHandler) HandleGhostProtocolConfirmCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid confirmation."})
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	message, err := h.doGhostProtocol(ctx, campID)
	if err != nil {
		return c.Edit(message)
	}
	return c.Edit(message, telebot.ModeHTML)
}

func (h *JobsHandler) HandleGhostProtocolCancelCallback(c telebot.Context) error {
	_ = c.Edit("👻 Ghost Protocol cancelled - nothing was charged, your outpost hasn't moved, and no intel locks were cleared.")
	return c.Respond()
}

func (h *JobsHandler) doGhostProtocol(ctx context.Context, campID string) (string, error) {
	var lastGhost sql.NullTime
	_ = h.DB.QueryRowContext(ctx, "SELECT last_ghost_protocol_at FROM encampments WHERE id = $1", campID).Scan(&lastGhost)
	if lastGhost.Valid && time.Since(lastGhost.Time) < ghostProtocolCooldown {
		remaining := ghostProtocolCooldown - time.Since(lastGhost.Time)
		return fmt.Sprintf("⏳ Ghost Protocol is on cooldown for another %.0f days.", remaining.Hours()/24), errors.New("on cooldown")
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return "⚠️ Ghost Protocol failed.", err
	}
	defer tx.Rollback()

	var scrap, metal, crystal, dollars float64
	err = tx.QueryRowContext(ctx, "SELECT scrap, metal, crystal, dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap, &metal, &crystal, &dollars)
	if err != nil {
		return "⚠️ Error reading your resources.", err
	}
	scrapCost, metalCost, crystalCost, dollarsCost := scrap*ghostProtocolCostFraction, metal*ghostProtocolCostFraction, crystal*ghostProtocolCostFraction, dollars*ghostProtocolCostFraction

	newContinent := randomContinent(rand.New(rand.NewSource(time.Now().UnixNano())))
	newCoordID, newX, newY, err := allocateCoordinate(ctx, tx, time.Now().UnixNano(), newContinent)
	if err != nil {
		return "⚠️ Error finding new coordinates.", err
	}

	if _, err := tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1, metal = metal - $2, crystal = crystal - $3, dollars = dollars - $4 WHERE encampment_id = $5",
		scrapCost, metalCost, crystalCost, dollarsCost, campID); err != nil {
		return "⚠️ Error deducting Ghost Protocol's cost.", err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE encampments SET coordinate_id = $1, last_ghost_protocol_at = CURRENT_TIMESTAMP WHERE id = $2", newCoordID, campID); err != nil {
		return "⚠️ Error relocating.", err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM known_locations WHERE target_encampment_id = $1", campID); err != nil {
		return "⚠️ Error clearing intel locks on your position.", err
	}

	if err := tx.Commit(); err != nil {
		return "⚠️ Error completing Ghost Protocol.", err
	}

	return fmt.Sprintf("👻 %s Every scout and raider who'd locked your position has lost that lock. Your outpost now stands near %s, at a cost of %s Scrap, %s Metal, %s Crystal, and %s Dollars.",
		htmlBold("GHOST PROTOCOL EXECUTED"), locationDescriptor(newX, newY, newContinent),
		htmlCode(fmt.Sprintf("%.0f", scrapCost)), htmlCode(fmt.Sprintf("%.0f", metalCost)),
		htmlCode(fmt.Sprintf("%.0f", crystalCost)), htmlCode(fmt.Sprintf("%.0f", dollarsCost))), nil
}

func (h *JobsHandler) HandleOrbitalManeuver(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	if _, err := h.myCamp(ctx, sender.ID); err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.JobsNavigation())
	}

	return sendConfirmCard(c, "🛰️ "+htmlBold("CONFIRM ORBITAL MANEUVER"),
		fmt.Sprintf("Cost: %s Electricity. +30%% defense rating for the next 2 hours.", htmlCode("400")),
		"This spends real Electricity for a time-limited defensive buff - confirming first avoids burning it before you actually need the extra defense.",
		"orbitalmvr")
}

func (h *JobsHandler) HandleOrbitalManeuverConfirmCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid confirmation."})
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	message, err := h.doOrbitalManeuver(ctx, campID)
	if err != nil {
		return c.Edit(message)
	}
	return c.Edit(message, telebot.ModeHTML)
}

func (h *JobsHandler) HandleOrbitalManeuverCancelCallback(c telebot.Context) error {
	return sendCancelledCard(c, "🛰️ Orbital Maneuver")
}

func (h *JobsHandler) doOrbitalManeuver(ctx context.Context, campID string) (string, error) {
	const cost = 400.0
	const buffDuration = 2 * time.Hour

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return "⚠️ Maneuver failed.", err
	}
	defer tx.Rollback()

	var electricity float64
	_ = tx.QueryRowContext(ctx, "SELECT electricity FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&electricity)
	if electricity < cost {
		return fmt.Sprintf("❌ %s Need %s.", htmlBold("Insufficient Electricity!"), htmlCode(fmt.Sprintf("%.0f", cost))), errors.New("insufficient electricity")
	}

	buffUntil := time.Now().UTC().Add(buffDuration)
	if _, err := tx.ExecContext(ctx, "UPDATE resources SET electricity = electricity - $1 WHERE encampment_id = $2", cost, campID); err != nil {
		return "⚠️ Error deducting maneuver's cost.", err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE encampments SET orbital_buff_until = $1 WHERE id = $2", buffUntil, campID); err != nil {
		return "⚠️ Error activating maneuver.", err
	}

	if err := tx.Commit(); err != nil {
		return "⚠️ Error activating maneuver.", err
	}

	return fmt.Sprintf("🛰️✅ %s +30%% defense rating for the next %s.", htmlBold("ORBITAL MANEUVER ACTIVE!"), htmlCode(fmt.Sprintf("%.0f minutes", buffDuration.Minutes()))), nil
}

func (h *JobsHandler) HandleRepairUnits(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	if _, err := h.myCamp(ctx, sender.ID); err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.JobsNavigation())
	}

	return sendConfirmCard(c, "🔧 "+htmlBold("CONFIRM FIELD REPAIRS"),
		fmt.Sprintf("Cost: %s Scrap. Restores 5 Soldiers to fighting condition.", htmlCode("200")),
		"This spends real Scrap for a small, immediate gain - confirming first avoids spending it reflexively before you've decided you actually need more Soldiers right now.",
		"repairunits")
}

func (h *JobsHandler) HandleRepairUnitsConfirmCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid confirmation."})
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	message, err := h.doRepairUnits(ctx, campID)
	if err != nil {
		return c.Edit(message)
	}
	return c.Edit(message, telebot.ModeHTML)
}

func (h *JobsHandler) HandleRepairUnitsCancelCallback(c telebot.Context) error {
	return sendCancelledCard(c, "🔧 Field Repairs")
}

func (h *JobsHandler) doRepairUnits(ctx context.Context, campID string) (string, error) {
	const cost = 200.0
	const repaired = 5

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return "⚠️ Repair failed.", err
	}
	defer tx.Rollback()

	var scrap float64
	_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap)
	if scrap < cost {
		return fmt.Sprintf("❌ %s Need %s.", htmlBold("Insufficient Scrap!"), htmlCode(fmt.Sprintf("%.0f", cost))), errors.New("insufficient scrap")
	}

	if _, err := tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", cost, campID); err != nil {
		return "⚠️ Error deducting repair's cost.", err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE workshop_inventory SET soldiers = soldiers + $1 WHERE encampment_id = $2", repaired, campID); err != nil {
		return "⚠️ Error repairing units.", err
	}

	if err := tx.Commit(); err != nil {
		return "⚠️ Error repairing units.", err
	}

	return fmt.Sprintf("🔧✅ %s +%d Soldiers restored to fighting condition.", htmlBold("FIELD REPAIRS COMPLETE!"), repaired), nil
}

func (h *JobsHandler) HandleRepairBuildings(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.JobsNavigation())
	}

	var readyAt time.Time
	if err := h.DB.QueryRowContext(ctx, "SELECT upgrade_ready_at FROM modules WHERE encampment_id = $1 AND is_upgrading = TRUE ORDER BY upgrade_ready_at ASC LIMIT 1", campID).Scan(&readyAt); err != nil {
		return c.Send("❌ No buildings currently under construction to repair/rush.", keyboards.JobsNavigation())
	}

	return sendConfirmCard(c, "🏗️ "+htmlBold("CONFIRM CONSTRUCTION RUSH"),
		fmt.Sprintf("Cost: %s Scrap. Cuts your active building upgrade's remaining time in half.", htmlCode("150")),
		"This spends real Scrap for a one-time timing boost - confirming first avoids burning it on an upgrade you'd rather let finish naturally.",
		"repairbldg")
}

func (h *JobsHandler) HandleRepairBuildingsConfirmCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid confirmation."})
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	message, err := h.doRepairBuildings(ctx, campID)
	if err != nil {
		return c.Edit(message)
	}
	return c.Edit(message, telebot.ModeHTML)
}

func (h *JobsHandler) HandleRepairBuildingsCancelCallback(c telebot.Context) error {
	return sendCancelledCard(c, "🏗️ Construction Rush")
}

func (h *JobsHandler) doRepairBuildings(ctx context.Context, campID string) (string, error) {
	const cost = 150.0

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return "⚠️ Repair failed.", err
	}
	defer tx.Rollback()

	var moduleID string
	var readyAt time.Time
	err = tx.QueryRowContext(ctx, "SELECT id, upgrade_ready_at FROM modules WHERE encampment_id = $1 AND is_upgrading = TRUE ORDER BY upgrade_ready_at ASC LIMIT 1 FOR UPDATE", campID).Scan(&moduleID, &readyAt)
	if err != nil {
		return "❌ No buildings currently under construction to repair/rush.", err
	}

	var scrap float64
	_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap)
	if scrap < cost {
		return fmt.Sprintf("❌ %s Need %s.", htmlBold("Insufficient Scrap!"), htmlCode(fmt.Sprintf("%.0f", cost))), errors.New("insufficient scrap")
	}

	remaining := time.Until(readyAt)
	newReady := readyAt.Add(-remaining / 2)

	if _, err := tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", cost, campID); err != nil {
		return "⚠️ Error deducting rush's cost.", err
	}
	if _, err := tx.ExecContext(ctx, "UPDATE modules SET upgrade_ready_at = $1 WHERE id = $2", newReady, moduleID); err != nil {
		return "⚠️ Error rushing construction.", err
	}

	if err := tx.Commit(); err != nil {
		return "⚠️ Error rushing construction.", err
	}

	return "🏗️✅ " + htmlBold("CONSTRUCTION CREW DEPLOYED!") + " Remaining build time on your active upgrade cut in half.", nil
}

func (h *JobsHandler) HandleGatherSunlight(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	campID, err := h.myCamp(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.JobsNavigation())
	}

	var lastSunlight sql.NullTime
	_ = h.DB.QueryRowContext(ctx, "SELECT last_sunlight_at FROM encampments WHERE id = $1", campID).Scan(&lastSunlight)
	if lastSunlight.Valid && time.Since(lastSunlight.Time) < 30*time.Minute {
		remaining := 30*time.Minute - time.Since(lastSunlight.Time)
		return c.Send(fmt.Sprintf("⏳ Solar panels still recharging - %.0f minutes left.", remaining.Minutes()), keyboards.JobsNavigation())
	}

	const gain = 150.0
	var curElectricity float64
	_ = h.DB.QueryRowContext(ctx, "SELECT electricity FROM resources WHERE encampment_id = $1", campID).Scan(&curElectricity)
	storageCap := storagecap.CapFor(ctx, h.DB, campID)
	newElectricity, _ := storagecap.Clamp(curElectricity, gain, storageCap)
	_, _ = h.DB.ExecContext(ctx, "UPDATE resources SET electricity = $1 WHERE encampment_id = $2", newElectricity, campID)
	_, _ = h.DB.ExecContext(ctx, "UPDATE encampments SET last_sunlight_at = CURRENT_TIMESTAMP WHERE id = $1", campID)

	return c.Send(fmt.Sprintf("☀️✅ %s +%s Electricity harvested manually.", htmlBold("SUNLIGHT GATHERED!"), htmlCode(fmt.Sprintf("%.0f", gain))), telebot.ModeHTML, keyboards.JobsNavigation())
}

func (h *JobsHandler) HandleManualScanAlias(c telebot.Context) error {
	return c.Send("🔍 Manual Scan: use /scout [username] to instantly look up a rival's basic intel.", keyboards.JobsNavigation())
}

func (h *JobsHandler) HandleAutoScanAlias(c telebot.Context) error {
	return c.Send("📡 Automatic Scan: use /autoscan to toggle periodic automated recon reports.", keyboards.JobsNavigation())
}

func (h *JobsHandler) HandleAdvancedScanAlias(c telebot.Context) error {
	return c.Send("🛰️ Advanced Scan: after a /scout lookup, tap 'Intercept Signal' to launch a full satellite recon mission with real travel time and deeper intel.", keyboards.JobsNavigation())
}

func (h *JobsHandler) HandlePublishTradeAlias(c telebot.Context) error {
	return c.Send("💱 Publish Trade: open the Market Exchange from /econ ➜ Market Exchange to list Metal or Crystal for sale to other survivors.", keyboards.JobsNavigation())
}
