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

	var bankBalance float64
	var loanAmount float64
	queryBank := `SELECT balance, loan_amount FROM bank_accounts WHERE encampment_id = $1`
	err = h.DB.QueryRowContext(ctx, queryBank, campID).Scan(&bankBalance, &loanAmount)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO bank_accounts (encampment_id, balance, loan_amount) VALUES ($1, 0.00, 0.00)", campID)
		bankBalance = 0.0
		loanAmount = 0.0
	}

	var scrap, energy, steel, uranium, hydrogen, dollars float64
	queryRes := `SELECT scrap, energy, steel, uranium, hydrogen, dollars FROM resources WHERE encampment_id = $1`
	_ = h.DB.QueryRowContext(ctx, queryRes, campID).Scan(&scrap, &energy, &steel, &uranium, &hydrogen, &dollars)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🏦 SYSTEM ECONOMY & FINANCIAL CENTER\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Manage vault reserves or buy heavy war components.\n\n"+
			"FINANCIAL LEDGERS:\n"+
			"💵 Available Funds: $%.1f\n"+
			"⚙️ Scrap Reserves: %.1f\n"+
			"🏦 Vault Savings: %.1f Scrap\n"+
			"💳 Credit Debt: %.1f Scrap\n\n"+
			"HEAVY WAR RESOURCES STOCK:\n"+
			"🧱 Steel Stock: %.1f tons\n"+
			"☢️ Uranium Stock: %.1f kg\n"+
			"🎈 Hydrogen Stock: %.1f L\n\n"+
			"MARKET VALUES:\n"+
			"💵 Convert: Sell 100 Scrap -> Get $50.0\n"+
			"🧱 Buy Steel Case: Cost $100 -> Get +50 Steel\n"+
			"☢️ Buy Uranium Core: Cost $200 -> Get +20 Uranium\n"+
			"🎈 Buy Hydrogen Fuel: Cost $150 -> Get +40 Hydrogen\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		dollars, scrap, bankBalance, loanAmount, steel, uranium, hydrogen,
	)

	selector := &telebot.ReplyMarkup{}

	// Removed campID parameter completely to remain 64-byte safe (dynamic lookup used in callbacks)
	btnDeposit := selector.Data("🏦 Deposit 100", "bank_action", "deposit")
	btnBorrow := selector.Data("💳 Borrow 100 (Loan)", "bank_action", "borrow")
	btnSellScrap := selector.Data("💵 Sell 100 Scrap", "market_buy", "sell_scrap")
	btnBuySteel := selector.Data("🧱 Buy Steel", "market_buy", "buy_steel")
	btnBuyUranium := selector.Data("☢️ Buy Uranium", "market_buy", "buy_uranium")
	btnBuyHydrogen := selector.Data("🎈 Buy Hydrogen", "market_buy", "buy_hydrogen")

	selector.Inline(
		selector.Row(btnDeposit, btnBorrow),
		selector.Row(btnSellScrap),
		selector.Row(btnBuySteel, btnBuyUranium, btnBuyHydrogen),
	)

	return c.Send(panelText, selector)
}

// HandleBankCallback processes transactional actions in the Bank (Dynamic campID lookup)
func (h *EconomyHandler) HandleBankCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()

	action := c.Args()[0]

	// Resolve campID dynamically from Sender ID
	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error resolving Outpost."})
	}

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
		if loan >= 500.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Credit Limit Reached: Repay existing debt first."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap + 100.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE bank_accounts SET loan_amount = loan_amount + 100.0 WHERE encampment_id = $1", campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "💳 Borrowed 100 Scrap. 15% automatic tax applied to ticks."})
	}

	_ = tx.Commit()
	return h.HandleEconPanel(c)
}

// HandleMarketCallback processes item acquisitions (Dynamic campID lookup)
func (h *EconomyHandler) HandleMarketCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()

	item := c.Args()[0]

	// Resolve campID dynamically from Sender ID
	var campID string
	err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error resolving Outpost."})
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Transaction failed."})
	}
	defer tx.Rollback()

	var scrap, dollars float64
	_ = tx.QueryRowContext(ctx, "SELECT scrap, dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap, &dollars)

	switch item {
	case "sell_scrap":
		if scrap < 100.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Scrap to convert."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - 100.0, dollars = dollars + 50.0 WHERE encampment_id = $1", campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "💵 Exchanged 100 Scrap for $50.0 Cash!"})

	case "buy_steel":
		if dollars < 100.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Funds! Cost is $100."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars - 100.0, steel = steel + 50.0 WHERE encampment_id = $1", campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "🧱 Purchased 50 tons of Steel!"})

	case "buy_uranium":
		if dollars < 200.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Funds! Cost is $200."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars - 200.0, uranium = uranium + 20.0 WHERE encampment_id = $1", campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "☢️ Purchased 20 kg of Uranium!"})

	case "buy_hydrogen":
		if dollars < 150.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Funds! Cost is $150."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars - 150.0, hydrogen = hydrogen + 40.0 WHERE encampment_id = $1", campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "🎈 Purchased 40 L of Hydrogen Fuel!"})
	}

	_ = tx.Commit()
	return h.HandleEconPanel(c)
}
