package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/telebot.v3"
)

type ScoutMissionsHandler struct {
	DB *sql.DB
}

func NewScoutMissionsHandler(db *sql.DB) *ScoutMissionsHandler {
	return &ScoutMissionsHandler{DB: db}
}

// HandleDispatchScoutMission (/scout <count>) commits count scouts from
// workshop_inventory to a long-range scout mission - the human-facing
// entry point for AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section 3's
// scouting rework. One mission at a time per outpost, same one-dispatch
// convention /newjobexplore already uses. Resolution (the actual search,
// discovery, and return trip) happens entirely in the tick engine's
// scoutmissions.go - this handler only validates and creates the row.
func (h *ScoutMissionsHandler) HandleDispatchScoutMission(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	count, err := strconv.Atoi(strings.TrimSpace(c.Message().Payload))
	if err != nil || count <= 0 {
		return c.Send("🔭 Usage: /scout <number of scouts to commit>, e.g. /scout 5")
	}

	var campID string
	var availableScouts int
	err = h.DB.QueryRowContext(ctx, `
		SELECT e.id, COALESCE(w.scouts, 0)
		FROM encampments e
		LEFT JOIN workshop_inventory w ON w.encampment_id = e.id
		WHERE e.user_id = $1`, sender.ID).Scan(&campID, &availableScouts)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}
	if count > availableScouts {
		return c.Send(fmt.Sprintf("❌ You only have %d available scouts (some may already be committed elsewhere).", availableScouts))
	}

	var existingMission bool
	_ = h.DB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM scout_missions WHERE encampment_id = $1 AND phase IN ('searching', 'returning'))", campID).Scan(&existingMission)
	if existingMission {
		return c.Send("❌ You already have a scout party out. Check /scoutstatus and wait for them to return first.")
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ Error dispatching scout party.")
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, "UPDATE workshop_inventory SET scouts = scouts - $1 WHERE encampment_id = $2", count, campID); err != nil {
		return c.Send("⚠️ Error committing scouts.")
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO scout_missions (encampment_id, scouts_committed) VALUES ($1, $2)", campID, count); err != nil {
		return c.Send("⚠️ Error creating scout mission.")
	}
	if err := tx.Commit(); err != nil {
		return c.Send("⚠️ Error dispatching scout party.")
	}

	return c.Send(fmt.Sprintf(
		"🔭 SCOUT PARTY DISPATCHED: %d scouts have set out to search the wasteland. There's no fixed ETA - they'll search until they find something (or you'll hear from them periodically). Check /scoutstatus anytime.",
		count))
}

// HandleScoutStatus (/scoutstatus) reports the current mission's phase
// without waiting for the next periodic tick-engine ping.
func (h *ScoutMissionsHandler) HandleScoutStatus(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	var campID string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start")
	}

	var phase string
	var scoutsCommitted int
	var foundName sql.NullString
	var returnETA sql.NullTime
	err := h.DB.QueryRowContext(ctx, `
		SELECT sm.phase, sm.scouts_committed, te.name, sm.return_eta
		FROM scout_missions sm
		LEFT JOIN encampments te ON te.id = sm.found_target_encampment_id
		WHERE sm.encampment_id = $1 AND sm.phase IN ('searching', 'returning')
		ORDER BY sm.started_at DESC LIMIT 1`, campID).Scan(&phase, &scoutsCommitted, &foundName, &returnETA)
	if err == sql.ErrNoRows {
		return c.Send("🔭 No scout party is currently out. Use /scout <count> to dispatch one.")
	}
	if err != nil {
		return c.Send("⚠️ Error checking scout status.")
	}

	switch phase {
	case "searching":
		return c.Send(fmt.Sprintf("🔭 %d scouts are still searching the wasteland. No contact yet - no fixed ETA, but you'll be pinged periodically.", scoutsCommitted))
	case "returning":
		msg := fmt.Sprintf("🚶 %d scouts are returning home", scoutsCommitted)
		if foundName.Valid {
			msg += fmt.Sprintf(" after locating Outpost [%s]", foundName.String)
		}
		if returnETA.Valid {
			msg += fmt.Sprintf(". ETA: %s", returnETA.Time.UTC().Format("15:04 MST"))
		}
		return c.Send(msg + ".")
	default:
		return c.Send("🔭 No scout party is currently out. Use /scout <count> to dispatch one.")
	}
}
