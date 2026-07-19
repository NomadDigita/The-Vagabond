package handlers

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/game/devconsole"
	"gopkg.in/telebot.v3"
)

// defaultReportWindowDays is used when /weekly_report is called with
// no argument.
const defaultReportWindowDays = 7

// DevConsoleHandler exposes Phase J (AI Developer Console). New
// command (/weekly_report). Admin-only, using the exact same
// AdminIDs/IsAdmin gate every other admin-only action in
// internal/bot/handlers/admin.go already uses — this reports on the
// whole game's player activity, not any one player's own data.
type DevConsoleHandler struct {
	Console  *devconsole.Console
	AdminIDs []int64
}

func NewDevConsoleHandler(console *devconsole.Console, adminIDs []int64) *DevConsoleHandler {
	return &DevConsoleHandler{Console: console, AdminIDs: adminIDs}
}

// IsAdmin mirrors AdminHandler.IsAdmin exactly (see admin.go) — kept
// as its own small copy rather than a cross-package call, consistent
// with every other Phase B-I package's independence from one another.
func (h *DevConsoleHandler) IsAdmin(senderID int64) bool {
	for _, id := range h.AdminIDs {
		if id == senderID {
			return true
		}
	}
	return false
}

func buildDevConsoleKeyboard(windowDays int) *telebot.ReplyMarkup {
	selector := &telebot.ReplyMarkup{}
	btnRefresh := selector.Data("🔄 Refresh Report", "dev_console_refresh", strconv.Itoa(windowDays))
	selector.Inline(selector.Row(btnRefresh))
	return selector
}

func buildBalanceKeyboard(windowDays int) *telebot.ReplyMarkup {
	selector := &telebot.ReplyMarkup{}
	btnRefresh := selector.Data("🔄 Refresh", "balance_report_refresh", strconv.Itoa(windowDays))
	selector.Inline(selector.Row(btnRefresh))
	return selector
}

func (h *DevConsoleHandler) renderReport(ctx context.Context, adminID int64, windowDays int) (string, *telebot.ReplyMarkup, error) {
	rec, err := h.Console.Recommend(ctx, adminID, windowDays)
	if err != nil {
		return "", nil, err
	}
	return devconsole.FormatForTelegram(rec), buildDevConsoleKeyboard(windowDays), nil
}

// ── /weekly_report [days] ────────────────────────────────────────────
//
// Admin-only. Summarizes new player signups, top players, active
// users, and recent world news over the given window (default 7
// days). Read-only: never changes any player, setting, or game data.
func (h *DevConsoleHandler) HandleWeeklyReport(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.")
	}
	_ = c.Notify(telebot.Typing)

	windowDays := defaultReportWindowDays
	if arg := strings.TrimSpace(c.Message().Payload); arg != "" {
		if n, err := strconv.Atoi(arg); err == nil && n > 0 {
			windowDays = n
		}
	}

	ctx := context.Background()
	text, keyboard, err := h.renderReport(ctx, sender.ID, windowDays)
	if err != nil {
		return c.Send("⚠️ The AI Developer Console is temporarily unavailable: " + err.Error())
	}

	return c.Send(text, keyboard)
}

// ── callback: dev_console_refresh ────────────────────────────────────
//
// Re-runs the report for the same window on demand (a real new AI
// Foundation call, subject to the usual cost/cache/budget rules).
func (h *DevConsoleHandler) HandleDevConsoleRefreshCallback(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	if !h.IsAdmin(sender.ID) {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Access Denied."})
	}

	windowDays := defaultReportWindowDays
	if args := c.Args(); len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(args[0])); err == nil && n > 0 {
			windowDays = n
		}
	}

	ctx := context.Background()
	text, keyboard, err := h.renderReport(ctx, sender.ID, windowDays)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Report unavailable: " + err.Error()})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🔄 Report refreshed."})
	return c.Send(text, keyboard)
}

// ── /admin_ask <question> ────────────────────────────────────────────
//
// Admin-only. Natural-language question about game state, answered
// via the classify → execute → answer flow in devconsole.Ask. See
// devconsole/queries.go's doc comment for the safety model: the model
// never writes or executes SQL, only picks from a fixed whitelist of
// read-only queries.
func (h *DevConsoleHandler) HandleAdminAsk(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.")
	}

	question := strings.TrimSpace(c.Message().Payload)
	if question == "" {
		return c.Send("Usage: /admin_ask <question>\n\nExamples:\n  /admin_ask how many new players this week?\n  /admin_ask who are the top 10 players right now?\n  /admin_ask is anything unusual happening with the economy?")
	}

	_ = c.Notify(telebot.Typing)
	ctx := context.Background()

	rec, err := h.Console.Ask(ctx, sender.ID, question)
	if err != nil {
		return c.Send("⚠️ The AI Developer Console couldn't answer that: " + err.Error())
	}

	return c.Send(devconsole.FormatAnswerForTelegram(rec))
}

// ── /balance_report [days] ───────────────────────────────────────────
//
// Admin-only. Real unit usage/outcome correlations from completed
// raids, with AI commentary. See devconsole/balance.go's doc comment
// for the correlational-not-causal framing this is built around.
func (h *DevConsoleHandler) HandleBalanceReport(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.")
	}
	_ = c.Notify(telebot.Typing)

	windowDays := defaultReportWindowDays
	if arg := strings.TrimSpace(c.Message().Payload); arg != "" {
		if n, err := strconv.Atoi(arg); err == nil && n > 0 {
			windowDays = n
		}
	}

	ctx := context.Background()
	rec, err := h.Console.RecommendBalance(ctx, sender.ID, windowDays)
	if err != nil {
		return c.Send("⚠️ The AI Developer Console is temporarily unavailable: " + err.Error())
	}

	return c.Send(devconsole.FormatBalanceForTelegram(rec), buildBalanceKeyboard(windowDays))
}

// ── callback: balance_report_refresh ─────────────────────────────────
func (h *DevConsoleHandler) HandleBalanceReportRefreshCallback(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	if !h.IsAdmin(sender.ID) {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Access Denied."})
	}

	windowDays := defaultReportWindowDays
	if args := c.Args(); len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(args[0])); err == nil && n > 0 {
			windowDays = n
		}
	}

	ctx := context.Background()
	rec, err := h.Console.RecommendBalance(ctx, sender.ID, windowDays)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Report unavailable: " + err.Error()})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🔄 Refreshed."})
	return c.Send(devconsole.FormatBalanceForTelegram(rec), buildBalanceKeyboard(windowDays))
}
