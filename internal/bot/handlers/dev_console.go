package handlers

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/devconsole"
	"gopkg.in/telebot.v3"
)

const defaultReportWindowDays = 7

type DevConsoleHandler struct {
	Console  *devconsole.Console
	AdminIDs []int64
}

func NewDevConsoleHandler(console *devconsole.Console, adminIDs []int64) *DevConsoleHandler {
	return &DevConsoleHandler{Console: console, AdminIDs: adminIDs}
}

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

func (h *DevConsoleHandler) HandleWeeklyReport(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.", keyboards.AdminNavigation())
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
		return c.Send("⚠️ The AI Developer Console is temporarily unavailable: "+err.Error(), keyboards.AdminNavigation())
	}

	return c.Send(text, telebot.ModeHTML, keyboard, keyboards.AdminNavigation())
}

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
	return c.Send(text, telebot.ModeHTML, keyboard)
}

func (h *DevConsoleHandler) HandleAdminAsk(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.", keyboards.AdminNavigation())
	}

	question := strings.TrimSpace(c.Message().Payload)
	if question == "" {
		return c.Send("Usage: /admin_ask <question>\n\nExamples:\n  /admin_ask how many new players this week?\n  /admin_ask who are the top 10 players right now?\n  /admin_ask is anything unusual happening with the economy?", keyboards.AdminNavigation())
	}

	_ = c.Notify(telebot.Typing)
	ctx := context.Background()

	rec, err := h.Console.Ask(ctx, sender.ID, question)
	if err != nil {
		return c.Send("⚠️ The AI Developer Console couldn't answer that: "+err.Error(), keyboards.AdminNavigation())
	}

	return c.Send(devconsole.FormatAnswerForTelegram(rec), keyboards.AdminNavigation())
}

func (h *DevConsoleHandler) HandleBalanceReport(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	if !h.IsAdmin(sender.ID) {
		return c.Send("❌ Access Denied: Authorized administrators only.", keyboards.AdminNavigation())
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
		return c.Send("⚠️ The AI Developer Console is temporarily unavailable: "+err.Error(), keyboards.AdminNavigation())
	}

	return c.Send(devconsole.FormatBalanceForTelegram(rec), telebot.ModeHTML, buildBalanceKeyboard(windowDays), keyboards.AdminNavigation())
}

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
	return c.Send(devconsole.FormatBalanceForTelegram(rec), telebot.ModeHTML, buildBalanceKeyboard(windowDays))
}
