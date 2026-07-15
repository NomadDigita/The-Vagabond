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

	// OpenAIAPIKey / OpenAIModel configure the OpenAI provider
	// (internal/ai/providers/openaicompat, registered as "openai").
	OpenAIAPIKey string
	OpenAIModel  string

	// DeepSeekAPIKey / DeepSeekModel configure the DeepSeek provider
	// (openaicompat, registered as "deepseek"). Note: as of 2026-07-15
	// the legacy "deepseek-chat" model alias is scheduled for
	// deprecation by DeepSeek on 2026-07-24 — the default below uses
	// "deepseek-v4-flash" instead.
	DeepSeekAPIKey string
	DeepSeekModel  string

	// QwenAPIKey / QwenModel / QwenBaseURL configure the Qwen provider
	// (openaicompat, registered as "qwen") via Alibaba DashScope's
	// OpenAI-compatible endpoint. QwenBaseURL defaults to the
	// international (Singapore) endpoint; mainland China deployments
	// should set it to the dashscope.aliyuncs.com equivalent.
	QwenAPIKey  string
	QwenModel   string
	QwenBaseURL string

	// GrokAPIKey / GrokModel configure the Grok/xAI provider
	// (openaicompat, registered as "grok"). xAI's model lineup and
	// naming have shifted repeatedly through 2026 — verify GrokModel
	// against https://docs.x.ai/docs/models if recommendations seem
	// to be failing with a model-not-found error.
	GrokAPIKey string
	GrokModel  string

	// GeminiAPIKey / GeminiModel configure the Gemini provider
	// (internal/ai/providers/gemini, registered as "gemini").
	GeminiAPIKey string
	GeminiModel  string

	// OllamaBaseURL / OllamaModel configure the Ollama provider
	// (internal/ai/providers/ollama, registered as "ollama") for
	// self-hosted open-weight models. No API key is used. Empty
	// OllamaBaseURL means the provider reports itself unavailable —
	// it is never enabled by default.
	OllamaBaseURL string
	OllamaModel   string

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
//	OPENAI_API_KEY             (string, default "")
//	OPENAI_MODEL               (string, default "gpt-4o-mini")
//	DEEPSEEK_API_KEY           (string, default "")
//	DEEPSEEK_MODEL             (string, default "deepseek-v4-flash")
//	QWEN_API_KEY               (string, default "")
//	QWEN_MODEL                 (string, default "qwen-plus")
//	QWEN_BASE_URL              (string, default DashScope international endpoint)
//	GROK_API_KEY               (string, default "")
//	GROK_MODEL                 (string, default "grok-4-fast")
//	GEMINI_API_KEY             (string, default "")
//	GEMINI_MODEL               (string, default "gemini-2.5-flash")
//	OLLAMA_BASE_URL            (string, default "" — unset means disabled)
//	OLLAMA_MODEL               (string, default "llama3.1")
//	AI_MAX_USER_COST_USD_DAY   (float,  default 0.50)
//	AI_MAX_GLOBAL_COST_USD_DAY (float,  default 25.00)
//	AI_CACHE_TTL_SECONDS       (int,    default 120)
//
// Only ANTHROPIC_API_KEY is required for any real (non-mock) output;
// every other provider activates automatically the moment its own key
// is set, with no other code change needed — set only the ones you
// have.
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
		Enabled:         getenvBool("AI_ENABLED", true),
		DefaultProvider: getenvString("AI_DEFAULT_PROVIDER", "mock"),
		FallbackOrder:   order,

		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel:  getenvString("ANTHROPIC_MODEL", "claude-sonnet-4-6"),

		OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:  getenvString("OPENAI_MODEL", "gpt-4o-mini"),

		DeepSeekAPIKey: os.Getenv("DEEPSEEK_API_KEY"),
		DeepSeekModel:  getenvString("DEEPSEEK_MODEL", "deepseek-v4-flash"),

		QwenAPIKey:  os.Getenv("QWEN_API_KEY"),
		QwenModel:   getenvString("QWEN_MODEL", "qwen-plus"),
		QwenBaseURL: getenvString("QWEN_BASE_URL", "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"),

		GrokAPIKey: os.Getenv("GROK_API_KEY"),
		GrokModel:  getenvString("GROK_MODEL", "grok-4-fast"),

		GeminiAPIKey: os.Getenv("GEMINI_API_KEY"),
		GeminiModel:  getenvString("GEMINI_MODEL", "gemini-2.5-flash"),

		OllamaBaseURL: os.Getenv("OLLAMA_BASE_URL"),
		OllamaModel:   getenvString("OLLAMA_MODEL", "llama3.1"),

		MaxUserCostPerDayUSD:   getenvFloat("AI_MAX_USER_COST_USD_DAY", 0.50),
		MaxGlobalCostPerDayUSD: getenvFloat("AI_MAX_GLOBAL_COST_USD_DAY", 25.00),
		CacheTTLSeconds:        getenvInt("AI_CACHE_TTL_SECONDS", 120),
	}
}
