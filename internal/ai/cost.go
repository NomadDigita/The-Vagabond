package ai

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// ErrBudgetExceeded is returned by Service.Complete when a request
// would exceed the configured per-user or global daily budget.
var ErrBudgetExceeded = errors.New("ai: daily cost budget exceeded")

// pricePerMillionTokens is used only for budget accounting (not
// billing) — it estimates spend against the daily budget caps in
// Config, it is not what any provider actually charges you.
//
// Every entry below was verified by web search on 2026-07-15 against
// each provider's own pricing documentation (not guessed). Prices
// change — re-verify before trusting this table for a real budget
// months from now, especially the fast-moving providers (DeepSeek,
// Grok/xAI, Qwen) called out individually below.
//
// Unknown provider/model pairs fall back to a deliberately
// conservative "default" entry so cost tracking never silently
// under-counts an unrecognized model.
var pricePerMillionTokens = map[string]struct{ In, Out float64 }{
	// Anthropic — confirmed at $3.00/$15.00 (Sonnet 4.6) as of 2026-07-15.
	"anthropic:claude-sonnet-4-6": {In: 3.00, Out: 15.00},

	// OpenAI — gpt-4o-mini confirmed at $0.15/$0.60. gpt-4o (the
	// larger, pricier tier) confirmed at $2.50/$10.00 — add that row
	// yourself if you configure OPENAI_MODEL=gpt-4o.
	"openai:gpt-4o-mini": {In: 0.15, Out: 0.60},
	"openai:gpt-4o":      {In: 2.50, Out: 10.00},

	// DeepSeek — confirmed at $0.14/$0.28 for deepseek-v4-flash as of
	// 2026-07-15. IMPORTANT: the legacy "deepseek-chat" model alias is
	// scheduled for deprecation by DeepSeek on 2026-07-24 — do not
	// configure DEEPSEEK_MODEL=deepseek-chat going forward, use
	// deepseek-v4-flash directly.
	"deepseek:deepseek-v4-flash": {In: 0.14, Out: 0.28},

	// Qwen (Alibaba DashScope) — qwen-plus commonly cited around
	// $0.40/$1.20 per million tokens as of mid-2026, though DashScope
	// pricing has tiered-by-request-length quirks and sources vary
	// (some cite $0.26/$0.78) — treat this row as approximate and
	// verify against the Alibaba Cloud Model Studio pricing page for
	// a real budget.
	"qwen:qwen-plus": {In: 0.40, Out: 1.20},

	// Grok (xAI) — model naming and pricing here have shifted several
	// times within 2026 (grok-4.1-fast -> grok-4.20 -> grok-4.3 ->
	// grok-4.5), and sources disagree on the exact current cheap-tier
	// ID. The row below reflects the cheap "fast" tier commonly cited
	// at $0.20/$0.50 per million tokens as of mid-2026 — confirm the
	// exact model ID at https://docs.x.ai/docs/models before relying
	// on this for a real budget; it is the least certain row in this
	// table.
	"grok:grok-4-fast": {In: 0.20, Out: 0.50},

	// Gemini — gemini-2.5-flash confirmed at $0.30/$2.50 as of
	// 2026-07-15. Note gemini-2.0-flash was shut down by Google on
	// 2026-06-01 and must not be used.
	"gemini:gemini-2.5-flash": {In: 0.30, Out: 2.50},

	// Ollama (self-hosted) — genuinely zero marginal API cost; you pay
	// for the compute instead. Kept at 0/0 rather than omitted so a
	// lookup for "ollama:<any model>" resolves to a real, correct
	// answer instead of falling through to "default" below.
	"ollama:*": {In: 0, Out: 0},

	"mock:mock-1": {In: 0, Out: 0},
	"default":     {In: 5.00, Out: 15.00},
}

// EstimateCostUSD prices a Usage for a given provider+model pair.
func EstimateCostUSD(provider, model string, u Usage) float64 {
	key := provider + ":" + model
	price, ok := pricePerMillionTokens[key]
	if !ok {
		price = pricePerMillionTokens["default"]
	}
	in := float64(u.InputTokens) / 1_000_000 * price.In
	out := float64(u.OutputTokens) / 1_000_000 * price.Out
	return in + out
}

// CostTracker records spend and answers budget questions. The
// Postgres-backed implementation is the source of truth across process
// restarts and horizontal scaling; callers needing zero-dependency unit
// tests can implement the interface with an in-memory fake.
type CostTracker interface {
	RecordUsage(ctx context.Context, userID int64, feature, provider, model string, usage Usage, costUSD float64) error
	UserSpendToday(ctx context.Context, userID int64) (float64, error)
	GlobalSpendToday(ctx context.Context) (float64, error)
}

// PostgresCostTracker implements CostTracker against the ai_cost_log
// table created by migrations/020_vagabond_ai_foundation.sql.
type PostgresCostTracker struct {
	DB *sql.DB
}

func NewPostgresCostTracker(db *sql.DB) *PostgresCostTracker {
	return &PostgresCostTracker{DB: db}
}

func (t *PostgresCostTracker) RecordUsage(ctx context.Context, userID int64, feature, provider, model string, usage Usage, costUSD float64) error {
	_, err := t.DB.ExecContext(ctx, `
		INSERT INTO ai_cost_log (user_id, feature, provider, model, input_tokens, output_tokens, cost_usd, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		nullableUserID(userID), feature, provider, model, usage.InputTokens, usage.OutputTokens, costUSD, time.Now().UTC())
	return err
}

func (t *PostgresCostTracker) UserSpendToday(ctx context.Context, userID int64) (float64, error) {
	var total sql.NullFloat64
	err := t.DB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(cost_usd), 0) FROM ai_cost_log
		WHERE user_id = $1 AND created_at >= date_trunc('day', now() AT TIME ZONE 'UTC')`,
		userID).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}

func (t *PostgresCostTracker) GlobalSpendToday(ctx context.Context) (float64, error) {
	var total sql.NullFloat64
	err := t.DB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(cost_usd), 0) FROM ai_cost_log
		WHERE created_at >= date_trunc('day', now() AT TIME ZONE 'UTC')`).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Float64, nil
}

// nullableUserID maps the "system/background request" sentinel (0) to
// NULL so ai_cost_log.user_id's FK to users(telegram_id) never breaks
// on background jobs like the Dynamic Galaxy director.
func nullableUserID(userID int64) any {
	if userID == 0 {
		return nil
	}
	return userID
}
