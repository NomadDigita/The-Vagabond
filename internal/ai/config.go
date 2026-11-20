package ai

import (
	"os"
	"strconv"
	"strings"
)

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

	FeatureCommandInterpreter Feature = "ai_command_interpreter"
)

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
		FeatureCommandInterpreter,
	}
}

type Config struct {
	Enabled bool

	DefaultProvider string
	FallbackOrder   []string

	AnthropicAPIKey string
	AnthropicModel  string

	OpenAIAPIKey string
	OpenAIModel  string

	DeepSeekAPIKey string
	DeepSeekModel  string

	QwenAPIKey  string
	QwenModel   string
	QwenBaseURL string

	GrokAPIKey string
	GrokModel  string

	GeminiAPIKey         string
	GeminiModel          string
	GeminiModelFallbacks []string

	OllamaBaseURL string
	OllamaModel   string

	MaxUserCostPerDayUSD   float64
	MaxGlobalCostPerDayUSD float64

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

func LoadConfig() *Config {
	fallback := getenvString("AI_FALLBACK_PROVIDERS", "mock")
	var order []string
	for _, p := range strings.Split(fallback, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			order = append(order, p)
		}
	}

	geminiModelFallbacksRaw := getenvString("GEMINI_MODEL_FALLBACKS", "gemini-3.5-flash-lite,gemini-3.6-flash,gemini-3.1-flash-lite")
	var geminiModelFallbacks []string
	if !strings.EqualFold(strings.TrimSpace(geminiModelFallbacksRaw), "none") {
		for _, m := range strings.Split(geminiModelFallbacksRaw, ",") {
			m = strings.TrimSpace(m)
			if m != "" {
				geminiModelFallbacks = append(geminiModelFallbacks, m)
			}
		}
	}

	return &Config{
		Enabled:         getenvBool("AI_ENABLED", true),
		DefaultProvider: getenvString("AI_DEFAULT_PROVIDER", "mock"),
		FallbackOrder:   order,

		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		AnthropicModel:  getenvString("ANTHROPIC_MODEL", "claude-sonnet-4-6"),

		OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
		OpenAIModel:  getenvString("OPENAI_MODEL", "gpt-5.6-luna"),

		DeepSeekAPIKey: os.Getenv("DEEPSEEK_API_KEY"),
		DeepSeekModel:  getenvString("DEEPSEEK_MODEL", "deepseek-v4-flash"),

		QwenAPIKey:  os.Getenv("QWEN_API_KEY"),
		QwenModel:   getenvString("QWEN_MODEL", "qwen-plus"),
		QwenBaseURL: getenvString("QWEN_BASE_URL", "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"),

		GrokAPIKey: os.Getenv("GROK_API_KEY"),
		GrokModel:  getenvString("GROK_MODEL", "grok-4-fast"),

		GeminiAPIKey:         os.Getenv("GEMINI_API_KEY"),
		GeminiModel:          getenvString("GEMINI_MODEL", "gemini-3.5-flash"),
		GeminiModelFallbacks: geminiModelFallbacks,

		OllamaBaseURL: os.Getenv("OLLAMA_BASE_URL"),
		OllamaModel:   getenvString("OLLAMA_MODEL", "llama3.1"),

		MaxUserCostPerDayUSD:   getenvFloat("AI_MAX_USER_COST_USD_DAY", 0.50),
		MaxGlobalCostPerDayUSD: getenvFloat("AI_MAX_GLOBAL_COST_USD_DAY", 25.00),
		CacheTTLSeconds:        getenvInt("AI_CACHE_TTL_SECONDS", 120),
	}
}
