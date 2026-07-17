package mock

import (
	"context"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
	"github.com/NomadDigita/The-Vagabond/internal/game/econadvisor"
	"github.com/NomadDigita/The-Vagabond/internal/game/fleetcommander"
	"github.com/NomadDigita/The-Vagabond/internal/game/governor"
	"github.com/NomadDigita/The-Vagabond/internal/game/researchplanner"
)

func TestMockPlaceholder_ParsesForGovernor(t *testing.T) {
	p := New()
	resp, err := p.Complete(context.Background(), ai.CompletionRequest{Feature: "ai_planet_governor", JSONMode: true})
	if err != nil {
		t.Fatal(err)
	}
	rec := governor.ParseRecommendation(resp.Text)
	if rec.FellBackToRawText {
		t.Fatalf("expected mock governor placeholder to parse as valid JSON, got fallback. Raw: %s", resp.Text)
	}
}

func TestMockPlaceholder_ParsesForFleetCommander(t *testing.T) {
	p := New()
	resp, err := p.Complete(context.Background(), ai.CompletionRequest{Feature: "ai_fleet_commander", JSONMode: true})
	if err != nil {
		t.Fatal(err)
	}
	rec := fleetcommander.ParseRecommendation(resp.Text)
	if rec.FellBackToRawText {
		t.Fatalf("expected mock fleet commander placeholder to parse as valid JSON, got fallback. Raw: %s", resp.Text)
	}
}

func TestMockPlaceholder_ParsesForEconAdvisor(t *testing.T) {
	p := New()
	resp, err := p.Complete(context.Background(), ai.CompletionRequest{Feature: "ai_economy_advisor", JSONMode: true})
	if err != nil {
		t.Fatal(err)
	}
	rec := econadvisor.ParseRecommendation(resp.Text)
	if rec.FellBackToRawText {
		t.Fatalf("expected mock econ advisor placeholder to parse as valid JSON, got fallback. Raw: %s", resp.Text)
	}
}

func TestMockPlaceholder_ParsesForResearchPlanner(t *testing.T) {
	p := New()
	resp, err := p.Complete(context.Background(), ai.CompletionRequest{Feature: "ai_research_planner", JSONMode: true})
	if err != nil {
		t.Fatal(err)
	}
	rec := researchplanner.ParseRecommendation(resp.Text)
	if rec.FellBackToRawText {
		t.Fatalf("expected mock research planner placeholder to parse as valid JSON, got fallback. Raw: %s", resp.Text)
	}
}
