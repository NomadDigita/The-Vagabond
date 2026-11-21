package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/prestige"
	"gopkg.in/telebot.v3"
)

type PrestigeHandler struct {
	DB *sql.DB
}

func NewPrestigeHandler(db *sql.DB) *PrestigeHandler {
	return &PrestigeHandler{DB: db}
}

func (h *PrestigeHandler) HandlePrestigePanel(c telebot.Context) error {
	_ = c.Notify(telebot.Typing)
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}

	var campID string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
		return c.Send("⚠️ Create your outpost camp first using /start", keyboards.MainNavigation())
	}

	var prestigeLevel int
	var bonusPct float64
	_ = h.DB.QueryRowContext(ctx, "SELECT prestige_level, prestige_bonus_pct FROM encampments WHERE id = $1", campID).Scan(&prestigeLevel, &bonusPct)

	panelText := "♻️ " + htmlBold("PRESTIGE") + "\n" + divider + "\n" +
		fmt.Sprintf("🏅 Prestige Level: %s\n💠 Permanent Bonus: %s\n\n", htmlCode(fmt.Sprintf("%d", prestigeLevel)), htmlCode(fmt.Sprintf("+%.0f%%", bonusPct))) +
		htmlItalic("Reset your Outpost level, buildings, and research back to the start, and your resources back to a fresh account's baseline, in exchange for a permanent +5% resource-generation bonus that stacks forever. Your gear, achievements, Hero, Clan membership, and Legacy/Relic history are NOT affected.") + "\n\n"

	selector := &telebot.ReplyMarkup{}
	var buttons []telebot.Row

	eligibility, err := prestige.CheckEligibility(ctx, h.DB, campID)
	if err != nil {
		panelText += "📡 " + htmlItalic("Static: Connection interrupted.") + "\n"
	} else if eligibility.Eligible {
		panelText += "✅ " + htmlBold("ELIGIBLE TO PRESTIGE") + "\n\n"
		buttons = append(buttons, selector.Row(selector.Data("♻️ Prestige Now", "prestige_confirm_prompt")))
	} else {
		panelText += htmlBold("🔒 NOT YET ELIGIBLE - MISSING:") + "\n"
		for _, m := range eligibility.Missing {
			panelText += fmt.Sprintf("  • %s\n", htmlEscape(m))
		}
		panelText += "\n"
	}

	panelText += divider
	selector.Inline(buttons...)

	return sendPanelWithNavHTML(c, navCaptionCamp, keyboards.CampNavigation(), panelText, selector)
}

func (h *PrestigeHandler) HandlePrestigeConfirmPromptCallback(c telebot.Context) error {
	_ = c.Respond(&telebot.CallbackResponse{})
	selector := &telebot.ReplyMarkup{}
	btnConfirm := keyboards.Styled(selector.Data("♻️ Confirm Prestige", "prestige_confirm", "yes"), keyboards.StyleDanger)
	btnCancel := keyboards.Styled(selector.Data("❌ Cancel", "prestige_confirm", "no"), keyboards.StyleSuccess)

	text := "♻️ " + htmlBold("CONFIRM PRESTIGE - THIS CANNOT BE UNDONE") + "\n" + divider + "\n" +
		"RESETS: Outpost level, all buildings, all research, resources → starting baseline.\n" +
		"KEPT: Gear, Achievements, Hero (level/skills/story), Clan membership, Legacy/Relic history, garrison.\n" +
		"GAIN: +5% permanent resource-generation bonus, stacking forever.\n" + divider

	return keyboards.SendStyled(c, text, [][]keyboards.StyledBtn{{btnConfirm, btnCancel}})
}

func (h *PrestigeHandler) HandlePrestigeConfirmCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 1 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid request."})
	}
	if c.Args()[0] != "yes" {
		_ = c.Respond(&telebot.CallbackResponse{Text: "❌ Prestige cancelled."})
		return h.HandlePrestigePanel(c)
	}

	var campID string
	if err := h.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
	}

	tx, err := h.DB.BeginTx(ctx, nil)
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error starting Prestige."})
	}
	defer tx.Rollback()

	if err := prestige.DoPrestige(ctx, tx, campID); err != nil {
		return c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: "❌ Not eligible to Prestige right now - check /prestige for what's missing."})
	}
	if err := tx.Commit(); err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Error committing Prestige."})
	}

	_ = c.Respond(&telebot.CallbackResponse{ShowAlert: true, Text: "♻️ Prestige complete! Your permanent bonus has increased."})
	if sendErr := c.Send("♻️✨ "+htmlBold("PRESTIGE COMPLETE!")+" Your Outpost has been reborn - a new permanent bonus now applies to everything you rebuild.", telebot.ModeHTML, keyboards.MainNavigation()); sendErr != nil {
		return sendErr
	}
	return h.HandlePrestigePanel(c)
}
