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

// sendConfirmCard renders the standard Confirm/Cancel prompt card used
// by every resource-spending job below (2026-08-05, by explicit
// project-owner direction - "let there be confirm buttons for all
// jobs... in a situation whereby you mistakenly pressed teleport, it
// will just automatically go"). actionKey is the short callback-data
// prefix registered in cmd/bot/main.go as "\f"+actionKey+"_c" (confirm)
// and "\f"+actionKey+"_x" (cancel) - see HandleTeleport/
// HandleGhostProtocol above for the pattern this generalizes.
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

// sendCancelledCard is the standard reply for every "❌ Cancel" callback
// below - nothing was ever charged or executed, so this just closes out
// the card with a plain confirmation of that.
func sendCancelledCard(c telebot.Context, jobLabel string) error {
	_ = c.Edit(fmt.Sprintf("%s cancelled - nothing was charged, nothing changed.", jobLabel))
	return c.Respond()
}

// HandleJobsPanel is the "🛠️ Odd Jobs" mother-keyboard entry point -
// plants the JobsNavigation child keyboard (via sendPanelWithNav, see
// navhelper.go's doc comment for why a plain c.Send with two reply-markup
// arguments would silently drop one of them) and shows a cost/cooldown
// reference summary so a player can decide what to run without needing
// to trigger each job blind. Every button on the JobsNavigation keyboard
// fires its job directly - see that function's doc comment in
// internal/bot/keyboards/navigation.go for why there's no further
// sub-panel here. As of 2026-08-05, every resource-spending job (all
// except Gather Sunlight, which only ever gains resources with no
// downside to confirm against, and the pure-informational scan/trade
// aliases) shows its own inline Confirm/Cancel card before actually
// executing - see HandleTeleport's doc comment for the full rationale.
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

// hyperspeedTarget is the nearest-resolving accelerable mission - either
// an outbound/returning raid (raids.resolve_time) or a scout party
// already on its way home (scout_missions.return_eta, phase='returning').
// 2026-08-06 direct request: "job HyperSpeed should be able to work to
// HyperSpeed long range scouting units returning home" - previously
// HyperSpeed only ever looked at the raids table, so a player with no
// active raid but a scout party inbound had nothing to accelerate even
// though one clearly existed. A scout mission still in its 'searching'
// phase has no fixed ETA to cut in half (that's the whole point of an
// open-ended search - see scoutMissionPingInterval's doc comment in
// internal/engine/tick/scoutmissions.go), so only 'returning' qualifies,
// mirroring how a raid only qualifies once it has a resolve_time at all.
type hyperspeedTarget struct {
	kind        string // "raid" or "scout"
	id          string
	resolveTime time.Time
}

// hyperspeedQuerier is satisfied by both *sql.DB (preview) and *sql.Tx
// (execution, where the row must be locked FOR UPDATE) - findHyperSpeedTarget
// runs the same two candidate queries either way and returns whichever
// resolves soonest.
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

// HandleHyperSpeed (/newjobhyperspeed) shaves time off your earliest
// active raid's remaining travel, matching the SpaceHunt tip about
// launching HyperSpeed before departing a raid.
// HandleHyperSpeed (/newjobhyperspeed) shows a Confirm/Cancel preview
// before spending Electricity to halve your nearest active mission's
// remaining time - 2026-08-05 fix, same rationale as HandleTeleport.
// Since 2026-08-06, "nearest active mission" spans both raids and
// returning scout parties - see findHyperSpeedTarget's doc comment.
// See doHyperSpeed for the execution core.
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

// HandleHyperSpeedConfirmCallback fires when a player taps "✅
// Confirm" on the HyperSpeed card. Re-derives the caller's encampment
// server-side and calls doHyperSpeed, which re-validates funds and the
// target mission fresh.
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

// HandleHyperSpeedCancelCallback fires when a player taps "❌ Cancel".
func (h *JobsHandler) HandleHyperSpeedCancelCallback(c telebot.Context) error {
	return sendCancelledCard(c, "🚀 HyperSpeed")
}

// doHyperSpeed is the testable core of HandleHyperSpeedConfirmCallback -
// unchanged logic from the pre-2026-08-05 HandleHyperSpeed, just moved
// behind the new confirm callback and returning its result instead of
// sending it directly. Since 2026-08-06 it picks the nearest-resolving
// target across both raids and returning scout missions (see
// findHyperSpeedTarget) and updates whichever table that target came
// from.
func (h *JobsHandler) doHyperSpeed(ctx context.Context, campID string) (string, error) {
	const cost = 300.0 // Electricity

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
	newResolve := target.resolveTime.Add(-remaining / 2) // cuts remaining time in half

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

// HandleExtendPlanet (/newjobextendplanet) shows a Confirm/Cancel
// preview before permanently increasing storage capacity - 2026-08-05
// fix, same rationale as HandleTeleport. A real, growing investment
// rather than a one-time bonus, and cost grows every level, so
// confirming first matters more here as a player progresses. See
// doExtendPlanet for the execution core.
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

// HandleExtendPlanetConfirmCallback fires when a player taps "✅
// Confirm" on the Extend Planet card.
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

// HandleExtendPlanetCancelCallback fires when a player taps "❌ Cancel".
func (h *JobsHandler) HandleExtendPlanetCancelCallback(c telebot.Context) error {
	return sendCancelledCard(c, "🌍 Planet Extension")
}

// doExtendPlanet is the testable core of HandleExtendPlanetConfirmCallback -
// unchanged logic from the pre-2026-08-05 HandleExtendPlanet, just moved
// behind the new confirm callback.
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

// teleportCooldown and teleportCostFraction: 2026-08-05 rebalance, by
// explicit project-owner direction ("increase the price for Teleporting
// ... to almost unaffordable, make it super costly, make players see
// why"). Teleport used to cost a flat 1000 Electricity - negligible
// against typical late-game balances (a live report showed a player
// sitting on 7M+ Electricity, i.e. the "cost" was ~0.014% of holdings
// and functionally free). Now priced the same way as Ghost Protocol
// below - a FRACTION of current holdings across five resources, so it
// scales with wealth instead of going stale as the economy grows - but
// at a third of Ghost Protocol's severity, preserving the intentional
// tier gap between "frequent utility hop" (24h cooldown, relocate
// only) and "rare emergency measure" (90-day cooldown, relocate AND
// erase every intel lock on this base).
const (
	teleportCooldown     = 24 * time.Hour
	teleportCostFraction = 0.15
)

// HandleTeleport (/newjobteleport) shows a cost-preview Confirm/Cancel
// card rather than relocating immediately - 2026-08-05 fix, by explicit
// project-owner direction ("let there be confirm buttons for all jobs
// ... in a situation whereby you mistakenly pressed teleport, it will
// just automatically go"). Nothing is charged or moved until
// HandleTeleportConfirmCallback fires. See doTeleport for the actual
// execution core.
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

// HandleTeleportConfirmCallback fires when a player taps "✅ Confirm
// Teleport" on the card HandleTeleport rendered. Re-derives the
// caller's encampment server-side (never trusts anything from
// callback_data for this) and re-validates the cooldown fresh -
// doTeleport re-checks it independently of HandleTeleport's earlier
// check, since time may have passed (or a second Teleport already
// happened) between the prompt being shown and Confirm being tapped.
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

// HandleTeleportCancelCallback fires when a player taps "❌ Cancel" -
// nothing was ever charged or moved, so this just closes out the card.
func (h *JobsHandler) HandleTeleportCancelCallback(c telebot.Context) error {
	_ = c.Edit("🌀 Teleport cancelled - nothing was charged, your outpost hasn't moved.")
	return c.Respond()
}

// doTeleport is the testable core of HandleTeleportConfirmCallback,
// following the same pattern as doGhostProtocol below: no
// telebot.Context dependency, so it can be exercised directly against a
// real database in tests. Returns the message to send and, on failure,
// a non-nil error (in which case the message is plain-text, not HTML).
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

// ghostProtocolCooldown and ghostProtocolCostFraction are deliberately
// severe compared to /newjobteleport's cheap, frequent utility cost -
// per the project owner's own words ("very very very costly," "once in
// 3 months"). Cost is a fraction of *current* holdings rather than a
// fixed absolute number, so it stays meaningfully painful regardless of
// where the live economy's typical balances land - there's no historical
// usage data yet to calibrate a fixed number against (see
// AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 3.4 and its "Open
// questions" item 1). Treat both as tunable constants to revisit after a
// few weeks of real usage, same spirit as aidecisions.go's
// aiFairnessNormalBandAbove/aiOverdueRaidThreshold constants.
const (
	ghostProtocolCooldown     = 90 * 24 * time.Hour
	ghostProtocolCostFraction = 0.50
)

// HandleGhostProtocol (/ghostprotocol) shows a cost-preview
// Confirm/Cancel card rather than executing immediately - 2026-08-05
// fix, same rationale as HandleTeleport above, arguably even more
// important here given how much more severe and rarer (90-day
// cooldown) this action is. Nothing is charged, relocated, or erased
// until HandleGhostProtocolConfirmCallback fires. See doGhostProtocol
// for the actual execution core (unchanged).
//
// This is a separate, far more severe action than /newjobteleport -
// see AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 3.4 for why
// the existing cheap/frequent teleport was deliberately NOT
// repurposed for this. In addition to relocating (reusing
// /newjobteleport's random-coordinate logic), this deletes every
// known_locations row where this encampment is the target - every
// scout/attacker who'd locked this base's position loses that lock
// and must rediscover it from scratch. encampment_discoveries (the
// permanent "have you ever heard of this entity" relationship) is
// untouched - only the coordinate lock resets.
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

// HandleGhostProtocolConfirmCallback fires when a player taps "✅
// Confirm Ghost Protocol" on the card HandleGhostProtocol rendered.
// Re-derives the caller's encampment server-side and calls the
// unchanged doGhostProtocol core, which re-validates the cooldown
// fresh - see HandleTeleportConfirmCallback's doc comment for why that
// re-check matters even though HandleGhostProtocol already checked it
// once when the card was shown.
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

// HandleGhostProtocolCancelCallback fires when a player taps "❌
// Cancel" - nothing was ever charged, relocated, or erased, so this
// just closes out the card.
func (h *JobsHandler) HandleGhostProtocolCancelCallback(c telebot.Context) error {
	_ = c.Edit("👻 Ghost Protocol cancelled - nothing was charged, your outpost hasn't moved, and no intel locks were cleared.")
	return c.Respond()
}

// doGhostProtocol is the testable core of HandleGhostProtocolConfirmCallback,
// following the same pattern as admin.go's doSetTaxRate: no telebot.Context
// dependency, so it can be exercised directly against a real database in
// tests. Returns the message to send and, on failure, a non-nil error
// (in which case the message is a plain-text failure notice, not HTML).
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

// HandleOrbitalManeuver (/newjoborbitalmaneuver) grants a temporary
// defense rating boost.
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

// HandleOrbitalManeuverConfirmCallback fires when a player taps "✅
// Confirm" on the Orbital Maneuver card.
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

// HandleOrbitalManeuverCancelCallback fires when a player taps "❌ Cancel".
func (h *JobsHandler) HandleOrbitalManeuverCancelCallback(c telebot.Context) error {
	return sendCancelledCard(c, "🛰️ Orbital Maneuver")
}

// doOrbitalManeuver is the testable core of
// HandleOrbitalManeuverConfirmCallback - unchanged logic from the
// pre-2026-08-05 HandleOrbitalManeuver, just moved behind the new
// confirm callback.
func (h *JobsHandler) doOrbitalManeuver(ctx context.Context, campID string) (string, error) {
	const cost = 400.0 // Electricity
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

// HandleRepairUnits (/newjobrepairunits) shows a Confirm/Cancel preview
// before field-repairing a small batch of Soldiers for a Scrap cost -
// 2026-08-05 fix, same rationale as HandleTeleport. See doRepairUnits
// for the execution core.
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

// HandleRepairUnitsConfirmCallback fires when a player taps "✅
// Confirm" on the Field Repairs card.
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

// HandleRepairUnitsCancelCallback fires when a player taps "❌ Cancel".
func (h *JobsHandler) HandleRepairUnitsCancelCallback(c telebot.Context) error {
	return sendCancelledCard(c, "🔧 Field Repairs")
}

// doRepairUnits is the testable core of HandleRepairUnitsConfirmCallback -
// unchanged logic from the pre-2026-08-05 HandleRepairUnits, just moved
// behind the new confirm callback.
func (h *JobsHandler) doRepairUnits(ctx context.Context, campID string) (string, error) {
	const cost = 200.0 // Scrap, repairs 5 Soldiers
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

// HandleRepairBuildings (/newjobrepairbuildings) shows a Confirm/Cancel
// preview before spending Scrap to rush any in-progress building
// upgrade - 2026-08-05 fix, same rationale as HandleTeleport. See
// doRepairBuildings for the execution core.
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

// HandleRepairBuildingsConfirmCallback fires when a player taps "✅
// Confirm" on the Construction Rush card.
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

// HandleRepairBuildingsCancelCallback fires when a player taps "❌ Cancel".
func (h *JobsHandler) HandleRepairBuildingsCancelCallback(c telebot.Context) error {
	return sendCancelledCard(c, "🏗️ Construction Rush")
}

// doRepairBuildings is the testable core of
// HandleRepairBuildingsConfirmCallback - unchanged logic from the
// pre-2026-08-05 HandleRepairBuildings, just moved behind the new
// confirm callback.
func (h *JobsHandler) doRepairBuildings(ctx context.Context, campID string) (string, error) {
	const cost = 150.0 // Scrap

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

// HandleGatherSunlight (/newjobgathersunlight) - instant manual burst of
// Electricity, on a short cooldown.
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

// ── Scan command aliases for full command-name parity ──────────────────

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
