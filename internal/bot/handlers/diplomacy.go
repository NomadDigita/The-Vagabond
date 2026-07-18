package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type DiplomacyHandler struct {
	DB *sql.DB
}

func NewDiplomacyHandler(db *sql.DB) *DiplomacyHandler {
	return &DiplomacyHandler{DB: db}
}

// getMyClanForDiplomacy mirrors federation.go's private getMyClan helper
// (can't call that one directly - it's a method on *FederationHandler,
// a different receiver type).
func (h *DiplomacyHandler) getMyClanForDiplomacy(ctx context.Context, userID int64) (clanID string, clanName string, isKing bool, err error) {
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

func pactLabel(pactType string) string {
	if pactType == "alliance" {
		return "🤝 Alliance"
	}
	return "🕊️ Non-Aggression Pact"
}

// HandleDiplomacyPanel (/diplomacy) lists this Clan's active pacts and
// any pending proposals awaiting a response.
func (h *DiplomacyHandler) HandleDiplomacyPanel(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	clanID, _, isKing, err := h.getMyClanForDiplomacy(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ You must be in a Clan first. Use /clan to create or join one.")
	}

	panelText := "🕊️━━━━━━━━━━━━━━━━━━━━━━🕊️\n" +
		"🕊️ CLAN DIPLOMACY\n" +
		"🕊️━━━━━━━━━━━━━━━━━━━━━━🕊️\n"

	rows, err := h.DB.QueryContext(ctx, `
		SELECT d.id, d.pact_type, d.status, d.clan_a_id, d.clan_b_id,
		       CASE WHEN d.clan_a_id = $1 THEN cb.name ELSE ca.name END AS other_name,
		       d.proposed_by
		FROM clan_diplomacy d
		JOIN clans ca ON ca.id = d.clan_a_id
		JOIN clans cb ON cb.id = d.clan_b_id
		WHERE (d.clan_a_id = $1 OR d.clan_b_id = $1) AND d.status IN ('pending', 'active')
		ORDER BY d.status DESC, d.created_at DESC`, clanID)
	if err != nil {
		return c.Send("⚠️ Error querying diplomatic records.")
	}
	defer rows.Close()

	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row
	hasActive := false
	var activeText, pendingText string

	for rows.Next() {
		var id, pactType, status, clanAID, clanBID, otherName string
		var proposedBy int64
		if err := rows.Scan(&id, &pactType, &status, &clanAID, &clanBID, &otherName, &proposedBy); err != nil {
			continue
		}
		if status == "active" {
			hasActive = true
			activeText += fmt.Sprintf("%s with %s\n", pactLabel(pactType), otherName)
		} else {
			// Pending: only the RECEIVING Clan King (not the proposer) gets accept/reject buttons.
			isRecipient := (clanAID == clanID && proposedBy != sender.ID) || (clanBID == clanID && proposedBy != sender.ID)
			pendingText += fmt.Sprintf("%s proposed by %s\n", pactLabel(pactType), otherName)
			if isRecipient && isKing {
				btnAccept := selector.Data(fmt.Sprintf("✅ Accept %s (%s)", pactLabel(pactType), otherName), "diplo_respond", id, "accept")
				btnReject := selector.Data(fmt.Sprintf("❌ Reject %s (%s)", pactLabel(pactType), otherName), "diplo_respond", id, "reject")
				buttons = append(buttons, selector.Row(btnAccept), selector.Row(btnReject))
			}
		}
	}

	if hasActive {
		panelText += "\n✅ ACTIVE PACTS:\n" + activeText
	}
	if pendingText != "" {
		panelText += "\n⏳ PENDING PROPOSALS:\n" + pendingText
	}
	if !hasActive && pendingText == "" {
		panelText += "\nNo active pacts or pending proposals.\n"
	}

	panelText += "\n💡 A Clan King can propose an alliance with /ally [clan_name], or a\n" +
		"non-aggression pact with /nap [clan_name]. An active pact of either\n" +
		"kind blocks raids between the two Clans until broken with /break_pact [clan_name].\n" +
		"🕊️━━━━━━━━━━━━━━━━━━━━━━🕊️"

	selector.Inline(buttons...)
	return sendPanelWithNav(c, navCaptionEconomy, keyboards.EconomyNavigation(), panelText, selector)
}

// proposePact handles both /ally and /nap - they differ only in
// pactType, so the actual proposal logic is shared.
func (h *DiplomacyHandler) proposePact(c telebot.Context, pactType string) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	targetName := c.Message().Payload
	if targetName == "" {
		if pactType == "alliance" {
			return c.Send("⚠️ Usage: /ally [clan_name]")
		}
		return c.Send("⚠️ Usage: /nap [clan_name]")
	}

	myClanID, myClanName, isKing, err := h.getMyClanForDiplomacy(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ You must be in a Clan first. Use /clan to create or join one.")
	}
	if !isKing {
		return c.Send("❌ Only your Clan's King can propose diplomatic pacts.")
	}

	var targetClanID string
	err = h.DB.QueryRowContext(ctx, "SELECT id FROM clans WHERE LOWER(name) = LOWER($1)", targetName).Scan(&targetClanID)
	if err != nil {
		return c.Send("❌ No Clan found with that name.")
	}
	if targetClanID == myClanID {
		return c.Send("❌ You can't propose a pact with your own Clan.")
	}

	var existing sql.NullString
	_ = h.DB.QueryRowContext(ctx, `
		SELECT status FROM clan_diplomacy
		WHERE ((clan_a_id = $1 AND clan_b_id = $2) OR (clan_a_id = $2 AND clan_b_id = $1))
		AND status IN ('pending', 'active') LIMIT 1`, myClanID, targetClanID).Scan(&existing)
	if existing.Valid {
		return c.Send(fmt.Sprintf("❌ There's already a %s pact/proposal between your Clans.", existing.String))
	}

	_, err = h.DB.ExecContext(ctx, "INSERT INTO clan_diplomacy (clan_a_id, clan_b_id, pact_type, proposed_by) VALUES ($1, $2, $3, $4)",
		myClanID, targetClanID, pactType, sender.ID)
	if err != nil {
		return c.Send("⚠️ Error proposing pact.")
	}

	return c.Send(fmt.Sprintf("%s: %s proposed to %s! Their Clan King can accept via /diplomacy.", myClanName, pactLabel(pactType), targetName))
}

// HandleProposeAlliance (/ally [clan_name])
func (h *DiplomacyHandler) HandleProposeAlliance(c telebot.Context) error {
	return h.proposePact(c, "alliance")
}

// HandleProposeNAP (/nap [clan_name])
func (h *DiplomacyHandler) HandleProposeNAP(c telebot.Context) error {
	return h.proposePact(c, "nap")
}

// HandleDiplomacyRespondCallback fires when the receiving Clan King taps
// Accept/Reject on a pending proposal from /diplomacy.
func (h *DiplomacyHandler) HandleDiplomacyRespondCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	args := c.Args()
	pactID, decision := args[0], args[1]

	myClanID, _, isKing, err := h.getMyClanForDiplomacy(ctx, sender.ID)
	if err != nil || !isKing {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Only your Clan's King can respond to diplomatic proposals."})
	}

	var clanAID, clanBID, status string
	var proposedBy int64
	err = h.DB.QueryRowContext(ctx, "SELECT clan_a_id, clan_b_id, status, proposed_by FROM clan_diplomacy WHERE id = $1", pactID).Scan(&clanAID, &clanBID, &status, &proposedBy)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Proposal no longer exists."})
	}
	if status != "pending" {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ This proposal has already been resolved."})
	}
	if clanAID != myClanID && clanBID != myClanID {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ This proposal isn't addressed to your Clan."})
	}
	if proposedBy == sender.ID {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ You can't accept/reject your own Clan's outgoing proposal - wait for the other side to respond."})
	}

	newStatus := "rejected"
	if decision == "accept" {
		newStatus = "active"
	}

	_, err = h.DB.ExecContext(ctx, "UPDATE clan_diplomacy SET status = $1, responded_at = CURRENT_TIMESTAMP WHERE id = $2", newStatus, pactID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error responding to proposal."})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("Pact %s.", newStatus)})
	return h.HandleDiplomacyPanel(c)
}

// HandleBreakPact (/break_pact [clan_name]) lets either Clan King end an
// active pact unilaterally - diplomacy shouldn't be a permanent trap.
func (h *DiplomacyHandler) HandleBreakPact(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	targetName := c.Message().Payload
	if targetName == "" {
		return c.Send("⚠️ Usage: /break_pact [clan_name]")
	}

	myClanID, _, isKing, err := h.getMyClanForDiplomacy(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ You must be in a Clan first. Use /clan to create or join one.")
	}
	if !isKing {
		return c.Send("❌ Only your Clan's King can break a diplomatic pact.")
	}

	var targetClanID string
	err = h.DB.QueryRowContext(ctx, "SELECT id FROM clans WHERE LOWER(name) = LOWER($1)", targetName).Scan(&targetClanID)
	if err != nil {
		return c.Send("❌ No Clan found with that name.")
	}

	res, err := h.DB.ExecContext(ctx, `
		UPDATE clan_diplomacy SET status = 'broken', responded_at = CURRENT_TIMESTAMP
		WHERE ((clan_a_id = $1 AND clan_b_id = $2) OR (clan_a_id = $2 AND clan_b_id = $1)) AND status = 'active'`,
		myClanID, targetClanID)
	if err != nil {
		return c.Send("⚠️ Error breaking pact.")
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return c.Send("❌ No active pact found with that Clan.")
	}

	return c.Send(fmt.Sprintf("🕊️💔 Diplomatic pact with %s has been broken. Raids between your Clans are no longer blocked.", targetName))
}

// pactQueryer is satisfied by both *sql.DB and *sql.Tx, so combat.go's
// raid-launch check (already inside a transaction) and any future
// caller outside one can both use HasActivePact.
type pactQueryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// HasActivePact reports whether two Clans currently have an active
// alliance or non-aggression pact - used by combat.go's raid launch to
// block attacks between diplomatically protected Clans.
func HasActivePact(ctx context.Context, q pactQueryer, clanAID, clanBID string) bool {
	if clanAID == "" || clanBID == "" || clanAID == clanBID {
		return false
	}
	var exists bool
	_ = q.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM clan_diplomacy
			WHERE ((clan_a_id = $1 AND clan_b_id = $2) OR (clan_a_id = $2 AND clan_b_id = $1))
			AND status = 'active'
		)`, clanAID, clanBID).Scan(&exists)
	return exists
}
