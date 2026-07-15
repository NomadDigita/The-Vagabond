package ai

import (
	"os"
	"strconv"
	"strings"
)

// Feature is a stable identifier for an AI-driven subsystem. Every
// Phase B-J capability registers under one of these so permissions,
// cost attribution, and feature flags all key off the same string.
type Feature string

const (
	FeaturePlanetGovernor Feature = "ai_planet_governor"
	FeatureFleetCommander Feature = "ai_fleet_commander"
	FeatureEconomyAdvisor Feature = "ai_economy_advisor"
	FeatureResearchPlan   Feature = "ai_research_planner"
	FeatureBattleAnalyst  Feature = "ai_battle_analyst"
	FeatureGuildAssistant Feature = "ai_guild_assistant"
	FeatureDynamicGalaxy  Feature = "ai_dynamic_galaxy"
	FeatureNPCIntel       Feature = "ai_npc_intelligence"
	FeatureDevConsole     Feature = "ai_developer_console"
)

// AllFeatures lists every known feature flag, used to seed defaults.
func AllFeatures() []Feature {
	return []Feature{
		FeaturePlanetGovernor,
		FeatureFleetCommander,
		FeatureEconomyAdvisor,
		FeatureResearchPlan,
		FeatureBattleAnalyst,
		FeatureGuildAssistant,
		FeatureDynamicGalaxy,
		FeatureNPCIntel,
		FeatureDevConsole,
	}
}

// Config holds process-wide AI settings, loaded from environment
// variables at startup. Per-feature and per-user overrides live in the
// database (see ai_feature_flags / ai_permissions in
// migrations/020_vagabond_ai_foundation.sql) and are consulted by
// PermissionManager on top of this static config.
type Config struct {
	// Enabled is a master kill switch. When false, Service.Complete
	// returns ErrAIDisabled immediately for every request.
	Enabled bool

	// DefaultProvider is tried first for every request.
	DefaultProvider string
	// FallbackOrder lists provider names to try, in order, if the
	// default (or a preceding fallback) errors or is unavailable.
	FallbackOrder []string

	// AnthropicAPIKey / AnthropicModel configure the built-in
	// Anthropic provider. Empty key means the provider reports
	// itself Available() == false and is skipped.
	AnthropicAPIKey string
	AnthropicModel  string

	// MaxUserCostPerDayUSD caps spend attributable to a single
	// Telegram user across all features, per UTC day. Zero disables
	// the per-user cap (not recommended in production).
	MaxUserCostPerDayUSD float64
	// MaxGlobalCostPerDayUSD caps total AI spend across all users and
	// background jobs, per UTC day.
	MaxGlobalCostPerDayUSD float64

	// CacheTTLSeconds controls how long identical requests (same
	// feature + user + prompt hash) are served from cache instead of
	// hitting a provider. Zero disables caching.
	CacheTTLSeconds int
}

func getenvBool(key string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func getenvFloat(key string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func getenvInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	i, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return i
}

func getenvString(key, def string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return v
}

// LoadConfig reads AI configuration from the environment. Every
// variable is optional; a functioning (mock-only, zero-cost) config is
// produced even with no environment set at all, so the AI foundation
// never blocks a boot of the game server.
//
// Recognized variables:
//
//	AI_ENABLED                 (bool,   default true)
//	AI_DEFAULT_PROVIDER        (string, default "mock")
//	AI_FALLBACK_PROVIDERS      (comma-separated, default "mock")
//	ANTHROPIC_API_KEY          (string, default "")
//	ANTHROPIC_MODEL            (string, default "claude-sonnet-4-6")
//	AI_MAX_USER_COST_USD_DAY   (float,  default 0.50)
//	AI_MAX_GLOBAL_COST_USD_DAY (float,  default 25.00)
//	AI_CACHE_TTL_SECONDS       (int,    default 120)
func LoadConfig() *Config {
	fallback := getenvString("AI_FALLBACK_PROVIDERS", "mock")
	var order []string
	for _, p := range strings.Split(fallback, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			order = append(order, p)
		}
	}

	return &Config{
		Enabled:                getenvBool("AI_ENABLED", true),
		DefaultProvider:        getenvString("AI_DEFAULT_PROVIDER", "mock"),
		FallbackOrder:          order,
		AnthropicAPIKey:        os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel:         getenvString("ANTHROPIC_MODEL", "claude-sonnet-4-6"),
		MaxUserCostPerDayUSD:   getenvFloat("AI_MAX_USER_COST_USD_DAY", 0.50),
		MaxGlobalCostPerDayUSD: getenvFloat("AI_MAX_GLOBAL_COST_USD_DAY", 25.00),
		CacheTTLSeconds:        getenvInt("AI_CACHE_TTL_SECONDS", 120),
	}
}
