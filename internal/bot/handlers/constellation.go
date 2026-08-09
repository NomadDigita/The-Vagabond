package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
	"gopkg.in/telebot.v3"
)

// RARE_WORLD_FEATURES_PLAN.md Phase 5: Constellation Alliances.
//
// A rare, temporary, N-ary cross-clan structure sitting above the
// existing pairwise clan_diplomacy pacts (diplomacy.go). See the plan
// doc's section 5.1 for why this needed its own header+membership
// table pair (constellations/constellation_members) rather than
// reusing clan_diplomacy's pairwise shape. The tick.Engine side
// (dividend payout, expiry) lives in internal/engine/tick/constellations.go.

// constellationDurationDays is how long an activated Constellation
// lasts before automatically dissolving - long enough to feel like a
// real commitment, short enough to stay a special, opt-in moment
// rather than a silent permanent power bloc (see the plan doc's design
// principle 2 - "rare means rare").
const constellationDurationDays = 7

// constellationMaxMembers is the largest a Constellation can be
// (founder + up to 2 invited clans) - kept small so "which clans are
// in my Constellation" stays an easy, memorable fact rather than
// approaching a second clan-membership system.
const constellationMaxMembers = 3

type ConstellationHandler struct {
	DB *sql.DB
}

func NewConstellationHandler(db *sql.DB) *ConstellationHandler {
	return &ConstellationHandler{DB: db}
}

// getMyClanForConstellation mirrors diplomacy.go's
// getMyClanForDiplomacy exactly (can't call that one directly - it's a
// method on a different receiver type).
func (h *ConstellationHandler) getMyClanForConstellation(ctx context.Context, userID int64) (clanID string, clanName string, isLeader bool, err error) {
	var leaderID int64
	err = h.DB.QueryRowContext(ctx, `
		SELECT c.id, c.name, c.leader_id
		FROM clans c
		JOIN user_clans uc ON uc.clan_id = c.id
		WHERE uc.user_id = $1`, userID).Scan(&clanID, &clanName, &leaderID)
	if err != nil {
		return "", "", false, err
	}
	return clanID, clanName, leaderID == userID, nil
}

// HandleConstellationForm (/constellation_form Name, ClanA, ClanB)
// lets a Clan Leader propose forming a Constellation with 1-2 other
// named Clans (comma-separated so both the Constellation's own name
// and Clan names can contain spaces - see plan doc 5.3). Creates the
// constellations row (status 'forming'), an 'accepted' member row for
// the founder's own Clan, and one 'pending' row per invited Clan.
func (h *ConstellationHandler) HandleConstellationForm(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	parts := splitConstellationArgs(c.Message().Payload)
	if len(parts) < 2 || len(parts) > constellationMaxMembers {
		return c.Send(fmt.Sprintf("⚠️ Usage: %s\nName your Constellation and 1-2 other Clans to invite, comma-separated.",
			htmlCode("/constellation_form Name, ClanA, ClanB")), telebot.ModeHTML)
	}
	constellationName := parts[0]
	inviteeNames := parts[1:]

	myClanID, myClanName, isLeader, err := h.getMyClanForConstellation(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ You must be in a Clan first. Use /clan to create or join one.")
	}
	if !isLeader {
		return c.Send("❌ Only your Clan's Leader can propose forming a Constellation.")
	}

	var existingActive bool
	_ = h.DB.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM constellation_members cm
			JOIN constellations cst ON cst.id = cm.constellation_id
			WHERE cm.clan_id = $1 AND cst.status IN ('forming', 'active')
		)`, myClanID).Scan(&existingActive)
	if existingActive {
		return c.Send("❌ Your Clan is already forming or part of an active Constellation. Wait for it to resolve or expire first.")
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Send("⚠️ Error forming Constellation.")
	}
	defer tx.Rollback()

	inviteeClanIDs := make(map[string]string, len(inviteeNames)) // clanID -> display name
	for _, name := range inviteeNames {
		var clanID, clanName string
		if err := tx.QueryRowContext(ctx, "SELECT id, name FROM clans WHERE LOWER(name) = LOWER($1)", name).Scan(&clanID, &clanName); err != nil {
			return c.Send(fmt.Sprintf("❌ No Clan found named %s.", htmlEscape(name)), telebot.ModeHTML)
		}
		if clanID == myClanID {
			return c.Send("❌ You can't invite your own Clan to your own Constellation.")
		}
		inviteeClanIDs[clanID] = clanName
	}

	var constellationID string
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO constellations (name, founder_clan_id, status)
		VALUES ($1, $2, 'forming') RETURNING id`, constellationName, myClanID).Scan(&constellationID); err != nil {
		return c.Send("⚠️ Error forming Constellation.")
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO constellation_members (constellation_id, clan_id, invited_by, status, responded_at)
		VALUES ($1, $2, $3, 'accepted', CURRENT_TIMESTAMP)`, constellationID, myClanID, sender.ID); err != nil {
		return c.Send("⚠️ Error forming Constellation.")
	}
	var inviteeDisplay []string
	for clanID, clanName := range inviteeClanIDs {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO constellation_members (constellation_id, clan_id, invited_by, status)
			VALUES ($1, $2, $3, 'pending')`, constellationID, clanID, sender.ID); err != nil {
			return c.Send("⚠️ Error forming Constellation.")
		}
		inviteeDisplay = append(inviteeDisplay, clanName)
	}

	if err := tx.Commit(); err != nil {
		return c.Send("⚠️ Error forming Constellation.")
	}

	return c.Send(fmt.Sprintf("✨ %s: %s has proposed forming the %s Constellation with %s! Each invited Clan Leader can accept via %s. All invited Clans must accept before it activates.",
		htmlEscape(myClanName), htmlEscape(myClanName), htmlBold(htmlEscape(constellationName)), htmlEscape(strings.Join(inviteeDisplay, ", ")), htmlCode("/constellation")),
		telebot.ModeHTML)
}

// splitConstellationArgs splits a comma-separated payload into trimmed,
// non-empty fields.
func splitConstellationArgs(payload string) []string {
	raw := strings.Split(payload, ",")
	var out []string
	for _, p := range raw {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// HandleConstellationPanel (/constellation) shows this Clan's current
// Constellation (any status - forming/active/dissolved most recent
// first), member Clans, banner, expiry countdown, and Accept/Reject
// buttons for any pending invitation addressed to this Clan.
func (h *ConstellationHandler) HandleConstellationPanel(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	myClanID, _, isLeader, err := h.getMyClanForConstellation(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ You must be in a Clan first. Use /clan to create or join one.")
	}

	panelText := "✨ " + htmlBold("CONSTELLATION ALLIANCES") + "\n" + divider + "\n\n"
	selector := &telebot.ReplyMarkup{}

	var constellationID, name, bannerEmoji, status, founderClanID string
	var expiresAt sql.NullTime
	err = h.DB.QueryRowContext(ctx, `
		SELECT cst.id, cst.name, cst.banner_emoji, cst.status, cst.founder_clan_id, cst.expires_at
		FROM constellations cst
		JOIN constellation_members cm ON cm.constellation_id = cst.id
		WHERE cm.clan_id = $1
		ORDER BY cst.created_at DESC LIMIT 1`, myClanID).Scan(&constellationID, &name, &bannerEmoji, &status, &founderClanID, &expiresAt)

	var pendingAccept, pendingReject keyboards.StyledBtn
	hasPendingPrompt := false

	if err != nil {
		panelText += htmlItalic("Your Clan isn't part of a Constellation right now.") + "\n\n" +
			"💡 A Clan Leader can propose one with " + htmlCode("/constellation_form Name, ClanA, ClanB") + " - " +
			"2-3 Clans total, unanimous accept required, active for " + fmt.Sprintf("%d days", constellationDurationDays) + " once formed.\n"
	} else {
		memberRows, memErr := h.DB.QueryContext(ctx, `
			SELECT c.name, cm.status
			FROM constellation_members cm
			JOIN clans c ON c.id = cm.clan_id
			WHERE cm.constellation_id = $1
			ORDER BY cm.status DESC, c.name ASC`, constellationID)

		panelText += fmt.Sprintf("%s %s\n%s\n\n", bannerEmoji, htmlBold(htmlEscape(name)), statusLabel(status))
		if memErr == nil {
			panelText += htmlBold("Members:") + "\n"
			for memberRows.Next() {
				var mName, mStatus string
				if scanErr := memberRows.Scan(&mName, &mStatus); scanErr == nil {
					panelText += fmt.Sprintf("  %s %s (%s)\n", bannerEmoji, htmlEscape(mName), mStatus)
				}
			}
			memberRows.Close()
			panelText += "\n"
		}

		if status == "active" && expiresAt.Valid {
			panelText += fmt.Sprintf("⏳ Active until %s\n\n", expiresAt.Time.Format("Jan 2, 2006"))
		}

		if status == "forming" {
			var myStatus string
			var invitedBy int64
			_ = h.DB.QueryRowContext(ctx, "SELECT status, invited_by FROM constellation_members WHERE constellation_id = $1 AND clan_id = $2", constellationID, myClanID).Scan(&myStatus, &invitedBy)
			if myStatus == "pending" && isLeader {
				panelText += htmlItalic("Your Clan has been invited to join this Constellation - see the prompt below.") + "\n\n"
				pendingAccept = keyboards.Styled(selector.Data("✅ Accept", "constellation_r", constellationID, "accept"), keyboards.StyleSuccess)
				pendingReject = keyboards.Styled(selector.Data("❌ Reject", "constellation_r", constellationID, "reject"), keyboards.StyleDanger)
				hasPendingPrompt = true
			} else if myStatus == "pending" {
				panelText += htmlItalic("Your Clan has been invited, but only your Clan Leader can accept or reject.") + "\n\n"
			} else {
				panelText += htmlItalic("Waiting for every invited Clan to accept before this Constellation activates.") + "\n\n"
			}
		}
	}

	panelText += divider
	selector.Inline()
	if sendErr := sendPanelWithNavHTML(c, navCaptionEconomy, keyboards.EconomyNavigation(), panelText, selector); sendErr != nil {
		return sendErr
	}

	if hasPendingPrompt {
		promptText := "✨ " + htmlBold("CONSTELLATION INVITATION") + "\n" + divider + "\n" +
			fmt.Sprintf("%s Join the %s Constellation?", bannerEmoji, htmlBold(htmlEscape(name))) + "\n" + divider
		return keyboards.SendStyled(c, promptText, [][]keyboards.StyledBtn{{pendingAccept, pendingReject}})
	}
	return nil
}

func statusLabel(status string) string {
	switch status {
	case "forming":
		return "🔶 Forming - awaiting unanimous accept"
	case "active":
		return "🟢 Active"
	case "dissolved":
		return "⚪ Dissolved"
	default:
		return status
	}
}

// HandleConstellationRespondCallback fires when an invited Clan
// Leader taps Accept/Reject on HandleConstellationPanel. A single
// reject dissolves the whole forming Constellation (see plan doc 5.3
// point 4) rather than leaving it stuck at partial acceptance forever.
// Once every invited member has accepted, the Constellation activates:
// status -> 'active', activated_at/expires_at set, and a broadcast
// announces it to every member Clan's players.
func (h *ConstellationHandler) HandleConstellationRespondCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 2 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid response."})
	}
	constellationID, decision := c.Args()[0], c.Args()[1]

	myClanID, _, isLeader, err := h.getMyClanForConstellation(ctx, sender.ID)
	if err != nil || !isLeader {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Only your Clan's Leader can respond to a Constellation invitation."})
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error responding."})
	}
	defer tx.Rollback()

	var myStatus, constellationStatus string
	if err := tx.QueryRowContext(ctx, `
		SELECT cm.status, cst.status FROM constellation_members cm
		JOIN constellations cst ON cst.id = cm.constellation_id
		WHERE cm.constellation_id = $1 AND cm.clan_id = $2 FOR UPDATE`, constellationID, myClanID).Scan(&myStatus, &constellationStatus); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invitation no longer exists."})
	}
	if constellationStatus != "forming" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ This Constellation has already been resolved."})
	}
	if myStatus != "pending" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Your Clan has already responded to this invitation."})
	}

	if decision == "reject" {
		if _, err := tx.ExecContext(ctx, "UPDATE constellations SET status = 'dissolved' WHERE id = $1", constellationID); err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error responding."})
		}
		if _, err := tx.ExecContext(ctx, "UPDATE constellation_members SET status = 'rejected', responded_at = CURRENT_TIMESTAMP WHERE constellation_id = $1 AND clan_id = $2", constellationID, myClanID); err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error responding."})
		}
		if err := tx.Commit(); err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error responding."})
		}
		_ = c.Respond(&telebot.CallbackResponse{Text: "❌ Constellation invitation rejected - the whole proposal has been dissolved."})
		return h.HandleConstellationPanel(c)
	}

	if _, err := tx.ExecContext(ctx, "UPDATE constellation_members SET status = 'accepted', responded_at = CURRENT_TIMESTAMP WHERE constellation_id = $1 AND clan_id = $2", constellationID, myClanID); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error responding."})
	}

	var pendingCount int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM constellation_members WHERE constellation_id = $1 AND status = 'pending'", constellationID).Scan(&pendingCount); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error responding."})
	}

	if pendingCount == 0 {
		var name, bannerEmoji string
		_ = tx.QueryRowContext(ctx, "SELECT name, banner_emoji FROM constellations WHERE id = $1", constellationID).Scan(&name, &bannerEmoji)
		if _, err := tx.ExecContext(ctx, `
			UPDATE constellations SET status = 'active', activated_at = CURRENT_TIMESTAMP,
				expires_at = CURRENT_TIMESTAMP + ($2 * INTERVAL '1 day')
			WHERE id = $1`, constellationID, constellationDurationDays); err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error responding."})
		}
		if err := broadcastConstellationActivated(ctx, tx, constellationID, name, bannerEmoji); err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error responding."})
		}
	}

	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error responding."})
	}

	if pendingCount == 0 {
		_ = c.Respond(&telebot.CallbackResponse{Text: "✨ All Clans have accepted - the Constellation is now active!"})
	} else {
		_ = c.Respond(&telebot.CallbackResponse{Text: "✅ Accepted! Waiting on the remaining invited Clan(s)."})
	}
	return h.HandleConstellationPanel(c)
}

// broadcastConstellationActivated notifies every player in every member
// Clan directly (via notifications.Queue, not a region-scoped
// broadcast - member Clans' players may be spread across regions) that
// their Constellation has activated.
func broadcastConstellationActivated(ctx context.Context, tx *sql.Tx, constellationID, name, bannerEmoji string) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT uc.user_id
		FROM constellation_members cm
		JOIN user_clans uc ON uc.clan_id = cm.clan_id
		WHERE cm.constellation_id = $1`, constellationID)
	if err != nil {
		return fmt.Errorf("finding constellation member players: %w", err)
	}
	var userIDs []int64
	for rows.Next() {
		var uid int64
		if scanErr := rows.Scan(&uid); scanErr == nil {
			userIDs = append(userIDs, uid)
		}
	}
	rows.Close()

	msg := fmt.Sprintf("%s CONSTELLATION FORMED: The %s Constellation is now active for your Clan! Every member Clan's outposts will receive a joint Alignment Dividend for as long as it lasts.", bannerEmoji, name)
	for _, uid := range userIDs {
		if err := notifications.Queue(ctx, tx, uid, msg, "general"); err != nil {
			return fmt.Errorf("notifying constellation member %d: %w", uid, err)
		}
	}
	return nil
}
