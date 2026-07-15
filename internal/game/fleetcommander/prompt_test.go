package fleetcommander_test

import (
	"strings"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/game/fleetcommander"
)

func TestBuildUserPrompt_IsDeterministic(t *testing.T) {
	own := fleetcommander.FleetComposition{"soldiers": 50, "buggies": 10, "jets": 2}
	target := fleetcommander.TargetProfile{
		Name: "Rogue Drone Nest", IsPvE: true, ThreatTier: "Moderate",
		Garrison:    fleetcommander.FleetComposition{"soldiers": 30, "mechs": 4},
		TurretBonus: 0.1,
	}
	history := fleetcommander.CombatHistorySummary{RaidsAnalyzed: 5, ApparentWins: 3, AverageLosses: 4.2}

	p1 := fleetcommander.BuildUserPrompt(own, target, history)
	p2 := fleetcommander.BuildUserPrompt(own, target, history)
	if p1 != p2 {
		t.Fatalf("expected identical prompts for identical input (cache stability)")
	}
	for _, want := range []string{"soldiers: 50", "buggies: 10", "Moderate", "5 raids analyzed", "recommendation"} {
		if !strings.Contains(p1, want) {
			t.Errorf("expected prompt to contain %q, got:\n%s", want, p1)
		}
	}
}

func TestBuildUserPrompt_NoHistoryPath(t *testing.T) {
	own := fleetcommander.FleetComposition{"soldiers": 10}
	target := fleetcommander.TargetProfile{Name: "Rogue Drone Nest", IsPvE: true, ThreatTier: "Low"}
	p := fleetcommander.BuildUserPrompt(own, target, fleetcommander.CombatHistorySummary{})
	if !strings.Contains(p, "No recorded raids yet") {
		t.Errorf("expected no-history message, got:\n%s", p)
	}
}

func TestParseRecommendation_ValidJSON(t *testing.T) {
	raw := `{"recommendation": "attack", "confidence": "high", "reasoning": "you outnumber them 2:1", "risk_assessment": "low", "suggested_split": ""}`
	rec := fleetcommander.ParseRecommendation(raw)
	if rec.FellBackToRawText {
		t.Fatalf("expected clean JSON parse, got fallback")
	}
	if rec.Recommendation != "attack" {
		t.Errorf("unexpected recommendation: %q", rec.Recommendation)
	}
}

func TestParseRecommendation_FallsBackOnGarbage(t *testing.T) {
	raw := "yeah go for it probably"
	rec := fleetcommander.ParseRecommendation(raw)
	if !rec.FellBackToRawText {
		t.Fatalf("expected fallback for non-JSON text")
	}
	if rec.Reasoning != raw {
		t.Errorf("expected raw text preserved, got %q", rec.Reasoning)
	}
}

func TestFormatForTelegram_StructuredPath(t *testing.T) {
	rec := fleetcommander.ParseRecommendation(`{"recommendation": "retreat", "confidence": "medium", "reasoning": "garrison too strong", "risk_assessment": "high losses expected", "suggested_split": ""}`)
	out := fleetcommander.FormatForTelegram(rec)
	for _, want := range []string{"RETREAT", "garrison too strong", "high losses expected", "recommendation only"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected formatted output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestFormatForTelegram_FallbackPath(t *testing.T) {
	rec := fleetcommander.ParseRecommendation("not json at all")
	out := fleetcommander.FormatForTelegram(rec)
	if !strings.Contains(out, "not json at all") {
		t.Errorf("expected raw fallback text included, got: %s", out)
	}
}

func TestBuildRogueNestTarget_MatchesContentPackage(t *testing.T) {
	target := fleetcommander.BuildRogueNestTarget(10)
	if !target.IsPvE {
		t.Errorf("expected rogue nest target to be marked PvE")
	}
	if target.Garrison["soldiers"] <= 0 {
		t.Errorf("expected non-zero soldier garrison at level 10, got %+v", target.Garrison)
	}
}
