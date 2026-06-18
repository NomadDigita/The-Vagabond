package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type ArenaHandler struct {
	DB *sql.DB
}

func NewArenaHandler(db *sql.DB) *ArenaHandler {
	return &ArenaHandler{DB: db}
}

// HandleArenaPanel renders the matchmaking queue status panel (No markup conflict)
func (h *ArenaHandler) HandleArenaPanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	ctx := context.Background()

	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	if err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.MainNavigation())
	}

	var q1, q2, q3 int
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM arena_queue WHERE bracket = '1v1'").Scan(&q1)
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM arena_queue WHERE bracket = '2v2'").Scan(&q2)
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM arena_queue WHERE bracket = '3v3'").Scan(&q3)

	var dollars float64
	_ = h.DB.QueryRowContext(ctx, "SELECT dollars FROM resources WHERE encampment_id = $1", campID).Scan(&dollars)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🏟️ COGNITIVE COMBAT MATCHMAKER\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Register in the automated queues to match against other active outposts.\n\n"+
			"SURVIVAL CAPITAL:\n"+
			"💵 Available Balance: $%.1f\n\n"+
			"QUEUE METRICS:\n"+
			"⚔️ [1v1 Duel] — Active: %d players | Cost: $50.0\n"+
			"👥 [2v2 Skirmish] — Active: %d players | Cost: $100.0\n"+
			"🤖 [3v3 Team Clash] — Active: %d players | Cost: $200.0\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		dollars, q1, q2, q3,
	)

	selector := &telebot.ReplyMarkup{}

	btnJoin1 := selector.Data("⚔️ Queue 1v1", "join_queue", "1v1")
	btnJoin2 := selector.Data("👥 Queue 2v2", "join_queue", "2v2")
	btnJoin3 := selector.Data("🤖 Queue 3v3", "join_queue", "3v3")

	selector.Inline(
		selector.Row(btnJoin1, btnJoin2),
		selector.Row(btnJoin3),
	)

	// Send without a trailing Reply Keyboard parameter so that inline buttons display successfully
	return c.Send(panelText, selector)
}

// HandleJoinQueueCallback handles inserting the player into the matchmaking table
func (h *ArenaHandler) HandleJoinQueueCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()

	bracket := c.Args()[0]

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Database error."})
	}
	defer tx.Rollback()

	var exists bool
	_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM arena_queue WHERE user_id = $1)", sender.ID).Scan(&exists)
	if exists {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Already queued: Wait for a match."})
	}

	var dollars float64
	_ = tx.QueryRowContext(ctx, "SELECT dollars FROM resources WHERE encampment_id = (SELECT id FROM encampments WHERE user_id = $1) FOR UPDATE", sender.ID).Scan(&dollars)

	var entryFee float64
	switch bracket {
	case "2v2":
		entryFee = 100.0
	case "3v3":
		entryFee = 200.0
	default:
		entryFee = 50.0
	}

	if dollars < entryFee {
		return c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("❌ Insufficient Funds: Need $%0.f.", entryFee)})
	}

	_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars - $1 WHERE encampment_id = (SELECT id FROM encampments WHERE user_id = $2)", entryFee, sender.ID)

	_, err = tx.ExecContext(ctx, "INSERT INTO arena_queue (user_id, bracket) VALUES ($1, $2)", sender.ID, bracket)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error writing queue record."})
	}

	_ = tx.Commit()
	_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("🏟️ Successfully joined %s queue!", bracket)})
	return h.HandleArenaPanel(c)
}