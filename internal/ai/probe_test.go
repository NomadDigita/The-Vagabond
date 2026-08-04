package ai_test

import (
	"context"
	"testing"
)

// TestProbeAllProviders_ReportsRealErrorText is the core regression
// this file exists to guard: a provider that IS configured
// (Available()==true) but fails on the actual call must surface its
// exact error text through ProbeResult, not just "unavailable" — this
// is precisely the distinction /ai_status's old Providers list could
// never make, which is why an account-side quota/access error looked
// identical to a code bug across several past investigation sessions.
func TestProbeAllProviders_ReportsRealErrorText(t *testing.T) {
	broken := &fakeProvider{name: "gemini", available: true, fail: true}
	svc, _ := newTestService(t, broken)

	results := svc.ProbeAllProviders(context.Background())
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d: %+v", len(results), results)
	}
	r := results[0]
	if r.OK {
		t.Fatalf("expected probe to report failure for a broken provider")
	}
	if r.Err != errTestProviderFailure.Error() {
		t.Fatalf("expected the provider's exact error text, got %q", r.Err)
	}
}

// TestProbeAllProviders_ReportsSuccess confirms a genuinely working
// provider is reported OK with its model name.
func TestProbeAllProviders_ReportsSuccess(t *testing.T) {
	working := &fakeProvider{name: "openai", available: true}
	svc, _ := newTestService(t, working)

	results := svc.ProbeAllProviders(context.Background())
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("expected 1 successful result, got %+v", results)
	}
	if working.calls != 1 {
		t.Fatalf("expected exactly 1 real call to the provider, got %d", working.calls)
	}
}

// TestProbeAllProviders_UnavailableProviderNotCalled confirms a
// provider with no key configured is reported as "not configured"
// without ever invoking Complete (which would likely panic/error on
// most real provider implementations given no credentials).
func TestProbeAllProviders_UnavailableProviderNotCalled(t *testing.T) {
	noKey := &fakeProvider{name: "grok", available: false}
	svc, _ := newTestService(t, noKey)

	results := svc.ProbeAllProviders(context.Background())
	if len(results) != 1 || results[0].OK {
		t.Fatalf("expected 1 failed result, got %+v", results)
	}
	if noKey.calls != 0 {
		t.Fatalf("unavailable provider must never be called, got %d calls", noKey.calls)
	}
}

// TestProbeAllProviders_SkipsMock confirms the mock provider (never a
// real integration to diagnose) is excluded from probe results even
// when registered alongside real providers.
func TestProbeAllProviders_SkipsMock(t *testing.T) {
	mock := &fakeProvider{name: "mock", available: true}
	real := &fakeProvider{name: "anthropic", available: true}
	svc, _ := newTestService(t, mock, real)

	results := svc.ProbeAllProviders(context.Background())
	if len(results) != 1 || results[0].Provider != "anthropic" {
		t.Fatalf("expected only the non-mock provider to be probed, got %+v", results)
	}
}

// TestProbeAllProviders_RecordsCostOnSuccess confirms a successful
// probe still records its (tiny) cost against the global budget, so
// repeated /ai_probe runs can't bypass daily spend tracking.
func TestProbeAllProviders_RecordsCostOnSuccess(t *testing.T) {
	working := &fakeProvider{name: "deepseek", available: true}
	svc, _ := newTestService(t, working)

	svc.ProbeAllProviders(context.Background())

	global, err := svc.Cost.GlobalSpendToday(context.Background())
	if err != nil {
		t.Fatalf("unexpected error reading global spend: %v", err)
	}
	if global <= 0 {
		t.Fatalf("expected a successful probe to record nonzero cost, got %v", global)
	}
}

// TestProbeAllProviders_SkipsWhenGlobalBudgetExhausted confirms the
// probe respects the same daily budget Service.Complete does, rather
// than being a way to spend past the cap.
func TestProbeAllProviders_SkipsWhenGlobalBudgetExhausted(t *testing.T) {
	working := &fakeProvider{name: "openai", available: true}
	svc, _ := newTestService(t, working)
	svc.Config.MaxGlobalCostPerDayUSD = 0.0001
	fc := svc.Cost.(*fakeCost)
	fc.globalSpend = 1000 // already over budget

	results := svc.ProbeAllProviders(context.Background())
	if len(results) != 1 || results[0].OK {
		t.Fatalf("expected probe to be skipped once global budget is exhausted, got %+v", results)
	}
	if working.calls != 0 {
		t.Fatalf("expected no real call once budget is exhausted, got %d calls", working.calls)
	}
}
