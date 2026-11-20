package handlers

import (
	"context"
	"errors"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/researchplanner"
	"gopkg.in/telebot.v3"
)

type ResearchPlannerHandler struct {
	Planner *researchplanner.Planner
}

func NewResearchPlannerHandler(p *researchplanner.Planner) *ResearchPlannerHandler {
	return &ResearchPlannerHandler{Planner: p}
}

var goalLabels = map[researchplanner.Goal]string{
	researchplanner.GoalRaiding:  "⚔️ Raiding",
	researchplanner.GoalDefense:  "🛡️ Defense",
	researchplanner.GoalEconomy:  "💹 Economy",
	researchplanner.GoalBalanced: "⚖️ Balanced",
}

func buildResearchPlannerKeyboard() *telebot.ReplyMarkup {
	selector := &telebot.ReplyMarkup{}
	btnRefresh := selector.Data("🔄 Refresh Analysis", "research_refresh")

	goalBtns := make([]telebot.Btn, 0, len(researchplanner.ValidGoals()))
	for _, g := range researchplanner.ValidGoals() {
		goalBtns = append(goalBtns, selector.Data(goalLabels[g], "research_goal", string(g)))
	}

	selector.Inline(
		selector.Row(btnRefresh),
		selector.Row(goalBtns...),
	)
	return selector
}

func (h *ResearchPlannerHandler) renderResearchPlannerReport(ctx context.Context, userID int64, goal string) (string, *telebot.ReplyMarkup, error) {
	rec, err := h.Planner.Recommend(ctx, userID, goal)
	if err != nil {
		return "", nil, err
	}
	return researchplanner.FormatForTelegram(rec), buildResearchPlannerKeyboard(), nil
}

func (h *ResearchPlannerHandler) HandleResearchPlanner(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	_ = c.Notify(telebot.Typing)

	goal := strings.ToLower(strings.TrimSpace(c.Message().Payload))

	ctx := context.Background()
	text, keyboard, err := h.renderResearchPlannerReport(ctx, sender.ID, goal)
	if errors.Is(err, researchplanner.ErrNoEncampment) {
		return c.Send("❌ You don't have an outpost yet. Use /start to establish one first.", keyboards.AdvisorsNavigation())
	}
	if err != nil {
		return c.Send("⚠️ The AI Research Planner is temporarily unavailable: "+err.Error(), keyboards.AdvisorsNavigation())
	}

	return c.Send(text, telebot.ModeHTML, keyboard, keyboards.AdvisorsNavigation())
}

func (h *ResearchPlannerHandler) HandleResearchPlannerRefreshCallback(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	ctx := context.Background()

	text, keyboard, err := h.renderResearchPlannerReport(ctx, sender.ID, "")
	if errors.Is(err, researchplanner.ErrNoEncampment) {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ You don't have an outpost yet."})
	}
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Research Planner unavailable: " + err.Error()})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🔄 Analysis refreshed."})
	return c.Send(text, telebot.ModeHTML, keyboard)
}

func (h *ResearchPlannerHandler) HandleResearchPlannerGoalCallback(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	ctx := context.Background()

	args := c.Args()
	if len(args) == 0 || !researchplanner.IsValidGoal(args[0]) {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Unknown goal."})
	}
	goal := args[0]

	text, keyboard, err := h.renderResearchPlannerReport(ctx, sender.ID, goal)
	if errors.Is(err, researchplanner.ErrNoEncampment) {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ You don't have an outpost yet."})
	}
	if err != nil {
		return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Research Planner unavailable: " + err.Error()})
	}

	_ = c.Respond(&telebot.CallbackResponse{Text: "🎯 Goal set: " + goal})
	return c.Send(text, telebot.ModeHTML, keyboard)
}
