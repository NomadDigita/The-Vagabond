package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
	"gopkg.in/telebot.v3"
)

// RelicConvoyHandler is the panel/claim side of RARE_WORLD_FEATURES_PLAN.md
// Phase 1 - the tick.Engine side (spawn/expire) lives in
// internal/engine/tick/relicconvoys.go.
type RelicConvoyHandler struct {
	DB *sql.DB
}

func NewRelicConvoyHandler(db *sql.DB) *RelicConvoyHandler {
	return &RelicConvoyHandler{DB: db}
}

// HandleRelicPanel (/relic) shows the current live relic convoy, if
// any, with a Claim button, plus a Hall of Relics - the last 10 claimed
// relics server-wide, rendered as a Rich Message table (same pattern
// exchange.go's listings board and ranking.go's leaderboard use) so a
// claimed relic has permanent, visible legacy.
func (h *RelicConvoyHandler) HandleRelicPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	ctx := context.Background()

	panelText := "🏺 " + htmlBold("RELIC CONVOYS") + "\n" + divider + "\n\n"
	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	var convoyID, relicName string
	var reward float64
	var region string
	err := h.DB.QueryRowContext(ctx, `
		SELECT rc.id, rc.relic_name, rc.reward_dollars, c.region
		FROM relic_convoys rc
		JOIN coordinates c ON c.id = rc.coordinate_id
		WHERE rc.claimed_by IS NULL AND rc.expires_at > CURRENT_TIMESTAMP
		ORDER BY rc.spawned_at DESC LIMIT 1`).Scan(&convoyID, &relicName, &reward, &region)

	if err == nil {
		panelText += fmt.Sprintf(
			"%s\n%s Last seen passing through the %s region, carrying %s in pre-war salvage.\n\n"+
				"⚡ First outpost to tap Claim keeps it all - no travel, no risk, just speed.\n\n",
			htmlBold("🟢 "+relicName), "📍", htmlEscape(region), htmlCode(fmt.Sprintf("$%.0f", reward)))
		btnClaim := selector.Data(fmt.Sprintf("🏺 Claim %s", relicName), "claim_relic", convoyID)
		buttons = append(buttons, selector.Row(btnClaim))
	} else {
		panelText += "📡 " + htmlItalic("No relic convoy currently detected. Keep an eye on server announcements - convoys are exceptionally rare and vanish fast once claimed.") + "\n\n"
	}

	panelText += htmlBold("📜 HALL OF RELICS") + "\n"
	rows, hallErr := h.DB.QueryContext(ctx, `
		SELECT rc.relic_name, e.name, rc.reward_dollars, rc.claimed_at
		FROM relic_convoys rc
		JOIN encampments e ON e.id = rc.claimed_by
		WHERE rc.claimed_by IS NOT NULL
		ORDER BY rc.claimed_at DESC LIMIT 10`)
	if hallErr == nil {
		var hallRows [][3]string
		for rows.Next() {
			var name, winner string
			var rewardAmt float64
			var claimedAt sql.NullTime
			if scanErr := rows.Scan(&name, &winner, &rewardAmt, &claimedAt); scanErr == nil {
				dateStr := "—"
				if claimedAt.Valid {
					dateStr = claimedAt.Time.Format("Jan 2")
				}
				hallRows = append(hallRows, [3]string{htmlEscape(name), fmt.Sprintf("🏺 %s (%s)", htmlEscape(winner), dateStr), htmlCode(fmt.Sprintf("$%.0f", rewardAmt))})
			}
		}
		rows.Close()
		panelText += rankTable("Reward", hallRows)
	} else {
		panelText += htmlItalic("No relics have been claimed yet - be the first.") + "\n"
	}

	panelText += "\n" + divider
	selector.Inline(buttons...)

	return sendPanelWithNavHTML(c, navCaptionMain, keyboards.MainNavigation(), panelText, selector)
}

// doClaimRelicConvoy is the testable core behind HandleClaimRelicCallback -
// race-safe via SELECT ... FOR UPDATE, re-checking claimed_by IS NULL
// AND not expired inside the same transaction as the claim, exactly
// like doBuyListingQty's listing lock or HandleAttackBossCallback's
// boss lock elsewhere in this codebase. On success, credits the
// reward (storage-cap clamped) and - only if this is the encampment's
// FIRST relic ever - sets a permanent relic_title that is never
// overwritten by a later claim.
func (h *RelicConvoyHandler) doClaimRelicConvoy(ctx context.Context, myCampID, convoyID string) (string, error) {
	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return "⚠️ Transaction failed.", err
	}
	defer tx.Rollback()

	var relicName string
	var reward float64
	var claimedBy sql.NullString
	var expired bool
	if err := tx.QueryRowContext(ctx, `
		SELECT relic_name, reward_dollars, claimed_by, expires_at <= CURRENT_TIMESTAMP
		FROM relic_convoys WHERE id = $1 FOR UPDATE`, convoyID).Scan(&relicName, &reward, &claimedBy, &expired); err != nil {
		return "❌ That relic convoy no longer exists.", err
	}
	if claimedBy.Valid {
		return "❌ Too slow! Another outpost already claimed this relic convoy.", errors.New("relic already claimed")
	}
	if expired {
		return "❌ This relic convoy has already moved on.", errors.New("relic convoy expired")
	}

	if _, err := tx.ExecContext(ctx, "UPDATE relic_convoys SET claimed_by = $1, claimed_at = CURRENT_TIMESTAMP WHERE id = $2", myCampID, convoyID); err != nil {
		return "⚠️ Claim failed.", err
	}

	var currentDollars float64
	_ = tx.QueryRowContext(ctx, "SELECT dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", myCampID).Scan(&currentDollars)
	storageCap := storagecap.CapFor(ctx, tx, myCampID)
	newDollars, _ := storagecap.Clamp(currentDollars, reward, storageCap)
	if _, err := tx.ExecContext(ctx, "UPDATE resources SET dollars = $1 WHERE encampment_id = $2", newDollars, myCampID); err != nil {
		return "⚠️ Claim failed.", err
	}

	// The relic title is permanent and never overwritten - only the
	// FIRST relic an encampment ever wins sets it (see plan doc 1.3).
	var existingTitle sql.NullString
	_ = tx.QueryRowContext(ctx, "SELECT relic_title FROM encampments WHERE id = $1", myCampID).Scan(&existingTitle)
	if !existingTitle.Valid || existingTitle.String == "" {
		title := fmt.Sprintf("🏺 Keeper of %s", strings.TrimPrefix(relicName, "The "))
		_, _ = tx.ExecContext(ctx, "UPDATE encampments SET relic_title = $1 WHERE id = $2", title, myCampID)
	}

	var winnerName string
	_ = tx.QueryRowContext(ctx, "SELECT name FROM encampments WHERE id = $1", myCampID).Scan(&winnerName)

	broadcast := fmt.Sprintf("🏺 RELIC CLAIMED: %s has claimed %s, walking away with %s in salvage!",
		winnerName, relicName, fmt.Sprintf("$%.0f", reward))
	if err := notifications.QueueToAllPlayers(ctx, tx, broadcast, "general"); err != nil {
		return "⚠️ Claim failed.", err
	}

	if err := tx.Commit(); err != nil {
		return "⚠️ Claim failed to commit.", err
	}

	return fmt.Sprintf("🏺✅ %s You claimed %s and %s in salvage!", htmlBold("RELIC SECURED!"), htmlEscape(relicName), htmlCode(fmt.Sprintf("$%.0f", reward))), nil
}

// HandleClaimRelicCallback fires when a player taps the Claim button on
// HandleRelicPanel.
func (h *RelicConvoyHandler) HandleClaimRelicCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 1 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid claim request."})
	}
	convoyID := c.Args()[0]

	var campID string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	msg, err := h.doClaimRelicConvoy(ctx, campID, convoyID)
	if err != nil {
		_ = c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: msg})
		return h.HandleRelicPanel(c)
	}

	_ = c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: "🏺 Relic secured!"})
	if sendErr := c.Send(msg, telebot.ModeHTML, keyboards.MainNavigation()); sendErr != nil {
		return sendErr
	}
	return h.HandleRelicPanel(c)
}
