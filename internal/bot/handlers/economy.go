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

// HandleEconPanel renders the main economy summary HUD (Changes bottom menu to Economy Submenu)
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

	var scrap float64
	_ = h.DB.QueryRowContext(ctx, "SELECT scrap FROM resources WHERE encampment_id = $1", campID).Scan(&scrap)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🏦 SYSTEM ECONOMY & FINANCIAL CENTER\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"Outpost Name: Encampment Ledger\n\n"+
			"LEDGER SUMMARIES:\n"+
			"⚙️ Scrap Reserves: %.1f\n"+
			"🏦 Vault Savings: %.1f Scrap\n"+
			"💳 Credit Debt: %.1f Scrap\n\n"+
			"Select options on your bottom menu deck to access the Vault, Alliances, or Heavy Workshop.",
		scrap, bankBalance, loanAmount,
	)

	// Changes Reply Keyboard context to Economy Submenu cleanly
	return c.Send(panelText, keyboards.EconomyNavigation())
}

// HandleFinancialVault renders only the transaction inline buttons
func (h *EconomyHandler) HandleFinancialVault(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	var bankBalance float64
	var loanAmount float64
	_ = h.DB.QueryRowContext(ctx, "SELECT balance, loan_amount FROM bank_accounts WHERE encampment_id = $1", campID).Scan(&bankBalance, &loanAmount)

	var scrap, dollars float64
	_ = h.DB.QueryRowContext(ctx, "SELECT scrap, dollars FROM resources WHERE encampment_id = $1", campID).Scan(&scrap, &dollars)

	panelText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"🪙 VAULT & CREDIT OVERRIDE\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"💵 Available Cash: $%.1f\n"+
			"⚙️ Scrap Reserves: %.1f\n"+
			"🏦 Vault Savings: %.1f Scrap\n"+
			"💳 Credit Debt: %.1f Scrap\n\n"+
			"Convert rate: Sell 100 Scrap -> Get $50.0\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		dollars, scrap, bankBalance, loanAmount,
	)

	selector := &telebot.ReplyMarkup{}

	btnDeposit := selector.Data("🏦 Deposit 100", "bank_action", "deposit")
	btnBorrow := selector.Data("💳 Borrow 100", "bank_action", "borrow")
	btnSellScrap := selector.Data("💵 Sell 100 Scrap", "market_buy", "sell_scrap")

	selector.Inline(
		selector.Row(btnDeposit, btnBorrow),
		selector.Row(btnSellScrap),
	)

	return c.Send(panelText, selector)
}

// HandleWarehouseReserves renders the complete inventory grid of all 11 resources
func (h *EconomyHandler) HandleWarehouseReserves(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	var scrap, rations, energy, steel, uranium, hydrogen, iron, oil, gold, silver, diamond, dollars float64
	query := `
		SELECT scrap, rations, energy, steel, uranium, hydrogen, iron, oil, gold, silver, diamond, dollars 
		FROM resources 
		WHERE encampment_id = $1`
	
	_ = h.DB.QueryRowContext(ctx, query, campID).Scan(&scrap, &rations, &energy, &steel, &uranium, &hydrogen, &iron, &oil, &gold, &silver, &diamond, &dollars)

	inventoryText := fmt.Sprintf(
		"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"📦 WAREHOUSE RESERVES GRID\n"+
			"━━━━━━━━━━━━━━━━━━━━━━\n"+
			"FINANCIAL STOCK:\n"+
			"💵 Available Cash: $%.1f\n\n"+
			"SURVIVAL MATERIALS:\n"+
			"⚙️ Scrap Metal: %.1f\n"+
			"🥫 Food Rations: %.1f\n"+
			"🔋 Energy Cells: %.1f\n\n"+
			"HEAVY WAR METALS:\n"+
			"🧱 Steel Stock: %.1f tons\n"+
			"☢️ Uranium Stock: %.1f kg\n"+
			"🎈 Hydrogen Stock: %.1f L\n\n"+
			"HIGH-TECH PRECIOUS METALS:\n"+
			"🪨 Iron Stock: %.1f\n"+
			"🛢️ Oil Reserves: %.1f\n"+
			"🪙 Gold Stock: %.1f\n"+
			"🥈 Silver Stock: %.1f\n"+
			"💎 Diamonds Stock: %.1f\n"+
			"━━━━━━━━━━━━━━━━━━━━━━",
		dollars, scrap, rations, energy, steel, uranium, hydrogen, iron, oil, gold, silver, diamond,
	)

	return c.Send(inventoryText)
}

// HandleBankCallback processes transactional actions in the Bank (Dynamic campID lookup)
func (h *EconomyHandler) HandleBankCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()

	action := c.Args()[0]

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
		_ = c.Respond(&telebot.CallbackResponse{Text: "💳 Borrowed 100 Scrap."})
	}

	_ = tx.Commit()
	return h.HandleFinancialVault(c)
}

// HandleMarketCallback processes item acquisitions (Dynamic campID lookup)
func (h *EconomyHandler) HandleMarketCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()

	item := c.Args()[0]

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
	return h.HandleFinancialVault(c)
}