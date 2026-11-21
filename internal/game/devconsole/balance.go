package devconsole

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

var mobilizableUnitColumns = []string{
	"soldiers_mobilized", "mechs_mobilized", "buggies_mobilized",
	"destroyers_mobilized", "bombers_mobilized", "battlecruisers_mobilized",
	"deathstars_mobilized", "liberators_mobilized", "wraiths_mobilized",
}

type UnitUsageStat struct {
	Unit                    string
	TotalCompletedRaids     int
	RaidsUsedIn             int
	UsageRatePercent        float64
	ApparentWinRateWhenUsed float64
}

type BalanceSnapshot struct {
	WindowDays int
	Units      []UnitUsageStat
}

type UnitNote struct {
	Unit        string `json:"unit"`
	Observation string `json:"observation"`
	Caution     string `json:"caution"`
}

type BalanceRecommendation struct {
	Summary          string     `json:"summary"`
	UnitNotes        []UnitNote `json:"unit_notes"`
	RecommendedFocus string     `json:"recommended_focus"`
	Notes            string     `json:"notes"`

	Snapshot *BalanceSnapshot `json:"-"`

	FellBackToRawText bool
	Truncated         bool
}

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

func ParseBalanceRecommendation(text string, stopReason string) *BalanceRecommendation {
	candidate, found := ai.ExtractJSONObject(text)
	if !found {
		return &BalanceRecommendation{Summary: text, FellBackToRawText: true, Truncated: ai.WasTruncated(text) || ai.IsTruncatedStopReason(stopReason)}
	}
	var rec BalanceRecommendation
	if err := json.Unmarshal([]byte(candidate), &rec); err == nil && rec.Summary != "" {
		return &rec
	}
	repaired := ai.SanitizeJSONControlChars(candidate)
	if err := json.Unmarshal([]byte(repaired), &rec); err == nil && rec.Summary != "" {
		return &rec
	}
	return &BalanceRecommendation{Summary: text, FellBackToRawText: true, Truncated: ai.WasTruncated(text) || ai.IsTruncatedStopReason(stopReason)}
}

func FormatBalanceForTelegram(rec *BalanceRecommendation) string {
	var b strings.Builder
	b.WriteString("⚖️ " + ai.HTMLBold("AI DEVELOPER CONSOLE — BALANCE COMMENTARY") + "\n\n")

	if rec.FellBackToRawText {
		if rec.Truncated {
			b.WriteString("⚠️ " + ai.HTMLItalic("The AI's response got cut off before it finished — showing the partial reply below:") + "\n\n")
		} else {
			b.WriteString("⚠️ " + ai.HTMLItalic("Couldn't parse the AI's structured response — showing its raw reply below:") + "\n\n")
		}
		b.WriteString(ai.HTMLPre(ai.HTMLEscape(rec.Summary)))
		return b.String()
	}

	fmt.Fprintf(&b, "📋 %s\n\n", ai.HTMLEscape(rec.Summary))

	if rec.Snapshot != nil && len(rec.Snapshot.Units) > 0 {
		headers := []string{"Unit", "Used%", "Win%"}
		rows := make([][]string, 0, len(rec.Snapshot.Units))
		for _, u := range rec.Snapshot.Units {
			winCell := fmt.Sprintf("%.0f", u.ApparentWinRateWhenUsed)
			if u.RaidsUsedIn == 0 {
				winCell = "—"
			}
			rows = append(rows, []string{u.Unit, fmt.Sprintf("%.0f", u.UsageRatePercent), winCell})
		}
		b.WriteString("📊 " + ai.HTMLBold("UNIT USAGE (real data, this window)") + "\n")
		b.WriteString(ai.HTMLTable(headers, rows) + "\n\n")
	}

	if len(rec.UnitNotes) > 0 {
		b.WriteString("🔎 " + ai.HTMLBold("AI COMMENTARY") + "\n")
		for _, n := range rec.UnitNotes {
			fmt.Fprintf(&b, "  • %s: %s\n", ai.HTMLBold(ai.HTMLEscape(n.Unit)), ai.HTMLEscape(n.Observation))
			if n.Caution != "" {
				b.WriteString(ai.HTMLExpandableQuote("⚠️ "+ai.HTMLEscape(n.Caution)) + "\n")
			}
		}
		b.WriteString("\n")
	}

	if rec.RecommendedFocus != "" {
		fmt.Fprintf(&b, "🎯 Worth investigating: %s\n", ai.HTMLEscape(rec.RecommendedFocus))
	}
	if rec.Notes != "" {
		fmt.Fprintf(&b, "📝 %s\n", ai.HTMLEscape(rec.Notes))
	}

	b.WriteString("\n" + ai.HTMLItalic("Correlational data only — not a verdict on any unit's balance. Nothing has been changed automatically."))
	return b.String()
}

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
		MaxTokens:   4096,
		Temperature: 0.3,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("devconsole: balance ai completion failed: %w", err)
	}

	rec := ParseBalanceRecommendation(resp.Text, resp.StopReason)
	rec.Snapshot = snapshot

	if co.AI.Memory != nil {
		_ = co.AI.Memory.Append(ctx, callerUserID, balanceMemoryScope, ai.Message{Role: ai.RoleAssistant, Content: resp.Text})
	}

	return rec, nil
}
