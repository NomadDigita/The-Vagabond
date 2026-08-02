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

// fallbackNotRecognizedMsg is shown when nothing - not a hardcoded
// shortcut, not a lexical pattern, not the AI command interpreter -
// matched the player's free text. Extracted to a constant since the
// AI fallback path (added in Milestone 3, see
// FEEDBACK_CHANGELOG_NLP_PLAN.md) also degrades to this exact message
// on any error, so both call sites stay in sync.
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
	// Interpreter is Milestone 3's natural-language command
	// interpreter (FEEDBACK_CHANGELOG_NLP_PLAN.md) - the final
	// fallback in HandleTextMessage's chain, after every hardcoded
	// shortcut and lexical pattern below. May be nil (e.g. in tests
	// that don't need it), in which case that fallback is skipped
	// entirely and behavior is identical to pre-Milestone-3.
	Interpreter *nlpcommand.Interpreter
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

// HandleTextMessage parses raw player text and routes it contextually using dynamic tokens
func (h *NLPHandler) HandleTextMessage(c telebot.Context) error {
	raw := c.Text()
	text := strings.ToLower(raw)

	// --- 1. CORE MOTHER-KEYBOARD NAVIGATION SHORTCUTS ---
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

	// --- 2. CAMP CONTEXTUAL SUBMENU SHORTCUTS ---
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

	// --- 3. COMBAT CONTEXTUAL SUBMENU SHORTCUTS ---
	if text == "🛰️ scan targets" {
		return h.Combat.HandleRaidBoard(c)
	}
	if text == "🛸 expedition radar" || text == "radar" {
		return h.Combat.HandleExpeditionRadar(c)
	}
	if text == "📻 wasteland radio" {
		return h.World.HandleWorldFeed(c)
	}

	// --- 4. ECONOMY CONTEXTUAL SUBMENU SHORTCUTS ---
	if text == "🪙 financial vault" {
		return h.Econ.HandleFinancialVault(c)
	}
	if text == "🛡️ clan alliances" {
		return h.Clan.HandleClanPanel(c)
	}
	if text == "💱 market exchange" {
		return h.Exchange.HandleExchangePanel(c)
	}

	// --- 5. WORKSHOP CONTEXTUAL SUBMENU SHORTCUTS ---
	if text == "🪖 recruit troops" {
		return h.Factory.HandleRecruitPanel(c)
	}
	if text == "🚗 logistics vehicles" {
		return h.Factory.HandleVehiclesPanel(c)
	}

	// --- 6. GLOBAL CONTROLS ---
	if text == "⬅️ back to hq" {
		return h.Onboarding.HandleStart(c)
	}

	// --- 7. AI COMMAND INTERPRETER (Milestone 3) ---
	// Tried before the broad lexical "contains" heuristics below,
	// since those are fuzzy navigation shortcuts (e.g. any message
	// containing "scout" routing to the Raid Board) that would
	// otherwise swallow a specific, actionable command like "send 5
	// scouts" before the smarter classifier ever saw it. If nothing
	// here is handled (no Interpreter configured, no live AI provider,
	// budget exceeded, or the model itself didn't find a confident
	// match), control falls through to section 8 exactly as before
	// Milestone 3 - this is purely additive.
	if handled, err := h.tryNaturalLanguageCommand(c, raw); handled {
		return err
	}

	// --- 8. LEXICAL INTENT MATCHING PATTERNS ---
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

// tryNaturalLanguageCommand is Milestone 3's entry point
// (FEEDBACK_CHANGELOG_NLP_PLAN.md): it hands the player's raw message
// to the AI command interpreter and, if a confident result came back,
// handles it (executing read-only actions immediately, or showing a
// Confirm/Cancel card for anything that spends a resource or commits
// forces) and reports handled=true. Returns handled=false for every
// case that should fall through to the existing lexical shortcuts
// instead - including "no Interpreter wired", "AI disabled/budget
// exceeded/erroring", and "no live provider configured" (the mock
// provider's placeholder text is intentionally never shown here,
// unlike the dedicated AI Advisor panels elsewhere that do show it -
// a player typing ordinary free text never explicitly asked for an AI
// feature, so silently degrading to the pre-Milestone-3 experience is
// the better default until a real provider key is set).
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

// dispatchParsedCommand routes a matched, allow-listed command to its
// execution: read-only actions run immediately through the exact same
// panel handlers the buttons use; mutating actions show a
// Confirm/Cancel card first (see Action.RequiresConfirmation).
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
	case nlpcommand.ActionDispatchScoutMission:
		return h.confirmDispatchScoutMission(c, cmd)
	default:
		return c.Send(fallbackNotRecognizedMsg)
	}
}

// confirmListMarketItem renders the Confirm/Cancel card for a parsed
// list_market_item command. Nothing here touches the database - the
// actual listing is only posted if the player taps Confirm, via
// HandleNLPConfirmCallback calling the same doPostListing core the
// button-driven Exchange panel uses.
func (h *NLPHandler) confirmListMarketItem(c telebot.Context, cmd nlpcommand.ParsedCommand) error {
	resource := strings.ToLower(strings.TrimSpace(cmd.ArgString("resource")))
	qty := cmd.ArgInt("quantity")
	price := cmd.ArgFloat("price")

	if qty <= 0 || price <= 0 {
		return c.Send("🤖 I couldn't quite catch the listing details - try something like \"list 300k scrap for $500\".")
	}
	if _, ok := marketResourceColumn(resource); !ok {
		return c.Send(fmt.Sprintf("🤖 %s isn't tradeable on the exchange - try Metal, Crystal, or Scrap.", htmlEscape(capitalizeWord(resource))))
	}

	cardText := "🤖 " + htmlBold("CONFIRM LISTING") + "\n" + divider + "\n" +
		fmt.Sprintf("%s List %s %s for %s?\n", resourceEmoji(resource), htmlCode(fmt.Sprintf("%d", qty)), htmlEscape(resource), htmlCode(fmt.Sprintf("$%.0f", price))) +
		divider

	selector := &telebot.ReplyMarkup{}
	btnConfirm := keyboards.Styled(selector.Data("✅ Confirm", "nlp_c", "mkt", resource, fmt.Sprintf("%d", qty), fmt.Sprintf("%.0f", price)), keyboards.StyleSuccess)
	btnCancel := keyboards.Styled(selector.Data("❌ Cancel", "nlp_x", ""), keyboards.StyleDanger)
	return keyboards.SendStyled(c, cardText, [][]keyboards.StyledBtn{{btnCancel, btnConfirm}})
}

// confirmDispatchScoutMission renders the Confirm/Cancel card for a
// parsed dispatch_scout_mission command. See confirmListMarketItem's
// doc comment - same "nothing executes until Confirm" contract.
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

// HandleNLPConfirmCallback fires when a player taps the green Confirm
// button on an AI-parsed command's card. It re-derives the caller's
// encampment server-side (never trusts a campID from callback_data)
// and executes through the exact same core function the button-driven
// UI uses - doPostListing / doDispatchScoutMission - so a
// natural-language "list 300k scrap for $500" enforces identical
// validation to tapping the Exchange panel's own buttons.
func (h *NLPHandler) HandleNLPConfirmCallback(c telebot.Context) error {
	ctx := context.Background()
	sender := c.Sender()
	if sender == nil || len(c.Args()) < 1 {
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid confirmation."})
	}

	switch c.Args()[0] {
	case "mkt":
		if h.Exchange == nil || len(c.Args()) < 4 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid listing."})
		}
		resource := c.Args()[1]
		qty, qtyErr := strconv.Atoi(c.Args()[2])
		price, priceErr := strconv.ParseFloat(c.Args()[3], 64)
		if qtyErr != nil || priceErr != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid listing details."})
		}

		var campID string
		if err := h.Exchange.DB.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", sender.ID).Scan(&campID); err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
		}

		if _, err := h.Exchange.doPostListing(ctx, campID, resource, qty, price); err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Could not list - see the panel for details."})
		}
		_ = c.Respond(&telebot.CallbackResponse{Text: "💱 Listing posted!"})
		return h.Exchange.HandleExchangePanel(c)

	case "sct":
		if h.ScoutMissions == nil || len(c.Args()) < 2 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid scout dispatch."})
		}
		count, err := strconv.Atoi(c.Args()[1])
		if err != nil || count <= 0 {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Invalid scout count."})
		}

		campID, err := h.ScoutMissions.myScoutCamp(ctx, sender.ID)
		if err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "⚠️ Create your outpost camp first using /start"})
		}

		if _, err := h.ScoutMissions.doDispatchScoutMission(ctx, campID, count); err != nil {
			return c.Respond(&telebot.CallbackResponse{Text: "❌ Could not dispatch - see the panel for details."})
		}
		_ = c.Respond(&telebot.CallbackResponse{Text: fmt.Sprintf("🔭 %d Scout Walkers dispatched!", count)})
		return h.ScoutMissions.HandleScoutPanel(c)

	default:
		return c.Respond(&telebot.CallbackResponse{Text: "❌ Unknown confirmation."})
	}
}

// HandleNLPCancelCallback fires when a player taps the red Cancel
// button on an AI-parsed command's card. Nothing was ever committed
// before this point, so cancelling is just a toast acknowledgment.
func (h *NLPHandler) HandleNLPCancelCallback(c telebot.Context) error {
	return c.Respond(&telebot.CallbackResponse{Text: "❌ Cancelled - nothing was changed."})
}
