package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
	"gopkg.in/telebot.v3"
)

type EconomyHandler struct {
	DB              *sql.DB
	exchangeHandler *ExchangeHandler
}

func NewEconomyHandler(db *sql.DB, exchangeHandler *ExchangeHandler) *EconomyHandler {
	return &EconomyHandler{DB: db, exchangeHandler: exchangeHandler}
}

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
	var bankBalanceCash float64
	var loanAmount float64
	var loanCash float64
	queryBank := `SELECT balance, balance_cash, loan_amount, loan_cash FROM bank_accounts WHERE encampment_id = $1`
	err = h.DB.QueryRowContext(ctx, queryBank, campID).Scan(&bankBalance, &bankBalanceCash, &loanAmount, &loanCash)
	if errors.Is(err, sql.ErrNoRows) {
		_, _ = h.DB.ExecContext(ctx, "INSERT INTO bank_accounts (encampment_id, balance, balance_cash, loan_amount, loan_cash) VALUES ($1, 0.00, 0.00, 0.00, 0.00)", campID)
		bankBalance = 0.0
		bankBalanceCash = 0.0
		loanAmount = 0.0
		loanCash = 0.0
	}

	var scrap, metal, crystal, dollars float64
	_ = h.DB.QueryRowContext(ctx, "SELECT scrap, metal, crystal, dollars FROM resources WHERE encampment_id = $1", campID).Scan(&scrap, &metal, &crystal, &dollars)

	var activeListings int
	_ = h.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM market_exchange WHERE is_sold = FALSE").Scan(&activeListings)

	panelText := fmt.Sprintf(
		"🏪━━━━━━━━━━━━━━━━━━━━━━🏪\n"+
			"💱 THE TRADE HUB 💱\n"+
			"🏪━━━━━━━━━━━━━━━━━━━━━━🏪\n\n"+
			"💰 YOUR WALLET:\n"+
			"⚙️ Scrap: %.1f  |  🔩 Metal: %.1f\n"+
			"💎 Crystal: %.1f  |  💵 Cash: $%.1f\n\n"+
			"🏦 BANK VAULT SNAPSHOT:\n"+
			"💰 Savings: %.1f Scrap | $%.1f Cash\n"+
			"💳 Debt: %.1f Scrap | $%.1f Cash\n\n"+
			"🛒 PLAYER MARKET SNAPSHOT:\n"+
			"📋 Active Listings: %d\n\n"+
			"Choose where to trade:\n"+
			"🏦 [Financial Vault] — Deposit, borrow, repay, and convert Scrap/Metal/Crystal into Cash\n"+
			"🛒 [Market Exchange] — Buy and sell directly with other survivors\n"+
			"🏪━━━━━━━━━━━━━━━━━━━━━━🏪",
		scrap, metal, crystal, dollars,
		bankBalance, bankBalanceCash, loanAmount, loanCash,
		activeListings,
	)

	selector := &telebot.ReplyMarkup{}
	btnVault := selector.Data("🏦 Financial Vault", "trade_hub_nav", "vault")
	btnMarket := selector.Data("🛒 Market Exchange", "trade_hub_nav", "market")

	selector.Inline(selector.Row(btnVault, btnMarket))

	return sendPanelWithNav(c, navCaptionEconomy, keyboards.EconomyNavigation(), panelText, selector)
}
// buttons directly into the Vault or Market sub-panels, so players don't
// have to hunt through the bottom reply-keyboard menu to get there.
func (h *EconomyHandler) HandleTradeHubNavCallback(c telebot.Context) error {
	dest := c.Args()[0]
	if dest == "vault" {
		return h.HandleFinancialVault(c)
	}
	return h.exchangeHandler.HandleExchangePanel(c)
}

func (h *EconomyHandler) HandleFinancialVault(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	var bankBalance, bankBalanceCash, loanAmount, loanCash float64
	_ = h.DB.QueryRowContext(ctx, "SELECT balance, balance_cash, loan_amount, loan_cash FROM bank_accounts WHERE encampment_id = $1", campID).Scan(&bankBalance, &bankBalanceCash, &loanAmount, &loanCash)

	var scrap, dollars float64
	_ = h.DB.QueryRowContext(ctx, "SELECT scrap, dollars FROM resources WHERE encampment_id = $1", campID).Scan(&scrap, &dollars)

	panelText := fmt.Sprintf(
		"🏦━━━━━━━━━━━━━━━━━━━━━━🏦\n"+
			"🪙 BANK VAULT & CREDIT PAYBACK 🪙\n"+
			"🏦━━━━━━━━━━━━━━━━━━━━━━🏦\n"+
			"💵 Available Cash: $%.1f\n"+
			"⚙️ Scrap Reserves: %.1f\n\n"+
			"🏦 Vault Savings: %.1f Scrap | $%.1f Cash\n"+
			"💳 Credit Debt: %.1f Scrap | $%.1f Cash\n\n"+
			"💱 CONVERSION RATES:\n"+
			"⚙️ 100 Scrap ➜ $50\n"+
			"🔩 100 Metal ➜ $80\n"+
			"💎 50 Crystal ➜ $120\n"+
			"$100 ➜ 🔩 50 Metal\n"+
			"$200 ➜ 💎 20 Crystal\n"+
			"$150 ➜ 🎈 40 Hydrogen\n"+
			"🏦━━━━━━━━━━━━━━━━━━━━━━🏦",
		dollars, scrap, bankBalance, bankBalanceCash, loanAmount, loanCash,
	)

	selector := &telebot.ReplyMarkup{}

	btnDepositScrap := selector.Data("🏦 Deposit 100 Scrap", "bank_action", "deposit_scrap")
	btnDepositCash := selector.Data("🏦 Deposit $100 Cash", "bank_action", "deposit_cash")
	btnBorrowScrap := selector.Data("💳 Borrow 100 Scrap", "bank_action", "borrow_scrap")
	btnBorrowCash := selector.Data("💳 Borrow $100 Cash", "bank_action", "borrow_cash")
	btnRepayScrap := selector.Data("💳 Repay 100 Scrap", "bank_action", "repay_scrap")
	btnRepayCash := selector.Data("💳 Repay $100 Cash", "bank_action", "repay_cash")
	btnSellScrap := selector.Data("💵 Sell 100 Scrap", "market_buy", "sell_scrap")
	btnSellMetal := selector.Data("💵 Sell 100 Metal", "market_buy", "sell_metal")
	btnSellCrystal := selector.Data("💵 Sell 50 Crystal", "market_buy", "sell_crystal")
	btnBuyMetal := selector.Data("🔩 Buy 50 Metal ($100)", "market_buy", "buy_metal")
	btnBuyCrystal := selector.Data("💎 Buy 20 Crystal ($200)", "market_buy", "buy_crystal")
	btnBuyHydrogen := selector.Data("🎈 Buy 40 Hydrogen ($150)", "market_buy", "buy_hydrogen")

	selector.Inline(
		selector.Row(btnDepositScrap, btnDepositCash),
		selector.Row(btnBorrowScrap, btnBorrowCash),
		selector.Row(btnRepayScrap, btnRepayCash),
		selector.Row(btnSellScrap, btnSellMetal, btnSellCrystal),
		selector.Row(btnBuyMetal, btnBuyCrystal),
		selector.Row(btnBuyHydrogen),
	)

	return sendPanelWithNav(c, navCaptionEconomy, keyboards.EconomyNavigation(), panelText, selector)
}

func (h *EconomyHandler) HandleWarehouseReserves(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)

	sender := c.Sender()
	ctx := context.Background()

	var campID string
	_ = h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID)

	var scrap, rations, electricity, metal, crystal, hydrogen, dollars float64
	query := `
		SELECT scrap, rations, electricity, metal, crystal, hydrogen, dollars 
		FROM resources 
		WHERE encampment_id = $1`

	_ = h.DB.QueryRowContext(ctx, query, campID).Scan(&scrap, &rations, &electricity, &metal, &crystal, &hydrogen, &dollars)

	inventoryText := fmt.Sprintf(
		"📦━━━━━━━━━━━━━━━━━━━━━━📦\n"+
			"📦 WAREHOUSE RESERVES GRID 📦\n"+
			"📦━━━━━━━━━━━━━━━━━━━━━━📦\n\n"+
			"💰 FINANCIAL STOCK:\n"+
			"💵 Available Cash: $%.1f\n\n"+
			"🥫 SURVIVAL MATERIALS:\n"+
			"⚙️ Scrap: %.1f\n"+
			"🥫 Food Rations: %.1f\n"+
			"⚡ Electricity: %.1f cells\n\n"+
			"🏗️ CORE SPACEHUNT RESOURCES:\n"+
			"🔩 Metal: %.1f tons\n"+
			"💎 Crystal: %.1f kg\n"+
			"🎈 Hydrogen: %.1f L\n"+
			"📦━━━━━━━━━━━━━━━━━━━━━━📦",
		dollars, scrap, rations, electricity, metal, crystal, hydrogen,
	)

	return c.Send(inventoryText, keyboards.EconomyNavigation())
}

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

	var scrap, dollars float64
	_ = tx.QueryRowContext(ctx, "SELECT scrap, dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&scrap, &dollars)

	var balance, balanceCash, loanAmount, loanCash float64
	_ = tx.QueryRowContext(ctx, "SELECT balance, balance_cash, loan_amount, loan_cash FROM bank_accounts WHERE encampment_id = $1 FOR UPDATE", campID).Scan(&balance, &balanceCash, &loanAmount, &loanCash)

	// Loan draws land in the same resources table as everything else,
	// so they're subject to the same outpost storage cap as any other gain.
	storageCap := storagecap.CapFor(ctx, tx, campID)

	switch action {
	case "deposit_scrap", "deposit":
		if scrap < 100.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Scrap: Need at least 100 Scrap."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - 100.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE bank_accounts SET balance = balance + 100.0 WHERE encampment_id = $1", campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "🏦 Deposited 100 Scrap into savings."})

	case "deposit_cash":
		if dollars < 100.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Cash: Need at least $100 Cash."})
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars - 100.0 WHERE encampment_id = $1", campID)
		_, _ = tx.ExecContext(ctx, "UPDATE bank_accounts SET balance_cash = balance_cash + 100.0 WHERE encampment_id = $1", campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "🏦 Deposited $100 Cash into savings."})

	case "borrow_scrap", "borrow":
		if loanAmount >= 500.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Credit Limit Reached: Repay existing scrap debt first."})
		}
		newScrap, _ := storagecap.Clamp(scrap, 100.0, storageCap)
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = $1 WHERE encampment_id = $2", newScrap, campID)
		_, _ = tx.ExecContext(ctx, "UPDATE bank_accounts SET loan_amount = loan_amount + 100.0 WHERE encampment_id = $1", campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "💳 Borrowed 100 Scrap."})

	case "borrow_cash":
		if loanCash >= 500.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Credit Limit Reached: Repay existing cash debt first."})
		}
		newDollars, _ := storagecap.Clamp(dollars, 100.0, storageCap)
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = $1 WHERE encampment_id = $2", newDollars, campID)
		_, _ = tx.ExecContext(ctx, "UPDATE bank_accounts SET loan_cash = loan_cash + 100.0 WHERE encampment_id = $1", campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "💳 Borrowed $100 Cash."})

	case "repay_scrap":
		if scrap < 100.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Scrap: Repaying requires at least 100 Scrap."})
		}
		if loanAmount <= 0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ No Debt: You have no active scrap credit debt to repay."})
		}
		repayAmt := 100.0
		if loanAmount < 100.0 {
			repayAmt = loanAmount
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - $1 WHERE encampment_id = $2", repayAmt, campID)
		_, _ = tx.ExecContext(ctx, "UPDATE bank_accounts SET loan_amount = loan_amount - $1 WHERE encampment_id = $2", repayAmt, campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("💳 Repaid %.1f Scrap debt successfully!", repayAmt)})

	case "repay_cash":
		if dollars < 100.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Cash: Repaying requires at least $100 Cash."})
		}
		if loanCash <= 0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ No Debt: You have no active cash credit debt to repay."})
		}
		repayAmt := 100.0
		if loanCash < 100.0 {
			repayAmt = loanCash
		}
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars - $1 WHERE encampment_id = $2", repayAmt, campID)
		_, _ = tx.ExecContext(ctx, "UPDATE bank_accounts SET loan_cash = loan_cash - $1 WHERE encampment_id = $2", repayAmt, campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("💳 Repaid $%.1f Cash debt!", repayAmt)})
	}

	_ = tx.Commit()
	return h.HandleFinancialVault(c)
}

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

	storageCap := storagecap.CapFor(ctx, tx, campID)

	switch item {
	case "sell_scrap":
		if scrap < 100.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Scrap to convert."})
		}
		newDollars, _ := storagecap.Clamp(dollars, 50.0, storageCap)
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET scrap = scrap - 100.0, dollars = $1 WHERE encampment_id = $2", newDollars, campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "💵 Exchanged 100 Scrap for $50.0 Cash!"})

	case "sell_metal":
		var metal float64
		_ = tx.QueryRowContext(ctx, "SELECT metal FROM resources WHERE encampment_id = $1", campID).Scan(&metal)
		if metal < 100.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Metal to convert."})
		}
		newDollars, _ := storagecap.Clamp(dollars, 80.0, storageCap)
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET metal = metal - 100.0, dollars = $1 WHERE encampment_id = $2", newDollars, campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "💵 Exchanged 100 Metal for $80.0 Cash!"})

	case "sell_crystal":
		var crystal float64
		_ = tx.QueryRowContext(ctx, "SELECT crystal FROM resources WHERE encampment_id = $1", campID).Scan(&crystal)
		if crystal < 50.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Crystal to convert."})
		}
		newDollars, _ := storagecap.Clamp(dollars, 120.0, storageCap)
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET crystal = crystal - 50.0, dollars = $1 WHERE encampment_id = $2", newDollars, campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "💵 Exchanged 50 Crystal for $120.0 Cash!"})

	case "buy_metal":
		if dollars < 100.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Funds! Cost is $100."})
		}
		var metal float64
		_ = tx.QueryRowContext(ctx, "SELECT metal FROM resources WHERE encampment_id = $1", campID).Scan(&metal)
		newMetal, _ := storagecap.Clamp(metal, 50.0, storageCap)
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars - 100.0, metal = $1 WHERE encampment_id = $2", newMetal, campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "🔩 Purchased 50 tons of Metal!"})

	case "buy_crystal":
		if dollars < 200.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Funds! Cost is $200."})
		}
		var crystal float64
		_ = tx.QueryRowContext(ctx, "SELECT crystal FROM resources WHERE encampment_id = $1", campID).Scan(&crystal)
		newCrystal, _ := storagecap.Clamp(crystal, 20.0, storageCap)
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars - 200.0, crystal = $1 WHERE encampment_id = $2", newCrystal, campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "💎 Purchased 20 kg of Crystal!"})

	case "buy_hydrogen":
		if dollars < 150.0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Insufficient Funds! Cost is $150."})
		}
		var hydrogen float64
		_ = tx.QueryRowContext(ctx, "SELECT hydrogen FROM resources WHERE encampment_id = $1", campID).Scan(&hydrogen)
		newHydrogen, _ := storagecap.Clamp(hydrogen, 40.0, storageCap)
		_, _ = tx.ExecContext(ctx, "UPDATE resources SET dollars = dollars - 150.0, hydrogen = $1 WHERE encampment_id = $2", newHydrogen, campID)
		_ = c.Respond(&telebot.CallbackResponse{Text: "🎈 Purchased 40 L of Hydrogen Fuel!"})
	}

	_ = tx.Commit()
	return h.HandleFinancialVault(c)
}
