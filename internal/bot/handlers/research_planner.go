package handlers

import (
	"context"
	"errors"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/game/researchplanner"
	"gopkg.in/telebot.v3"
)

// ResearchPlannerHandler exposes Phase E (AI Research Planner). New
// command name (/research_planner) — no collision with any SpaceHunt
// phase 1-6 command or the existing /research panel.
type ResearchPlannerHandler struct {
	Planner *researchplanner.Planner
}

func NewResearchPlannerHandler(p *researchplanner.Planner) *ResearchPlannerHandler {
	return &ResearchPlannerHandler{Planner: p}
}

// goalLabels maps each Goal to its inline-button display text.
var goalLabels = map[researchplanner.Goal]string{
	researchplanner.GoalRaiding:  "⚔️ Raiding",
	researchplanner.GoalDefense:  "🛡️ Defense",
	researchplanner.GoalEconomy:  "💹 Economy",
	researchplanner.GoalBalanced: "⚖️ Balanced",
}

// buildResearchPlannerKeyboard renders the inline keyboard attached to
// every Research Planner report: a refresh button (re-runs with
// whatever goal was last used/inferred) and one button per goal so the
// player can steer future recommendations without typing a command.
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

// renderResearchPlannerReport runs a fresh recommendation and returns
// the formatted text plus its attached keyboard, shared by the
// /research_planner command and both callbacks so the three can never
// drift apart. goal may be empty (infer it).
func (h *ResearchPlannerHandler) renderResearchPlannerReport(ctx context.Context, userID int64, goal string) (string, *telebot.ReplyMarkup, error) {
	rec, err := h.Planner.Recommend(ctx, userID, goal)
	if err != nil {
		return "", nil, err
	}
	return researchplanner.FormatForTelegram(rec), buildResearchPlannerKeyboard(), nil
}

// ── /research_planner [goal] ────────────────────────────────────────
//
// Analyzes the player's tech tree and Neuro Core stockpile, and
// recommends the best research order for a stated or inferred
// strategic goal (raiding, defense, economy, balanced). Read-only:
// never spends Neuro Cores or upgrades any tech node on the player's
// behalf — /research remains the only place that actually happens.
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
		return c.Send("❌ You don't have an outpost yet. Use /start to establish one first.")
	}
	if err != nil {
		return c.Send("⚠️ The AI Research Planner is temporarily unavailable: " + err.Error())
	}

	return c.Send(text, keyboard)
}

// ── callback: research_refresh ──────────────────────────────────────
//
// Re-runs the analysis on demand with an inferred goal (a real new AI
// Foundation call, subject to the usual cost/cache/budget rules) and
// posts a fresh report.
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
	return c.Send(text, keyboard)
}

// ── callback: research_goal <goal> ──────────────────────────────────
//
// Re-runs the analysis pinned to whichever goal button the player
// tapped, overriding inference for that one report.
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
	return c.Send(text, keyboard)
}
