package ai

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrBudgetExceeded = errors.New("ai: daily cost budget exceeded")

var pricePerMillionTokens = map[string]struct{ In, Out float64 }{
	"anthropic:claude-sonnet-4-6": {In: 3.00, Out: 15.00},

	"openai:gpt-5.6-luna": {In: 1.00, Out: 6.00},
	"openai:gpt-4o-mini":  {In: 0.15, Out: 0.60},
	"openai:gpt-4o":       {In: 2.50, Out: 10.00},

	"deepseek:deepseek-v4-flash": {In: 0.14, Out: 0.28},

	"qwen:qwen-plus": {In: 0.40, Out: 1.20},

	"grok:grok-4-fast": {In: 0.20, Out: 0.50},

	"gemini:gemini-3.5-flash": {In: 1.50, Out: 9.00},

	"ollama:*": {In: 0, Out: 0},

	"mock:mock-1": {In: 0, Out: 0},
	"default":     {In: 5.00, Out: 15.00},
}

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

type CostTracker interface {
	RecordUsage(ctx context.Context, userID int64, feature, provider, model string, usage Usage, costUSD float64) error
	UserSpendToday(ctx context.Context, userID int64) (float64, error)
	GlobalSpendToday(ctx context.Context) (float64, error)
}

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

func nullableUserID(userID int64) any {
	if userID == 0 {
		return nil
	}
	return userID
}
