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

func TestBuildRogueNestTarget_IncludesEnrichedDefenseData(t *testing.T) {
	// Level 20 crosses every enrichment threshold confirmed in
	// content.RogueNestComposition (turrets, guardians, observers,
	// integrity tech, shields, and the hero-superpower threshold at
	// level 20), so this exercises the full enriched path, not just
	// the legacy Soldiers/Mechs/Drones/Jets fields.
	target := fleetcommander.BuildRogueNestTarget(20)

	if target.IntegrityTechLvl <= 0 {
		t.Errorf("expected non-zero IntegrityTechLvl at level 20, got %d", target.IntegrityTechLvl)
	}
	if target.Garrison["guardians"] <= 0 {
		t.Errorf("expected non-zero guardians in garrison at level 20, got %+v", target.Garrison)
	}
	if target.Garrison["observers"] <= 0 {
		t.Errorf("expected non-zero observers in garrison at level 20, got %+v", target.Garrison)
	}
	if target.TurretGrid["light_laser"] <= 0 {
		t.Errorf("expected non-zero light_laser turret level at level 20, got %+v", target.TurretGrid)
	}
	if target.HeroSuperpower == "" {
		t.Errorf("expected a hero superpower to be present at level 20 (the documented threshold), got empty string")
	}

	// The enriched data must actually reach the rendered prompt, not
	// just live unused on the struct.
	prompt := fleetcommander.BuildUserPrompt(fleetcommander.FleetComposition{"soldiers": 100}, target, fleetcommander.CombatHistorySummary{})
	for _, want := range []string{"Defense Grid", "Integrity Tech level", target.HeroSuperpower} {
		if !strings.Contains(prompt, want) {
			t.Errorf("expected prompt to surface enriched defense data (%q), got:\n%s", want, prompt)
		}
	}
}
