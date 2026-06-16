package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"gopkg.in/telebot.v3"
)

type EconomyHandler struct {
	DB *sql.DB
}

func NewEconomyHandler(db *sql.DB) *EconomyHandler {
	return &EconomyHandler{DB: db}
}

// HandleEconPanel renders the banking and trade hub
func (h *EconomyHandler) HandleEconPanel(c telebot.Context) error {
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

	// Fetch or Initialize Bank Account
	var bankBalance float64
	var loanAmount float64
	queryBank := `SELECT balance, loan_amount FROM bank_accounts WHERE encampment_id = $1`
	err = h.DB.QueryRowContext(ctx, queryBank, campID).Scan(&bankBalance, &loanAmount)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO bank_accounts (encampment_id, balance, loan_amount) VALUES ($1, 0.00, 0.00)", campID)
		bankBalance = 0.0
		loanAmount = 0.0
	}

	var scrap float64
	_ = h.DB.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1", campID).Scan(&scrap)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🏦 IRONCLAD VAULT & MARKETPLACE\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Manage your savings, borrow survival credit, or trade for tactical assets.\n\n"+
			"LEDGER BALANCES:\n"+
			"⚙️ Available Scrap: %.1f\n"+
			"🏦 Vault Savings: %.1f Scrap\n"+
			"💳 Credit Debt: %.1f Scrap\n\n"+
			"MARKET CONTRACTS AVAILABILITY:\n"+
			"📦 [Rations Supply Case] — Cost: 50 Scrap (+100 Rations)\n"+
			"👥 [Elite Enforcer Contract] — Cost: 200 Scrap (+5 Enforcers)\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		scrap, bankBalance, loanAmount,
	)

	selector := &telebot.ReplyMarkup{}

	btnDeposit := selector.Data("🏦 Deposit 100 Scrap", "bank_action", "deposit", campID)
	btnBorrow := selector.Data("💳 Borrow 100 Scrap", "bank_action", "borrow", campID)
	btnBuyRations := selector.Data("📦 Buy Rations Case", "market_buy", "rations", campID)
	btnBuyTroops := selector.Data("👥 Hire Enforcers", "market_buy", "enforcers", campID)

	selector.Inline(
		selector.Row(btnDeposit, btnBorrow),
		selector.Row(btnBuyRations),
		selector.Row(btnBuyTroops),
	)

	return c.Send(panelText, selector, keyboards.CombatNavigation())
}

// HandleBankCallback processes transactional actions in the Bank
func (h *EconomyHandler) HandleBankCallback(c telebot.Context) error {
	ctx := context.Background()

	action := c.Args()[0]
	campID := c.Args()[1]

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transaction failed."})
	}
	defer tx.Rollback()

	var scrap float64
	_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap)

	var balance float64
	var loan float64
	_ = tx.QueryRowContext(ctx, "SELECT balance, loan_amount FROM bank_accounts WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&balance, &loan)

	switch action {
	case "deposit":
		if scrap < 100.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Scrap to deposit 100."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - 100.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE bank_accounts SET balance = balance + 100.0 WHERE encampment_id = $1", campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "🏦 Deposited 100 Scrap into savings."})

	case "borrow":
		if loan >= 300.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Credit Limit Reached: Repay existing debt first."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap + 100.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE bank_accounts SET loan_amount = loan_amount + 100.0 WHERE encampment_id = $1", campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "💳 Borrowed 100 Scrap. Debt accumulated."})
	}

	_ = tx.Commit()
	return h.HandleEconPanel(c)
}

// HandleMarketCallback processes item acquisitions
func (h *EconomyHandler) HandleMarketCallback(c telebot.Context) error {
	ctx := context.Background()

	item := c.Args()[0]
	campID := c.Args()[1]

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transaction failed."})
	}
	defer tx.Rollback()

	var scrap float64
	_ = tx.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap)

	switch item {
	case "rations":
		if scrap < 50.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Scrap. Cost is 50."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - 50.0, rations = rations + 100.0 WHERE encampment_id = $1", campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "📦 Rations supply acquired! +100 food."})

	case "enforcers":
		if scrap < 200.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Scrap. Cost is 200."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - 200.0 WHERE encampment_id = $1", campID)

		// Spawn or Increment Enforcers
		var exists bool
		_ = tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM units WHERE encampment_id = $1 AND type = 'enforcer')", campID).Scan(&exists)

		if exists {
			_, _ = tx.ExecContext(ctx, "UPDATE units SET quantity = quantity + 5 WHERE encampment_id = $1 AND type = 'enforcer'", campID)
		} else {
			_, _ = tx.ExecContext(ctx, "INSERT INTO units (encampment_id, type, quantity) VALUES ($1, 'enforcer', 5)", campID)
		}
		_ = c.Respond(&telebot.CallbackResponse{Text: "👥 Hired 5 Enforcers! Barracks updated."})
	}

	_ = tx.Commit()
	return h.HandleEconPanel(c)
}
