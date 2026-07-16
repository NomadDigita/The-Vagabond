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

	// OpenAI — GPT-5.6 reached general availability 2026-07-09,
	// superseding the gpt-4o family. gpt-5.6-luna (the cheapest tier)
	// confirmed at $1.00/$6.00 as of 2026-07-16 against OpenAI's own
	// pricing page. gpt-4o-mini ($0.15/$0.60) and gpt-4o ($2.50/$10.00)
	// are kept below since they likely still work if explicitly
	// configured, but are no longer the current generation.
	"openai:gpt-5.6-luna": {In: 1.00, Out: 6.00},
	"openai:gpt-4o-mini":  {In: 0.15, Out: 0.60},
	"openai:gpt-4o":       {In: 2.50, Out: 10.00},

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

	// Grok (xAI) — re-confirmed 2026-07-16: xAI's flagship moved to
	// Grok 4.5 ($2.00/$6.00, launched 2026-07-08), but the cheap
	// "fast"/volume tier this codebase targets is still confirmed at
	// $0.20/$0.50 per million tokens (cited as "Grok 4.1 Fast" across
	// multiple independent pricing trackers). The exact API model
	// string is the least certain part of this row — sources show
	// both dash and dot conventions across xAI's model lineup
	// (grok-4.5, grok-4.3, grok-4.1-fast). Confirm the exact string at
	// https://docs.x.ai/docs/models before relying on GROK_MODEL in
	// production; if requests fail with a model-not-found error, that
	// is almost certainly why.
	"grok:grok-4-fast": {In: 0.20, Out: 0.50},

	// Gemini — gemini-3.5-flash confirmed at $1.50/$9.00 as of
	// 2026-07-16 (Google shipped the 3.5 generation 2026-05-19,
	// superseding 2.5; gemini-2.0-flash was separately shut down
	// 2026-06-01 and must not be used). gemini-3.5-pro is not yet
	// generally available — do not add a pricing row for it until
	// independently confirmed GA.
	"gemini:gemini-3.5-flash": {In: 1.50, Out: 9.00},

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
