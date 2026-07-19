// This file implements balance-suggestion tooling: real unit
// usage/outcome statistics from completed raids, with AI commentary
// on what they might imply for game balance.
//
// Honesty boundary, stated up front because it matters: these are
// CORRELATIONAL statistics from real games, not a controlled
// experiment. A unit showing a high "win rate when used" could mean
// the unit is strong, or could just mean stronger/more experienced
// players happen to favor it — the data alone can't tell those apart.
// SystemPrompt instructs the model to flag this explicitly rather than
// hand admins a false sense of causal certainty. This mirrors the
// project's existing discipline (ADR-017/018/019) of never presenting
// a proxy or a correlation as more certain than it actually is.
package devconsole

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

// mobilizableUnitColumns are raid_forces' own mobilized-count columns
// — a fixed, hardcoded list (never derived from user or model input),
// so building a column name into SQL here carries no injection risk.
var mobilizableUnitColumns = []string{
	"soldiers_mobilized", "mechs_mobilized", "buggies_mobilized",
	"destroyers_mobilized", "bombers_mobilized", "battlecruisers_mobilized",
	"deathstars_mobilized", "liberators_mobilized", "wraiths_mobilized",
}

// UnitUsageStat is one unit type's real usage/outcome correlation
// across completed raids in the reporting window.
type UnitUsageStat struct {
	Unit                    string
	TotalCompletedRaids     int
	RaidsUsedIn             int
	UsageRatePercent        float64
	ApparentWinRateWhenUsed float64 // 0 if RaidsUsedIn is 0 — see BuildBalanceSnapshot
}

// BalanceSnapshot is everything fed to the model for one balance
// commentary pass.
type BalanceSnapshot struct {
	WindowDays int
	Units      []UnitUsageStat
}

// UnitNote is the model's per-unit commentary.
type UnitNote struct {
	Unit        string `json:"unit"`
	Observation string `json:"observation"`
	Caution     string `json:"caution"`
}

// BalanceRecommendation is the Developer Console's structured output
// for a balance-commentary pass.
type BalanceRecommendation struct {
	Summary          string     `json:"summary"`
	UnitNotes        []UnitNote `json:"unit_notes"`
	RecommendedFocus string     `json:"recommended_focus"`
	Notes            string     `json:"notes"`

	FellBackToRawText bool
	Truncated         bool
}

// BalanceSystemPrompt is the fixed instruction for the balance-
// commentary call.
const BalanceSystemPrompt = `You are the AI Developer Console for The Vagabond, giving admins balance commentary based on real unit usage/outcome statistics from completed raids. You NEVER take any action yourself — you only observe and suggest what's worth investigating.

Critical epistemic rule: this data is CORRELATIONAL, not a controlled experiment. A unit with a high apparent win rate when used could mean the unit is strong, OR could simply mean stronger/more experienced players happen to favor it — you cannot tell these apart from this data alone. NEVER state a unit is "overpowered" or "underpowered" as a fact. Always frame observations as "worth investigating" or "a pattern worth a closer look," never as a verdict.

Rules:
- If a unit's RaidsUsedIn is small (say, under 10), say so explicitly and treat any percentage from it as low-confidence.
- If a unit is barely used at all (usage rate near 0%), that itself is worth noting — it may mean the unit is unattractive, too expensive, or players don't know about it — not necessarily that it's weak in combat.
- recommended_focus should name the ONE most interesting pattern worth a human balance designer actually looking into, not a list of every unit.
- Keep summary to 2-3 sentences.`

func BuildBalanceUserPrompt(s BalanceSnapshot) string {
	var b strings.Builder
	fmt.Fprintf(&b, "REPORTING WINDOW: last %d day(s)\n\n", s.WindowDays)
	b.WriteString("UNIT USAGE/OUTCOME STATS (from completed raids, attacker side):\n")
	if len(s.Units) == 0 {
		b.WriteString("  No completed raids in this window.\n")
	} else {
		for _, u := range s.Units {
			fmt.Fprintf(&b, "  - %s: used in %d/%d raids (%.1f%% usage rate), apparent win rate when used: %.1f%%\n",
				u.Unit, u.RaidsUsedIn, u.TotalCompletedRaids, u.UsageRatePercent, u.ApparentWinRateWhenUsed)
		}
	}

	b.WriteString("\nRespond with a single JSON object matching this shape exactly:\n")
	b.WriteString(`{"summary": "...", "unit_notes": [{"unit": "...", "observation": "...", "caution": "..."}], "recommended_focus": "...", "notes": "..."}`)

	return b.String()
}

func ParseBalanceRecommendation(text string) *BalanceRecommendation {
	candidate, found := ai.ExtractJSONObject(text)
	if !found {
		return &BalanceRecommendation{Summary: text, FellBackToRawText: true, Truncated: ai.WasTruncated(text)}
	}
	var rec BalanceRecommendation
	if err := json.Unmarshal([]byte(candidate), &rec); err == nil && rec.Summary != "" {
		return &rec
	}
	repaired := ai.SanitizeJSONControlChars(candidate)
	if err := json.Unmarshal([]byte(repaired), &rec); err == nil && rec.Summary != "" {
		return &rec
	}
	return &BalanceRecommendation{Summary: text, FellBackToRawText: true, Truncated: ai.WasTruncated(text)}
}

// FormatBalanceForTelegram renders a BalanceRecommendation as a
// plain-text Telegram reply.
func FormatBalanceForTelegram(rec *BalanceRecommendation) string {
	var b strings.Builder
	b.WriteString("⚖️ AI DEVELOPER CONSOLE — BALANCE COMMENTARY\n\n")

	if rec.FellBackToRawText {
		if rec.Truncated {
			b.WriteString("⚠️ The AI's response got cut off before it finished — showing the partial reply below:\n\n")
		} else {
			b.WriteString("⚠️ Couldn't parse the AI's structured response — showing its raw reply below:\n\n")
		}
		fmt.Fprintf(&b, "```\n%s\n```", rec.Summary)
		return b.String()
	}

	fmt.Fprintf(&b, "📋 %s\n\n", rec.Summary)

	if len(rec.UnitNotes) > 0 {
		b.WriteString("UNIT NOTES:\n")
		for _, n := range rec.UnitNotes {
			fmt.Fprintf(&b, "  • %s: %s\n    ⚠️ %s\n", n.Unit, n.Observation, n.Caution)
		}
		b.WriteString("\n")
	}

	if rec.RecommendedFocus != "" {
		fmt.Fprintf(&b, "🎯 Worth investigating: %s\n", rec.RecommendedFocus)
	}
	if rec.Notes != "" {
		fmt.Fprintf(&b, "📝 %s\n", rec.Notes)
	}

	b.WriteString("\n⚠️ Correlational data only — not a verdict on any unit's balance. Nothing has been changed automatically.")
	return b.String()
}

// BuildBalanceSnapshot computes real usage/outcome correlations for
// every mobilizable unit type across completed raids in the last
// windowDays days. "Apparent win" reuses the same stolen-resources
// heuristic already established for the attacker side in
// fleetcommander.BuildCombatHistory and battleanalyst — see those
// packages' own doc comments for why it's a proxy, not an
// authoritative outcome column.
func (co *Console) BuildBalanceSnapshot(ctx context.Context, windowDays int) (*BalanceSnapshot, error) {
	since := windowSince(windowDays)
	s := &BalanceSnapshot{WindowDays: windowDays}

	for _, col := range mobilizableUnitColumns {
		query := fmt.Sprintf(`
			SELECT
				COUNT(*) AS total,
				COUNT(*) FILTER (WHERE rf.%s > 0) AS used_count,
				COUNT(*) FILTER (WHERE rf.%s > 0 AND (r.stolen_scrap + r.stolen_metal + r.stolen_crystal) > 0) AS used_and_won
			FROM raids r
			JOIN raid_forces rf ON rf.raid_id = r.id
			WHERE r.state = 'completed' AND r.resolve_time >= $1`, col, col)

		var total, usedCount, usedAndWon int
		if err := co.DB.QueryRowContext(ctx, query, since).Scan(&total, &usedCount, &usedAndWon); err != nil {
			return nil, fmt.Errorf("devconsole: balance stats for %s: %w", col, err)
		}

		stat := UnitUsageStat{
			Unit:                strings.TrimSuffix(col, "_mobilized"),
			TotalCompletedRaids: total,
			RaidsUsedIn:         usedCount,
		}
		if total > 0 {
			stat.UsageRatePercent = float64(usedCount) / float64(total) * 100
		}
		if usedCount > 0 {
			stat.ApparentWinRateWhenUsed = float64(usedAndWon) / float64(usedCount) * 100
		}
		s.Units = append(s.Units, stat)
	}

	return s, nil
}

// RecommendBalance produces a fresh AI balance-commentary pass for the
// last windowDays days. It stores both turns in ai_memory under its
// own scope, distinct from the weekly-report and NL-query scopes.
//
// Read-only: nothing in this method changes any unit, cost, or game
// setting — it only reads and comments.
func (co *Console) RecommendBalance(ctx context.Context, callerUserID int64, windowDays int) (*BalanceRecommendation, error) {
	snapshot, err := co.BuildBalanceSnapshot(ctx, windowDays)
	if err != nil {
		return nil, err
	}

	userPrompt := BuildBalanceUserPrompt(*snapshot)
	const balanceMemoryScope = "dev_console_balance"

	if co.AI.Memory != nil {
		_ = co.AI.Memory.Append(ctx, callerUserID, balanceMemoryScope, ai.Message{Role: ai.RoleUser, Content: userPrompt})
	}

	resp, err := co.AI.Complete(ctx, ai.CompletionRequest{
		Feature:     string(ai.FeatureDevConsole),
		UserID:      callerUserID,
		System:      BalanceSystemPrompt,
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: userPrompt}},
		MaxTokens:   2048,
		Temperature: 0.3,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("devconsole: balance ai completion failed: %w", err)
	}

	rec := ParseBalanceRecommendation(resp.Text)

	if co.AI.Memory != nil {
		_ = co.AI.Memory.Append(ctx, callerUserID, balanceMemoryScope, ai.Message{Role: ai.RoleAssistant, Content: resp.Text})
	}

	return rec, nil
}
