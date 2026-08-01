package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type ScoutMissionsHandler struct {
	DB *sql.DB
}

func NewScoutMissionsHandler(db *sql.DB) *ScoutMissionsHandler {
	return &ScoutMissionsHandler{DB: db}
}

// scoutQuickDispatchCounts are the one-tap quantity options shown on the
// panel's inline keyboard - common round numbers rather than an exhaustive
// picker, matching the "a handful of good defaults, not a slider" spirit
// of every other quantity choice in this game (see e.g. the Workshop's
// batch-production buttons).
var scoutQuickDispatchCounts = []int{1, 5, 10, 25}

// maxConcurrentScoutMissions caps how many independent scout_missions rows
// an encampment can have active (phase 'searching' or 'returning') at
// once. Originally 1 (a hard EXISTS gate); raised to 3 per player request
// and AI_AND_SCOUTING_EXPANSION_PLAN.md item 3 - each mission still finds
// its own target and returns on its own timeline independently, since the
// tick-side processing in internal/engine/tick/scoutmissions.go already
// operates per-row (queries every mission in a phase, not "the"
// encampment's mission), so raising this cap needed no tick-engine
// changes, only the dispatch gate and the status rendering below.
const maxConcurrentScoutMissions = 3

// doDispatchScoutMission is the testable core of HandleDispatchScoutMission
// and HandleScoutDispatchCallback - no telebot.Context dependency, so both
// the slash command and the inline quantity buttons share one code path
// (and one set of tests), matching admin.go's doSetTaxRate / jobs.go's
// doGhostProtocol convention. Returns the HTML-formatted message to show
// and, on failure, a non-nil error alongside a plain-text failure notice.
func (h *ScoutMissionsHandler) doDispatchScoutMission(ctx context.Context, campID string, count int) (string, error) {
	if count <= 0 {
		return "🔭 Commit at least 1 scout.", errors.New("count must be positive")
	}

	var availableScouts int
	if err := h.DB.QueryRowContext(ctx, "SELECT COALESCE(scouts, 0) FROM workshop_inventory WHERE encampment_id = $1", campID).Scan(&availableScouts); err != nil {
		return "⚠️ Error reading your Scout Walker roster.", err
	}
	if count > availableScouts {
		return fmt.Sprintf("❌ You only have %s available Scout Walkers (some may already be committed elsewhere).", htmlCode(fmt.Sprintf("%d", availableScouts))), errors.New("insufficient scouts")
	}

	var activeMissions int
	if err := h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM scout_missions WHERE encampment_id = $1 AND phase IN ('searching', 'returning')", campID).Scan(&activeMissions); err != nil {
		return "⚠️ Error checking your active scout parties.", err
	}
	if activeMissions >= maxConcurrentScoutMissions {
		return fmt.Sprintf("❌ You already have %s scout parties out - that's the maximum at once. Check status below and wait for one to return first.", htmlCode(fmt.Sprintf("%d", maxConcurrentScoutMissions))), errors.New("max concurrent scout missions reached")
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return "⚠️ Error dispatching scout party.", err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "UPDATE workshop_inventory SET scouts = scouts - $1 WHERE encampment_id = $2", count, campID); err != nil {
		return "⚠️ Error committing scouts.", err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO scout_missions (encampment_id, scouts_committed) VALUES ($1, $2)", campID, count); err != nil {
		return "⚠️ Error creating scout mission.", err
	}
	if err := tx.Commit(); err != nil {
		return "⚠️ Error dispatching scout party.", err
	}

	return "🔭 " + htmlBold("SCOUT PARTY DISPATCHED") + "\n" + divider + "\n" +
		fmt.Sprintf("%s Scout Walkers committed\n", htmlCode(fmt.Sprintf("%d", count))) +
		htmlItalic("No fixed ETA - they'll search until they find something, and you'll hear from them periodically along the way.") + "\n" +
		divider, nil
}

// HandleDispatchScoutMission (/scoutmission <count>) is the slash-command
// entry point - kept for players who prefer typing, alongside the wired
// panel below.
func (h *ScoutMissionsHandler) HandleDispatchScoutMission(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	count, err := strconv.Atoi(strings.TrimSpace(c.Message().Payload))
	if err != nil || count <= 0 {
		return c.Send("🔭 Usage: /scoutmission <number of scouts to commit>, e.g. /scoutmission 5")
	}

	campID, err := h.myScoutCamp(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	message, err := h.doDispatchScoutMission(ctx, campID, count)
	if err != nil {
		return c.Send(message, telebot.ModeHTML)
	}
	return c.Send(message, telebot.ModeHTML, keyboards.CombatNavigation())
}

// HandleScoutDispatchCallback fires when a player taps one of the panel's
// quantity buttons. Mirrors HandleDispatchExpeditionCallback's
// respond-then-redraw pattern in exploration.go: a short toast
// acknowledgment, then the panel itself resent with the now-updated
// mission state.
func (h *ScoutMissionsHandler) HandleScoutDispatchCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 1 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid selection."})
	}
	count, err := strconv.Atoi(c.Args()[0])
	if err != nil || count <= 0 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid scout count."})
	}

	campID, err := h.myScoutCamp(ctx, sender.ID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	_, dispatchErr := h.doDispatchScoutMission(ctx, campID, count)
	if dispatchErr != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Could not dispatch - see the panel for details."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("🔭 %d Scout Walkers dispatched!", count)})
	return h.HandleScoutPanel(c)
}

// myScoutCamp resolves the caller's outpost ID.
func (h *ScoutMissionsHandler) myScoutCamp(ctx context.Context, userID int64) (string, error) {
	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", userID).Scan(&campID)
	return campID, err
}

// HandleScoutPanel (button-driven, and also useful as a direct command)
// is the wired "mother" screen for long-range scouting - the entry point
// missing before this pass: dispatching or checking a mission previously
// required already knowing the exact /scoutmission or /scoutstatus slash
// syntax, with no button path and no formatting at all.
//
// Shows the in-flight mission status (if any) with no action buttons -
// nothing to tap mid-mission, matching HandleExplorePanel's "expedition
// en route" read-only state - or, if the outpost is free, an info panel
// plus one-tap quantity buttons that dispatch directly.
func (h *ScoutMissionsHandler) HandleScoutPanel(c telebot.Context) error {
	_ = c.Notify(telebot.FindingLocation)
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	var campID string
	var availableScouts int
	err := h.DB.QueryRowContext(ctx, `
		SELECT e.id, COALESCE(w.scouts, 0)
		FROM encampments e
		LEFT JOIN workshop_inventory w ON w.encampment_id = e.id
		WHERE e.user_id = $1`, sender.ID).Scan(&campID, &availableScouts)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.MainNavigation())
	}

	statusText, activeCount, err := h.renderScoutStatus(ctx, campID)
	if err != nil {
		return c.Send("⚠️ Error checking scout status.", keyboards.CombatNavigation())
	}
	if activeCount >= maxConcurrentScoutMissions {
		return sendPanelWithNavHTML(c, navCaptionCombat, keyboards.CombatNavigation(), statusText, nil)
	}

	panelText := statusText + "\n\n" +
		htmlItalic("Commit Scout Walkers to search the entire wasteland - no fixed destination, no fixed ETA. They search until they find another outpost, then report back and come home.") + "\n\n" +
		fmt.Sprintf("🪖 Available Scout Walkers: %s\n", htmlCode(fmt.Sprintf("%d", availableScouts))) +
		divider

	selector := &telebot.ReplyMarkup{}
	var row []telebot.Btn
	for _, n := range scoutQuickDispatchCounts {
		if n > availableScouts {
			continue
		}
		row = append(row, selector.Data(fmt.Sprintf("🔭 %d", n), "scout_dispatch", fmt.Sprintf("%d", n)))
	}
	var rows []telebot.Row
	if len(row) > 0 {
		rows = append(rows, selector.Row(row...))
	}
	if availableScouts > 0 {
		rows = append(rows, selector.Row(selector.Data("🔭 All Available", "scout_dispatch", fmt.Sprintf("%d", availableScouts))))
	}
	if len(rows) == 0 {
		panelText += "\n\n" + htmlItalic("No Scout Walkers available - recruit some at the Heavy Workshop first.")
		return sendPanelWithNavHTML(c, navCaptionCombat, keyboards.CombatNavigation(), panelText, nil)
	}
	selector.Inline(rows...)
	return sendPanelWithNavHTML(c, navCaptionCombat, keyboards.CombatNavigation(), panelText, selector)
}

// renderScoutStatus builds the beautified status block shared by
// HandleScoutStatus and HandleScoutPanel, listing every one of this
// encampment's currently active missions (up to maxConcurrentScoutMissions)
// rather than just the most recent one, and reports how many are active so
// callers can decide whether there's still a free dispatch slot.
func (h *ScoutMissionsHandler) renderScoutStatus(ctx context.Context, campID string) (string, int, error) {
	rows, err := h.DB.QueryContext(ctx, `
		SELECT sm.phase, sm.scouts_committed, te.name, sm.return_eta
		FROM scout_missions sm
		LEFT JOIN encampments te ON te.id = sm.found_target_encampment_id
		WHERE sm.encampment_id = $1 AND sm.phase IN ('searching', 'returning')
		ORDER BY sm.started_at ASC`, campID)
	if err != nil {
		return "", 0, err
	}
	defer rows.Close()

	type activeMission struct {
		phase           string
		scoutsCommitted int
		foundName       sql.NullString
		returnETA       sql.NullTime
	}
	var missions []activeMission
	for rows.Next() {
		var m activeMission
		if scanErr := rows.Scan(&m.phase, &m.scoutsCommitted, &m.foundName, &m.returnETA); scanErr == nil {
			missions = append(missions, m)
		}
	}
	if err := rows.Err(); err != nil {
		return "", 0, err
	}

	header := "🔭 " + htmlBold("LONG-RANGE SCOUTING") + "\n" + divider + "\n"
	if len(missions) == 0 {
		return header + htmlItalic("No scout party is currently out.") + "\n" + divider, 0, nil
	}

	var body string
	for i, m := range missions {
		label := htmlBold(fmt.Sprintf("Party %d/%d", i+1, maxConcurrentScoutMissions))
		switch m.phase {
		case "searching":
			body += fmt.Sprintf("%s: 🪖 %s Scout Walkers still searching the wasteland\n", label, htmlCode(fmt.Sprintf("%d", m.scoutsCommitted)))
		case "returning":
			body += fmt.Sprintf("%s: 🚶 %s Scout Walkers returning home", label, htmlCode(fmt.Sprintf("%d", m.scoutsCommitted)))
			if m.foundName.Valid {
				body += fmt.Sprintf(" after locating Outpost %s", htmlBold(htmlEscape(m.foundName.String)))
			}
			body += "\n"
			if m.returnETA.Valid {
				remaining := time.Until(m.returnETA.Time.UTC())
				body += fmt.Sprintf("   ⏱️ ETA: %s (%s remaining)\n", htmlCode(m.returnETA.Time.UTC().Format("15:04 MST")), htmlCode(formatDuration(remaining)))
			}
		}
	}
	if len(missions) < maxConcurrentScoutMissions {
		body += "\n" + htmlItalic(fmt.Sprintf("%d of %d scouting slots free - dispatch another below.", maxConcurrentScoutMissions-len(missions), maxConcurrentScoutMissions))
	}
	return header + body + divider, len(missions), nil
}

// HandleScoutStatus (/scoutstatus) is the slash-command equivalent of the
// panel's read-only status view, kept for players who prefer typing.
func (h *ScoutMissionsHandler) HandleScoutStatus(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	campID, err := h.myScoutCamp(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	statusText, _, err := h.renderScoutStatus(ctx, campID)
	if err != nil {
		return c.Send("⚠️ Error checking scout status.")
	}
	return c.Send(statusText, telebot.ModeHTML)
}
