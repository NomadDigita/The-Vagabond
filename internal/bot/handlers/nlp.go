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
	}
}

// HandleTextMessage parses raw player text and routes it contextually using dynamic tokens
func (h *NLPHandler) HandleTextMessage(c telebot.Context) error {
	text := strings.ToLower(c.Text())

	// Exact mother-route commands checks
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

	// Lexical sub-menu shortcuts checks
	if text == "🔨 structural upgrades" {
		return h.Camp.HandleStructuralUpgrades(c)
	}
	if text == "👥 hero commander" {
		return h.Hero.HandleHeroPanel(c) // Restored correct redirect
	}
	if text == "🧠 automation agent" {
		return h.Agent.HandleAgent(c) // Restored correct redirect
	}
	if text == "🧬 mutation core" {
		return h.Camp.HandleMutationsPanel(c)
	}
	if text == "⛏️ active mining" {
		return h.Camp.HandleActiveMining(c)
	}
	if text == "🪙 financial vault" {
		return h.Econ.HandleFinancialVault(c)
	}
	if text == "🛡️ clan alliances" {
		return h.Clan.HandleClanPanel(c)
	}
	if text == "💱 market exchange" {
		return h.Exchange.HandleExchangePanel(c) // Restored correct redirect
	}
	if text == "🧪 research lab" {
		return h.Research.HandleResearchPanel(c) // Restored correct redirect
	}

	// Lexical intents token matching
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