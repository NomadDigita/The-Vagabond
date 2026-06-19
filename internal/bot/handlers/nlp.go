package handlers

import (
	"strings"

	"gopkg.in/telebot.v3"
)

type NLPHandler struct {
	Onboarding *OnboardingHandler
	Camp       *CampHandler
	Combat     *CombatHandler
	Econ       *EconomyHandler
	Clan       *ClanHandler
	Hero       *HeroHandler
	Agent      *AgentHandler
	Factory    *FactoryHandler
	Silo       *SiloHandler
	Research   *ResearchHandler
	Exchange   *ExchangeHandler
	World      *WorldHandler
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
) *NLPHandler {
	return &NLPHandler{
		Onboarding: onb,
		Camp:       camp,
		Combat:     comb,
		Econ:       econ,
		Clan:       clan,
		Hero:       hero,
		Agent:      agent,
		Factory:    factory,
		Silo:       silo,
		Research:   research,
		Exchange:   exchange,
		World:      world,
	}
}

// HandleTextMessage parses raw player text and routes it contextually using dynamic tokens
func (h *NLPHandler) HandleTextMessage(c telebot.Context) error {
	text := strings.ToLower(c.Text())

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

	// --- 7. LEXICAL INTENT MATCHING PATTERNS ---
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

	return c.Send("🤖 SECURE SHELL: Intent not recognized. Please utilize the persistent interface options below.")
}