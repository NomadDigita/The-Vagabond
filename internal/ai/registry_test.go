package ai_test

import (
	"context"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

type fakeNamedProvider struct {
	name      string
	available bool
}

func (f *fakeNamedProvider) Name() string    { return f.name }
func (f *fakeNamedProvider) Available() bool { return f.available }
func (f *fakeNamedProvider) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResponse, error) {
	return &ai.CompletionResponse{Text: "ok from " + f.name}, nil
}

func TestRegistry_MockIsAlwaysLast(t *testing.T) {
	reg := ai.NewRegistry()
	mock := &fakeNamedProvider{name: "mock", available: true}
	qwen := &fakeNamedProvider{name: "qwen", available: true}
	gemini := &fakeNamedProvider{name: "gemini", available: true}

	reg.Register(mock)
	reg.Register(qwen)
	reg.Register(gemini)
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

func TestRegistry_UnlistedProviderStillReachable(t *testing.T) {
	reg := ai.NewRegistry()
	primary := &fakeNamedProvider{name: "anthropic", available: true}
	unlisted := &fakeNamedProvider{name: "openai", available: true}
	mock := &fakeNamedProvider{name: "mock", available: true}
	reg.Register(primary)
	reg.Register(unlisted)
	reg.Register(mock)
	reg.SetOrder([]string{"anthropic"})

	ordered := reg.Ordered()
	names := providerNames(ordered)
	if len(ordered) != 3 {
		t.Fatalf("expected all 3 available providers reachable, got: %v", names)
	}
	if names[len(names)-1] != "mock" {
		t.Fatalf("expected mock last even when an unlisted provider is also present, got: %v", names)
	}
}
