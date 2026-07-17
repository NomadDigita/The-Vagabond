package econadvisor_test

import (
	"strings"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/game/econadvisor"
)

func sampleSnapshot() econadvisor.Snapshot {
	return econadvisor.Snapshot{
		EncampmentID: "abc-123",
		Name:         "Fort Wasteland",
		Level:        6,
		Resources: map[string]float64{
			"scrap": 4200, "rations": 300, "electricity": 12,
			"metal": 90, "crystal": 3, "hydrogen": 0, "dollars": 1500, "ether": 2, "neuro_cores": 5,
		},
		Modules: []econadvisor.ModuleState{
			{Type: "warehouse", Level: 2},
			{Type: "generator", Level: 1},
		},
		BankBalance: 500, BankBalanceCash: 200, LoanAmount: 3000, LoanCash: 0,
		OwnListings: []econadvisor.MarketListing{{ItemType: "metal", Quantity: 50, PriceDollars: 4.5}},
		MarketStats: []econadvisor.MarketItemStats{{ItemType: "metal", ActiveCount: 12, AveragePrice: 5.0, MinPrice: 3.0, MaxPrice: 8.0}},
	}
}

func TestBuildUserPrompt_Deterministic(t *testing.T) {
	s := sampleSnapshot()
	p1 := econadvisor.BuildUserPrompt(s)
	p2 := econadvisor.BuildUserPrompt(s)
	if p1 != p2 {
		t.Fatalf("expected identical prompts for identical input")
	}
}

func TestBuildUserPrompt_ContainsKeyFacts(t *testing.T) {
	p := econadvisor.BuildUserPrompt(sampleSnapshot())
	for _, want := range []string{
		"Fort Wasteland", "generator: level 1", "debt 3000.0",
		"metal x50", "12 active listings", "top_roi_actions",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("expected prompt to contain %q, got:\n%s", want, p)
		}
	}
}

func TestBuildUserPrompt_EmptyMarketPath(t *testing.T) {
	s := econadvisor.Snapshot{EncampmentID: "x", Name: "Bare Camp", Resources: map[string]float64{}}
	p := econadvisor.BuildUserPrompt(s)
	if !strings.Contains(p, "(none)") || !strings.Contains(p, "(no active market data)") {
		t.Errorf("expected empty-listing and empty-market messages, got:\n%s", p)
	}
}

func TestParseRecommendation_ValidJSON(t *testing.T) {
	raw := `{"summary": "Upgrade warehouse.", "top_roi_actions": [{"action": "upgrade", "target": "warehouse", "reason": "near cap", "expected_gain": "+500 storage"}], "bottlenecks": "scrap near cap", "market_timing": "sell now", "trading_advice": "diversify"}`
	rec := econadvisor.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected clean JSON parse, got fallback")
	}
	if len(rec.TopROIActions) != 1 || rec.TopROIActions[0].ExpectedGain != "+500 storage" {
		t.Errorf("unexpected actions: %+v", rec.TopROIActions)
	}
}

func TestParseRecommendation_FallsBackOnGarbage(t *testing.T) {
	raw := "sell your metal I guess"
	rec := econadvisor.ParseRecommendation(raw)
	if !rec.FellBackToRawText {
		t.Fatalf("expected fallback for non-JSON text")
	}
}

// See PROJECT_MASTER_PLAN.md ADR-015 / §1.8 — reproduces a real
// production failure where prose or a raw newline around/inside an
// otherwise-valid JSON object made json.Unmarshal fail outright.

func TestParseRecommendation_TrailingProseAroundJSON(t *testing.T) {
	raw := `{"summary": "Focus on storage.", "top_roi_actions": []}` +
		"\n\nHappy to dig into any of these further."
	rec := econadvisor.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected trailing prose around valid JSON to still parse, got fallback. Raw: %s", raw)
	}
	if rec.Summary != "Focus on storage." {
		t.Errorf("unexpected summary: %q", rec.Summary)
	}
}

func TestParseRecommendation_RawNewlineInsideStringValue(t *testing.T) {
	raw := "{\"summary\": \"Economy is advanced\nbut severely unbalanced.\", \"top_roi_actions\": []}"
	rec := econadvisor.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected raw newline inside string value to be repaired, got fallback. Raw: %s", raw)
	}
	if !strings.Contains(rec.Summary, "severely unbalanced") {
		t.Errorf("unexpected summary: %q", rec.Summary)
	}
}

func TestFormatForTelegram_StructuredPath(t *testing.T) {
	rec := econadvisor.ParseRecommendation(`{"summary": "Focus on storage.", "top_roi_actions": [{"action": "upgrade", "target": "warehouse", "reason": "overflow risk", "expected_gain": "+30% capacity"}], "bottlenecks": "scrap overflow", "market_timing": "prices are high, sell", "trading_advice": "hold crystal"}`)
	out := econadvisor.FormatForTelegram(rec)
	for _, want := range []string{"Focus on storage.", "+30% capacity", "scrap overflow", "prices are high, sell", "hold crystal", "recommendation only"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}
