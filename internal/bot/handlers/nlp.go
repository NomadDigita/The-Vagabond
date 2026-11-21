package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/game/nlpcommand"
	"gopkg.in/telebot.v3"
)

const fallbackNotRecognizedMsg = "🤖 SECURE SHELL: Intent not recognized. Please utilize the persistent interface options below."

type NLPHandler struct {
	Onboarding    *OnboardingHandler
	Camp          *CampHandler
	Combat        *CombatHandler
	Econ          *EconomyHandler
	Clan          *ClanHandler
	Hero          *HeroHandler
	Agent         *AgentHandler
	Factory       *FactoryHandler
	Silo          *SiloHandler
	Research      *ResearchHandler
	Exchange      *ExchangeHandler
	World         *WorldHandler
	ScoutMissions *ScoutMissionsHandler
	Interpreter   *nlpcommand.Interpreter
}

func NewNLPHandler(
	onb *OnboardingHandler,
	camp *CampHandler,
	comb *CombatHandler,
	econ *EconomyHandler,
	clan *ClanHandler,
	hero *HeroHandler,
	agent *AgentHandler,
	factory *FactoryHandler,
	silo *SiloHandler,
	research *ResearchHandler,
	exchange *ExchangeHandler,
	world *WorldHandler,
	scoutMissions *ScoutMissionsHandler,
	interpreter *nlpcommand.Interpreter,
) *NLPHandler {
	return &NLPHandler{
		Onboarding:    onb,
		Camp:          camp,
		Combat:        comb,
		Econ:          econ,
		Clan:          clan,
		Hero:          hero,
		Agent:         agent,
		Factory:       factory,
		Silo:          silo,
		Research:      research,
		Exchange:      exchange,
		World:         world,
		ScoutMissions: scoutMissions,
		Interpreter:   interpreter,
	}
}

func (h *NLPHandler) HandleTextMessage(c telebot.Context) error {
	raw := c.Text()
	text := strings.ToLower(raw)

	if text == "📡 terminal hq" || text == "/start" || text == "start" {
		return h.Onboarding.HandleStart(c)
	}
	if text == "⛺ outpost camp" || text == "camp" {
		return h.Camp.HandleCamp(c)
	}
	if text == "⚔️ tactical combat" || text == "combat" || text == "raid" {
		return h.Combat.HandleRaidBoard(c)
	}
	if text == "🏦 system economy" || text == "economy" || text == "bank" {
		return h.Econ.HandleEconPanel(c)
	}
	if text == "🏭 heavy workshop" || text == "workshop" {
		return h.Factory.HandleFactoryPanel(c)
	}

	if text == "🔨 structural upgrades" {
		return h.Camp.HandleStructuralUpgrades(c)
	}
	if text == "👥 hero commander" {
		return h.Hero.HandleHeroPanel(c)
	}
	if text == "🧠 automation agent" {
		return h.Agent.HandleAgent(c)
	}
	if text == "🧬 mutation core" {
		return h.Camp.HandleMutationsPanel(c)
	}
	if text == "⛏️ active mining" {
		return h.Camp.HandleActiveMining(c)
	}
	if text == "🧪 research lab" {
		return h.Research.HandleResearchPanel(c)
	}

	if text == "🛰️ scan targets" {
		return h.Combat.HandleRaidBoard(c)
	}
	if text == "🛸 expedition radar" || text == "radar" {
		return h.Combat.HandleExpeditionRadar(c)
	}
	if text == "📻 wasteland radio" {
		return h.World.HandleWorldFeed(c)
	}

	if text == "🪙 financial vault" {
		return h.Econ.HandleFinancialVault(c)
	}
	if text == "🛡️ clan alliances" {
		return h.Clan.HandleClanPanel(c)
	}
	if text == "💱 market exchange" {
		return h.Exchange.HandleExchangePanel(c)
	}

	if text == "🪖 recruit troops" {
		return h.Factory.HandleRecruitPanel(c)
	}
	if text == "🚗 logistics vehicles" {
		return h.Factory.HandleVehiclesPanel(c)
	}

	if text == "⬅️ back to hq" {
		return h.Onboarding.HandleStart(c)
	}

	if handled, err := h.tryNaturalLanguageCommand(c, raw); handled {
		return err
	}

	if strings.Contains(text, "upgrade") || strings.Contains(text, "build") {
		return h.Camp.HandleStructuralUpgrades(c)
	}
	if strings.Contains(text, "warehouse") || strings.Contains(text, "stock") || strings.Contains(text, "resources") || strings.Contains(text, "inventory") {
		return h.Econ.HandleWarehouseReserves(c)
	}
	if strings.Contains(text, "vault") || strings.Contains(text, "loan") || strings.Contains(text, "deposit") {
		return h.Econ.HandleFinancialVault(c)
	}
	if strings.Contains(text, "scout") || strings.Contains(text, "find") || strings.Contains(text, "spy") {
		return h.Combat.HandleRaidBoard(c)
	}
	if strings.Contains(text, "alliance") || strings.Contains(text, "clan") {
		return h.Clan.HandleClanPanel(c)
	}
	if strings.Contains(text, "help") || strings.Contains(text, "guide") || strings.Contains(text, "tutorial") {
		return h.Onboarding.HandleHelp(c)
	}
	if strings.Contains(text, "mine") || strings.Contains(text, "extract") || strings.Contains(text, "dig") {
		return h.Camp.HandleActiveMining(c)
	}

	return c.Send(fallbackNotRecognizedMsg)
}

func (h *NLPHandler) tryNaturalLanguageCommand(c telebot.Context, text string) (bool, error) {
	if h.Interpreter == nil {
		return false, nil
	}
	sender := c.Sender()
	if sender == nil {
		return false, nil
	}

	result, err := h.Interpreter.Interpret(context.Background(), sender.ID, text)
	if err != nil {
		return false, nil
	}

	if result.Matched {
		return true, h.dispatchParsedCommand(c, result.Command)
	}
	if result.ClarifyText != "" {
		return true, c.Send("🤖 " + result.ClarifyText)
	}
	return false, nil
}

func (h *NLPHandler) dispatchParsedCommand(c telebot.Context, cmd nlpcommand.ParsedCommand) error {
	switch cmd.Action {
	case nlpcommand.ActionCheckResources:
		return h.Econ.HandleWarehouseReserves(c)
	case nlpcommand.ActionCheckScoutStatus:
		if h.ScoutMissions == nil {
			return c.Send(fallbackNotRecognizedMsg)
		}
		return h.ScoutMissions.HandleScoutPanel(c)
	case nlpcommand.ActionListMarketItem:
		return h.confirmListMarketItem(c, cmd)
	case nlpcommand.ActionBuyMarketItem:
		return h.confirmBuyMarketItem(c, cmd)
	case nlpcommand.ActionDispatchScoutMission:
		return h.confirmDispatchScoutMission(c, cmd)
	case nlpcommand.ActionLaunchRaid:
		if h.Combat == nil {
			return c.Send(fallbackNotRecognizedMsg)
		}
		return h.Combat.HandleNLPLaunchRaid(c, cmd.ArgString("target_name"))
	default:
		return c.Send(fallbackNotRecognizedMsg)
	}
}

func (h *NLPHandler) confirmListMarketItem(c telebot.Context, cmd nlpcommand.ParsedCommand) error {
	resource := strings.ToLower(strings.TrimSpace(cmd.ArgString("resource")))
	qty := cmd.ArgInt("quantity")
	askType := strings.ToLower(strings.TrimSpace(cmd.ArgString("ask_type")))
	askQty := cmd.ArgFloat("ask_quantity")
	if askType == "" {
		askType = "dollars"
	}

	if qty <= 0 || askQty <= 0 {
		return c.Send("🤖 I couldn't quite catch the listing details - try something like \"list 300k scrap for $500\" or \"list 200k scrap for 40k metal\".")
	}
	sellColumn, ok := marketResourceColumn(resource)
	if !ok {
		return c.Send(fmt.Sprintf("🤖 %s isn't tradeable on the exchange - try Metal, Crystal, or Scrap.", htmlEscape(capitalizeWord(resource))))
	}
	if askType != "dollars" {
		askColumn, ok := marketResourceColumn(askType)
		if !ok {
			return c.Send(fmt.Sprintf("🤖 %s isn't something I can barter for - try Dollars, Metal, Crystal, or Scrap.", htmlEscape(capitalizeWord(askType))))
		}
		if askColumn == sellColumn {
			return c.Send(fmt.Sprintf("🤖 Can't barter %s for itself - what would you like to ask for instead?", htmlEscape(capitalizeWord(sellColumn))))
		}
	}

	cardText := "🤖 " + htmlBold("CONFIRM LISTING") + "\n" + divider + "\n" +
		fmt.Sprintf("%s List %s %s for %s?\n", resourceEmoji(resource), htmlCode(fmt.Sprintf("%d", qty)), htmlEscape(resource), htmlCode(formatAsk(askType, askQty))) +
		divider

	selector := &telebot.ReplyMarkup{}
	btnConfirm := keyboards.Styled(selector.Data("✅ Confirm", "nlp_c", "mkt", resource, fmt.Sprintf("%d", qty), askType, fmt.Sprintf("%.0f", askQty)), keyboards.StyleSuccess)
	btnCancel := keyboards.Styled(selector.Data("❌ Cancel", "nlp_x", ""), keyboards.StyleDanger)
	return keyboards.SendStyled(c, cardText, [][]keyboards.StyledBtn{{btnCancel, btnConfirm}})
}

func (h *NLPHandler) confirmBuyMarketItem(c telebot.Context, cmd nlpcommand.ParsedCommand) error {
	resource := strings.ToLower(strings.TrimSpace(cmd.ArgString("resource")))
	minQty := cmd.ArgInt("min_quantity")
	maxDollars := cmd.ArgFloat("max_dollars")

	if minQty <= 0 || maxDollars <= 0 {
		return c.Send("🤖 I couldn't quite catch what to buy - try something like \"buy 200 metal for $500\".")
	}
	if _, ok := marketResourceColumn(resource); !ok {
		return c.Send(fmt.Sprintf("🤖 %s isn't tradeable on the exchange - try Metal, Crystal, or Scrap.", htmlEscape(capitalizeWord(resource))))
	}

	cardText := "🤖 " + htmlBold("CONFIRM PURCHASE") + "\n" + divider + "\n" +
		fmt.Sprintf("%s Buy %s %s from the cheapest matching listing for up to %s?\n", resourceEmoji(resource), htmlCode(fmt.Sprintf("%d", minQty)), htmlEscape(resource), htmlCode(fmt.Sprintf("$%.0f", maxDollars))) +
		htmlItalic("If the matching listing has more than you asked for, only your requested amount is bought - the seller keeps the rest listed. The exact price will be shown once a match is found.") + "\n" +
		divider

	selector := &telebot.ReplyMarkup{}
	btnConfirm := keyboards.Styled(selector.Data("✅ Confirm", "nlp_c", "buy", resource, fmt.Sprintf("%d", minQty), fmt.Sprintf("%.0f", maxDollars)), keyboards.StyleSuccess)
	btnCancel := keyboards.Styled(selector.Data("❌ Cancel", "nlp_x", ""), keyboards.StyleDanger)
	return keyboards.SendStyled(c, cardText, [][]keyboards.StyledBtn{{btnCancel, btnConfirm}})
}

func (h *NLPHandler) confirmDispatchScoutMission(c telebot.Context, cmd nlpcommand.ParsedCommand) error {
	count := cmd.ArgInt("count")
	if count <= 0 {
		return c.Send("🤖 How many scouts would you like to send? Try \"dispatch 5 scouts\".")
	}

	cardText := "🤖 " + htmlBold("CONFIRM SCOUT DISPATCH") + "\n" + divider + "\n" +
		fmt.Sprintf("🔭 Commit %s Scout Walkers to a long-range search?\n", htmlCode(fmt.Sprintf("%d", count))) +
		divider

	selector := &telebot.ReplyMarkup{}
	btnConfirm := keyboards.Styled(selector.Data("✅ Confirm", "nlp_c", "sct", fmt.Sprintf("%d", count)), keyboards.StyleSuccess)
	btnCancel := keyboards.Styled(selector.Data("❌ Cancel", "nlp_x", ""), keyboards.StyleDanger)
	return keyboards.SendStyled(c, cardText, [][]keyboards.StyledBtn{{btnCancel, btnConfirm}})
}

func nlpCardOutcome(c telebot.Context, cardText, toastText string) error {
	_ = c.Respond(&telebot.CallbackResponse{Text: toastText})
	return c.Edit(cardText, telebot.ModeHTML)
}

func (h *NLPHandler) HandleNLPConfirmCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 1 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid confirmation."})
	}

	switch c.Args()[0] {
	case "mkt":
		if h.Exchange == nil || len(c.Args()) < 5 {
			return nlpCardOutcome(c, "❌ Invalid listing.", "❌ Invalid listing.")
		}
		resource := c.Args()[1]
		qty, qtyErr := strconv.Atoi(c.Args()[2])
		askType := c.Args()[3]
		askQty, askQtyErr := strconv.ParseFloat(c.Args()[4], 64)
		if qtyErr != nil || askQtyErr != nil {
			return nlpCardOutcome(c, "❌ Invalid listing details.", "❌ Invalid listing details.")
		}

		var campID string
		if err := h.Exchange.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
			return nlpCardOutcome(c, "⚠️ Create your outpost camp first using /start", "⚠️ No outpost found")
		}

		if _, err := h.Exchange.doPostListing(ctx, campID, resource, qty, askType, askQty); err != nil {
			return nlpCardOutcome(c, "❌ Could not list - see the Exchange panel for details.", "❌ Could not list")
		}
		if err := nlpCardOutcome(c, "💱 Listing posted!", "💱 Listing posted!"); err != nil {
			return err
		}
		return h.Exchange.HandleExchangePanel(c)

	case "buy":
		if h.Exchange == nil || len(c.Args()) < 4 {
			return nlpCardOutcome(c, "❌ Invalid purchase request.", "❌ Invalid purchase request.")
		}
		resource := c.Args()[1]
		minQty, qtyErr := strconv.Atoi(c.Args()[2])
		maxDollars, dollarsErr := strconv.ParseFloat(c.Args()[3], 64)
		if qtyErr != nil || dollarsErr != nil {
			return nlpCardOutcome(c, "❌ Invalid purchase details.", "❌ Invalid purchase details.")
		}

		var campID string
		if err := h.Exchange.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
			return nlpCardOutcome(c, "⚠️ Create your outpost camp first using /start", "⚠️ No outpost found")
		}

		if _, err := h.Exchange.doBuyMarketItem(ctx, campID, resource, minQty, maxDollars); err != nil {
			return nlpCardOutcome(c,
				"❌ No matching listing was found within that budget.\nCheck the Exchange panel to see what's actually available.",
				"❌ No matching listing found")
		}
		if err := nlpCardOutcome(c, "🛍️ Purchase complete!", "🛍️ Purchase complete!"); err != nil {
			return err
		}
		return h.Exchange.HandleExchangePanel(c)

	case "sct":
		if h.ScoutMissions == nil || len(c.Args()) < 2 {
			return nlpCardOutcome(c, "❌ Invalid scout dispatch.", "❌ Invalid scout dispatch.")
		}
		count, err := strconv.Atoi(c.Args()[1])
		if err != nil || count <= 0 {
			return nlpCardOutcome(c, "❌ Invalid scout count.", "❌ Invalid scout count.")
		}

		campID, err := h.ScoutMissions.myScoutCamp(ctx, sender.ID)
		if err != nil {
			return nlpCardOutcome(c, "⚠️ Create your outpost camp first using /start", "⚠️ No outpost found")
		}

		if _, err := h.ScoutMissions.doDispatchScoutMission(ctx, campID, count); err != nil {
			return nlpCardOutcome(c, "❌ Could not dispatch - see the Scout panel for details.", "❌ Could not dispatch")
		}
		if err := nlpCardOutcome(c, fmt.Sprintf("🔭 %d Scout Walkers dispatched!", count), fmt.Sprintf("🔭 %d dispatched!", count)); err != nil {
			return err
		}
		return h.ScoutMissions.HandleScoutPanel(c)

	default:
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Unknown confirmation."})
	}
}

func (h *NLPHandler) HandleNLPCancelCallback(c telebot.Context) error {
	return nlpCardOutcome(c, "❌ Cancelled - nothing was changed.", "❌ Cancelled")
}
