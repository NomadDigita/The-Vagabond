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

// pricePerMillionTokens is a conservative, intentionally-static price
// table used only for budget accounting (not billing). Update as
// providers change published pricing. Unknown provider/model pairs
// fall back to a safe default so cost tracking never silently under-
// counts an unrecognized model.
var pricePerMillionTokens = map[string]struct{ In, Out float64 }{
	"anthropic:claude-sonnet-4-6": {In: 3.00, Out: 15.00},
	"mock:mock-1":                 {In: 0, Out: 0},
	"default":                     {In: 5.00, Out: 15.00},
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
