package devconsole_test

import (
	"strings"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/game/devconsole"
)

func sampleBalanceSnapshot() devconsole.BalanceSnapshot {
	return devconsole.BalanceSnapshot{
		WindowDays: 7,
		Units: []devconsole.UnitUsageStat{
			{Unit: "soldiers", TotalCompletedRaids: 100, RaidsUsedIn: 95, UsageRatePercent: 95.0, ApparentWinRateWhenUsed: 60.0},
			{Unit: "bombers", TotalCompletedRaids: 100, RaidsUsedIn: 3, UsageRatePercent: 3.0, ApparentWinRateWhenUsed: 100.0},
			{Unit: "wraiths", TotalCompletedRaids: 100, RaidsUsedIn: 0, UsageRatePercent: 0.0, ApparentWinRateWhenUsed: 0.0},
		},
	}
}

func TestBalanceSystemPrompt_WarnsAgainstCausalClaims(t *testing.T) {
	// This is a load-bearing safety property of the whole feature: the
	// model must be told explicitly not to state balance verdicts as
	// fact from correlational data.
	if !strings.Contains(devconsole.BalanceSystemPrompt, "CORRELATIONAL") {
		t.Fatalf("expected BalanceSystemPrompt to explicitly warn that the data is correlational, not causal")
	}
	if !strings.Contains(devconsole.BalanceSystemPrompt, "NEVER state a unit is") {
		t.Fatalf("expected BalanceSystemPrompt to explicitly forbid stating a unit is over/underpowered as fact")
	}
}

func TestFormatBalanceForTelegram_FallbackPath(t *testing.T) {
	rec := &devconsole.BalanceRecommendation{Summary: "raw text", FellBackToRawText: true}
	out := devconsole.FormatBalanceForTelegram(rec)
	if !strings.Contains(out, "Couldn't parse") || !strings.Contains(out, "raw text") {
		t.Errorf("expected fallback notice and raw text, got: %s", out)
	}
}

func TestFormatBalanceForTelegram_TruncatedPath(t *testing.T) {
	rec := &devconsole.BalanceRecommendation{
		Summary:           `{"summary": "cut off mid`,
		FellBackToRawText: true,
		Truncated:         true,
	}
	out := devconsole.FormatBalanceForTelegram(rec)
	if !strings.Contains(out, "cut off before it finished") {
		t.Errorf("expected truncation-specific message, got: %s", out)
	}
}

func TestFormatBalanceForTelegram_StructuredPath(t *testing.T) {
	rec := &devconsole.BalanceRecommendation{
		Summary: "Most raids favor soldiers; a couple units are barely used.",
		UnitNotes: []devconsole.UnitNote{
			{Unit: "bombers", Observation: "Used in only 3% of raids but 100% apparent win rate when used", Caution: "Sample size of 3 is far too small to draw any conclusion"},
		},
		RecommendedFocus: "Look into why bombers are so rarely mobilized despite a strong result when they are",
		Notes:            "Correlational only",
	}
	out := devconsole.FormatBalanceForTelegram(rec)
	for _, want := range []string{
		"BALANCE COMMENTARY",
		"Most raids favor soldiers",
		"bombers", "Sample size of 3 is far too small",
		"Look into why bombers are so rarely mobilized",
		"Correlational only",
		"Correlational data only — not a verdict",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, out)
		}
	}
}

func TestParseBalanceRecommendation_ValidJSON(t *testing.T) {
	raw := `{"summary": "Soldiers dominate usage.", "unit_notes": [{"unit": "bombers", "observation": "rarely used", "caution": "small sample"}], "recommended_focus": "investigate bomber cost", "notes": "correlational"}`
	rec := devconsole.ParseBalanceRecommendation(raw, "")
	if rec.FellBackToRawText {
		t.Fatalf("expected clean JSON parse, got fallback")
	}
	if len(rec.UnitNotes) != 1 || rec.UnitNotes[0].Unit != "bombers" {
		t.Errorf("unexpected unit notes: %+v", rec.UnitNotes)
	}
}

func TestParseBalanceRecommendation_StripsMarkdownFence(t *testing.T) {
	raw := "```json\n" + `{"summary": "ok", "unit_notes": [], "recommended_focus": "", "notes": ""}` + "\n```"
	rec := devconsole.ParseBalanceRecommendation(raw, "")
	if rec.FellBackToRawText {
		t.Fatalf("expected fence to be stripped and JSON parsed, got fallback")
	}
}

func TestParseBalanceRecommendation_FallsBackOnGarbage(t *testing.T) {
	raw := "bombers seem underused I guess"
	rec := devconsole.ParseBalanceRecommendation(raw, "")
	if !rec.FellBackToRawText {
		t.Fatalf("expected fallback for non-JSON text")
	}
	if rec.Truncated {
		t.Errorf("expected Truncated=false for prose with no JSON object at all")
	}
}

// See ADR-016: a response cut off mid-object is distinguished from one
// that never contained JSON at all.
func TestParseBalanceRecommendation_FallsBackOnTruncatedJSON(t *testing.T) {
	raw := `{"summary": "Bombers show a striking pattern worth a closer look because their apparent win rate is high despite being rarely mobil`
	rec := devconsole.ParseBalanceRecommendation(raw, "")
	if !rec.FellBackToRawText {
		t.Fatalf("expected fallback for truncated JSON")
	}
	if !rec.Truncated {
		t.Errorf("expected Truncated=true for a response cut off mid-object")
	}
}

func TestBuildBalanceUserPrompt_IsDeterministic(t *testing.T) {
	s := sampleBalanceSnapshot()
	p1 := devconsole.BuildBalanceUserPrompt(s)
	p2 := devconsole.BuildBalanceUserPrompt(s)
	if p1 != p2 {
		t.Fatalf("expected identical prompts for identical input")
	}
}

func TestBuildBalanceUserPrompt_ContainsKeyFacts(t *testing.T) {
	p := devconsole.BuildBalanceUserPrompt(sampleBalanceSnapshot())
	for _, want := range []string{
		"last 7 day(s)",
		"soldiers: used in 95/100 raids (95.0% usage rate)",
		"bombers: used in 3/100 raids (3.0% usage rate), apparent win rate when used: 100.0%",
		"wraiths: used in 0/100 raids (0.0% usage rate)",
		"unit_notes",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("expected prompt to contain %q, got:\n%s", want, p)
		}
	}
}

func TestBuildBalanceUserPrompt_NoRaidsPath(t *testing.T) {
	s := devconsole.BalanceSnapshot{WindowDays: 7}
	p := devconsole.BuildBalanceUserPrompt(s)
	if !strings.Contains(p, "No completed raids in this window.") {
		t.Errorf("expected no-raids message, got:\n%s", p)
	}
}

// TestFormatBalanceForTelegram_RendersRealDataTable is the regression
// test for the 2026-08-01 UI pass: the unit-usage table must be built
// from Snapshot's real numbers, not from anything the model wrote, so
// a made-up or mistranscribed AI figure can never end up looking like
// ground truth. It also checks the zero-usage case renders as "—"
// rather than a misleading literal 0% win rate (there's no rate to
// speak of if the unit was never deployed).
func TestFormatBalanceForTelegram_RendersRealDataTable(t *testing.T) {
	snap := sampleBalanceSnapshot()
	rec := &devconsole.BalanceRecommendation{
		Summary:  "Soldiers dominate; bombers rarely used.",
		Snapshot: &snap,
	}
	out := devconsole.FormatBalanceForTelegram(rec)

	if !strings.Contains(out, "<pre>") || !strings.Contains(out, "</pre>") {
		t.Fatalf("expected a <pre>-wrapped table, got: %s", out)
	}
	for _, want := range []string{"soldiers", "95", "60", "bombers", "wraiths"} {
		if !strings.Contains(out, want) {
			t.Errorf("expected table to contain %q (from real Snapshot data), got: %s", want, out)
		}
	}
	// wraiths was never deployed (RaidsUsedIn: 0) - must show "—", not "0",
	// for its win rate, since a real 0% would misleadingly imply it lost
	// every time it was used.
	if !strings.Contains(out, "—") {
		t.Errorf("expected a \"—\" placeholder for the never-deployed unit's win rate, got: %s", out)
	}
}

// TestFormatBalanceForTelegram_CautionUsesExpandableQuote guards the
// other half of the same UI pass: per-unit AI cautions render as a
// collapsible blockquote rather than dumped inline, since they can run
// long and this was previously a flat, unformatted wall of text (the
// one AI Developer Console report the original HTML polish campaign
// never reached).
func TestFormatBalanceForTelegram_CautionUsesExpandableQuote(t *testing.T) {
	rec := &devconsole.BalanceRecommendation{
		Summary: "test",
		UnitNotes: []devconsole.UnitNote{
			{Unit: "bombers", Observation: "rarely used", Caution: "sample size too small"},
		},
	}
	out := devconsole.FormatBalanceForTelegram(rec)
	if !strings.Contains(out, "<blockquote expandable>") {
		t.Errorf("expected unit caution to be wrapped in an expandable blockquote, got: %s", out)
	}
	if !strings.Contains(out, "sample size too small") {
		t.Errorf("expected caution text to still be present, got: %s", out)
	}
}

// TestFormatBalanceForTelegram_NilSnapshotOmitsTable makes sure the
// table section is skipped cleanly (not a nil-pointer panic) when
// Snapshot wasn't set - the case every hand-built test
// BalanceRecommendation above exercises, since none of them set it.
func TestFormatBalanceForTelegram_NilSnapshotOmitsTable(t *testing.T) {
	rec := &devconsole.BalanceRecommendation{Summary: "test"}
	out := devconsole.FormatBalanceForTelegram(rec)
	if strings.Contains(out, "<pre>") {
		t.Errorf("expected no table when Snapshot is nil, got: %s", out)
	}
}
