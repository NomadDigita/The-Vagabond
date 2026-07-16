package ai_test

import (
	"context"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

// fakeNamedProvider is a minimal Provider for registry-ordering tests
// (fakeProvider in service_test.go also exists, but that one is
// scoped to Service-level fallback behavior; this one is deliberately
// separate and simpler, scoped only to Registry.Ordered()).
type fakeNamedProvider struct {
	name      string
	available bool
}

func (f *fakeNamedProvider) Name() string    { return f.name }
func (f *fakeNamedProvider) Available() bool { return f.available }
func (f *fakeNamedProvider) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResponse, error) {
	return &ai.CompletionResponse{Text: "ok from " + f.name}, nil
}

// TestRegistry_MockIsAlwaysLast is a direct regression test for a
// confirmed real bug (2026-07-16): Config's defaults
// (AI_DEFAULT_PROVIDER="mock", AI_FALLBACK_PROVIDERS="mock") placed
// "mock" first in the configured order. Because mock is always
// Available() and its Complete never errors, Service.Complete's
// fallback loop returned immediately after mock every time — real,
// available providers (confirmed live: Qwen and Gemini, both with
// valid API keys configured) were never reached at all, despite
// /ai_status correctly reporting them as available.
//
// This test constructs the exact failure condition (mock listed
// first in order, alongside other available providers) and asserts
// mock is NOT first in the result — it must be moved to the end
// regardless of its position in the configured order.
func TestRegistry_MockIsAlwaysLast(t *testing.T) {
	reg := ai.NewRegistry()
	mock := &fakeNamedProvider{name: "mock", available: true}
	qwen := &fakeNamedProvider{name: "qwen", available: true}
	gemini := &fakeNamedProvider{name: "gemini", available: true}

	reg.Register(mock)
	reg.Register(qwen)
	reg.Register(gemini)
	// Reproduce the exact real-world default: mock listed first.
	reg.SetOrder([]string{"mock", "mock"})

	ordered := reg.Ordered()
	if len(ordered) != 3 {
		t.Fatalf("expected all 3 available providers, got %d: %+v", len(ordered), ordered)
	}
	if ordered[len(ordered)-1].Name() != "mock" {
		t.Fatalf("expected mock to be last, got order: %v", providerNames(ordered))
	}
	for _, p := range ordered[:len(ordered)-1] {
		if p.Name() == "mock" {
			t.Fatalf("mock must appear exactly once, at the end — got it earlier too: %v", providerNames(ordered))
		}
	}
}

// TestRegistry_MockIsSkippedIfUnavailable confirms mock's special
// last-place handling doesn't accidentally force it into the result
// when it isn't registered or isn't available.
func TestRegistry_MockIsSkippedIfUnavailable(t *testing.T) {
	reg := ai.NewRegistry()
	qwen := &fakeNamedProvider{name: "qwen", available: true}
	reg.Register(qwen)
	reg.SetOrder([]string{"mock", "qwen"})

	ordered := reg.Ordered()
	if len(ordered) != 1 || ordered[0].Name() != "qwen" {
		t.Fatalf("expected only qwen (mock never registered), got: %v", providerNames(ordered))
	}
}

// TestRegistry_UnlistedProviderStillReachable confirms a provider
// registered but not mentioned in SetOrder is still returned (just
// after the explicitly-ordered ones and before mock).
func TestRegistry_UnlistedProviderStillReachable(t *testing.T) {
	reg := ai.NewRegistry()
	primary := &fakeNamedProvider{name: "anthropic", available: true}
	unlisted := &fakeNamedProvider{name: "openai", available: true}
	mock := &fakeNamedProvider{name: "mock", available: true}
	reg.Register(primary)
	reg.Register(unlisted)
	reg.Register(mock)
	reg.SetOrder([]string{"anthropic"}) // "openai" deliberately not mentioned

	ordered := reg.Ordered()
	names := providerNames(ordered)
	if len(ordered) != 3 {
		t.Fatalf("expected all 3 available providers reachable, got: %v", names)
	}
	if names[len(names)-1] != "mock" {
		t.Fatalf("expected mock last even when an unlisted provider is also present, got: %v", names)
	}
}
