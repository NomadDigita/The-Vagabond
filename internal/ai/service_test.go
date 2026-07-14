package ai_test

import (
	"context"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

// fakeProvider lets tests control availability/failure without any
// network or database dependency.
type fakeProvider struct {
	name      string
	available bool
	fail      bool
	calls     int
}

func (f *fakeProvider) Name() string    { return f.name }
func (f *fakeProvider) Available() bool { return f.available }
func (f *fakeProvider) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResponse, error) {
	f.calls++
	if f.fail {
		return nil, errTestProviderFailure
	}
	return &ai.CompletionResponse{
		Text:  "ok from " + f.name,
		Model: "test-model",
		Usage: ai.Usage{InputTokens: 10, OutputTokens: 10},
	}, nil
}

var errTestProviderFailure = &testError{"provider failed"}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }

// fakeCost is a zero-dependency in-memory CostTracker.
type fakeCost struct {
	userSpend   map[int64]float64
	globalSpend float64
}

func newFakeCost() *fakeCost { return &fakeCost{userSpend: make(map[int64]float64)} }

func (c *fakeCost) RecordUsage(ctx context.Context, userID int64, feature, provider, model string, usage ai.Usage, costUSD float64) error {
	c.userSpend[userID] += costUSD
	c.globalSpend += costUSD
	return nil
}
func (c *fakeCost) UserSpendToday(ctx context.Context, userID int64) (float64, error) {
	return c.userSpend[userID], nil
}
func (c *fakeCost) GlobalSpendToday(ctx context.Context) (float64, error) {
	return c.globalSpend, nil
}

func newTestService(t *testing.T, providers ...ai.Provider) (*ai.Service, *ai.Registry) {
	t.Helper()
	cfg := &ai.Config{
		Enabled:                true,
		DefaultProvider:        providers[0].Name(),
		FallbackOrder:          providerNames(providers),
		MaxUserCostPerDayUSD:   1000,
		MaxGlobalCostPerDayUSD: 1000,
		CacheTTLSeconds:        60,
	}
	reg := ai.NewRegistry()
	for _, p := range providers {
		reg.Register(p)
	}
	svc := ai.NewService(cfg, reg, newFakeCost(), nil, nil)
	return svc, reg
}

func providerNames(providers []ai.Provider) []string {
	var out []string
	for _, p := range providers {
		out = append(out, p.Name())
	}
	return out
}

func TestComplete_HappyPath(t *testing.T) {
	primary := &fakeProvider{name: "primary", available: true}
	svc, _ := newTestService(t, primary)

	resp, err := svc.Complete(context.Background(), ai.CompletionRequest{
		Feature:  "ai_planet_governor",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Provider != "primary" {
		t.Fatalf("expected provider=primary, got %s", resp.Provider)
	}
	if primary.calls != 1 {
		t.Fatalf("expected 1 call, got %d", primary.calls)
	}
}

func TestComplete_FallsBackWhenPrimaryFails(t *testing.T) {
	primary := &fakeProvider{name: "primary", available: true, fail: true}
	secondary := &fakeProvider{name: "secondary", available: true}
	svc, _ := newTestService(t, primary, secondary)

	resp, err := svc.Complete(context.Background(), ai.CompletionRequest{
		Feature:  "ai_fleet_commander",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "attack?"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Provider != "secondary" {
		t.Fatalf("expected fallback to secondary, got %s", resp.Provider)
	}
}

func TestComplete_SkipsUnavailableProvider(t *testing.T) {
	primary := &fakeProvider{name: "primary", available: false}
	secondary := &fakeProvider{name: "secondary", available: true}
	svc, _ := newTestService(t, primary, secondary)

	resp, err := svc.Complete(context.Background(), ai.CompletionRequest{
		Feature:  "ai_economy_advisor",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "roi?"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Provider != "secondary" {
		t.Fatalf("expected secondary (primary unavailable), got %s", resp.Provider)
	}
	if primary.calls != 0 {
		t.Fatalf("unavailable provider should never be called, got %d calls", primary.calls)
	}
}

func TestComplete_ReturnsCachedResponseOnSecondCall(t *testing.T) {
	primary := &fakeProvider{name: "primary", available: true}
	svc, _ := newTestService(t, primary)

	req := ai.CompletionRequest{
		Feature:  "ai_research_planner",
		UserID:   42,
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "plan my research"}},
	}

	if _, err := svc.Complete(context.Background(), req); err != nil {
		t.Fatalf("unexpected error on first call: %v", err)
	}
	resp2, err := svc.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error on second call: %v", err)
	}
	if !resp2.Cached {
		t.Fatalf("expected second identical call to be served from cache")
	}
	if primary.calls != 1 {
		t.Fatalf("expected provider to be called exactly once (cache hit on 2nd), got %d", primary.calls)
	}
}

func TestComplete_ErrorsWhenAIDisabled(t *testing.T) {
	primary := &fakeProvider{name: "primary", available: true}
	svc, _ := newTestService(t, primary)
	svc.Config.Enabled = false

	_, err := svc.Complete(context.Background(), ai.CompletionRequest{
		Feature:  "ai_battle_analyst",
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "why did I lose?"}},
	})
	if err != ai.ErrAIDisabled {
		t.Fatalf("expected ErrAIDisabled, got %v", err)
	}
}

func TestComplete_RequiresFeature(t *testing.T) {
	primary := &fakeProvider{name: "primary", available: true}
	svc, _ := newTestService(t, primary)

	_, err := svc.Complete(context.Background(), ai.CompletionRequest{
		Messages: []ai.Message{{Role: ai.RoleUser, Content: "no feature set"}},
	})
	if err == nil {
		t.Fatalf("expected error when Feature is empty")
	}
}
