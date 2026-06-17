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
}

func NewNLPHandler(onb *OnboardingHandler, camp *CampHandler, comb *CombatHandler, econ *EconomyHandler, clan *ClanHandler) *NLPHandler {
	return &NLPHandler{
		Onboarding: onb,
		Camp:       camp,
		Combat:     comb,
		Econ:       econ,
		Clan:       clan,
	}
}

// HandleTextMessage parses raw player text and routes it contextually
func (h *NLPHandler) HandleTextMessage(c telebot.Context) error {
	text := strings.ToLower(c.Text())

	// Exact matches for bottom menu routing blocks
	if text == "📡 terminal hq" {
		return h.Onboarding.HandleStart(c)
	}
	if text == "⛺ outpost camp" {
		return h.Camp.HandleCamp(c)
	}
	if text == "⚔️ tactical combat" {
		return h.Combat.HandleRaidBoard(c)
	}
	if text == "🏦 system economy" {
		return h.Econ.HandleEconPanel(c)
	}

	// Lexical intents matching
	if strings.Contains(text, "upgrade") || strings.Contains(text, "build") {
		return h.Camp.HandleStructuralUpgrades(c)
	}
	if strings.Contains(text, "warehouse") || strings.Contains(text, "stock") || strings.Contains(text, "resources") {
		return h.Econ.HandleEconPanel(c)
	}
	if strings.Contains(text, "scout") || strings.Contains(text, "find") {
		return h.Combat.HandleRaidBoard(c)
	}
	if strings.Contains(text, "alliance") || strings.Contains(text, "clan") {
		return h.Clan.HandleClanPanel(c)
	}

	// Default fallback response
	return c.Send("🤖 SECURE SHELL: Intent not recognized. Please utilize the persistent interface options below.")
}
