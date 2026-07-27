package devconsole

import (
	"context"
	"fmt"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/engine/world"
	"github.com/NomadDigita/The-Vagabond/internal/game/scoring"
)

// ── Safety model for natural-language admin queries ──────────────────
//
// The model NEVER writes or sees raw SQL, and nothing it outputs is
// ever concatenated into a query. Instead: the model picks one name
// from queryIntents (a fixed whitelist) plus a small number of
// bounded, validated integer parameters (see clampDays/clampLimit in
// nlquery.go); this file maps that choice to one specific, already-
// written parameterized query and executes it. An admin asking "drop
// all users" or "run this SQL: ..." has no path to actually doing
// that — the worst a malicious or confused free-text question can do
// is select an intent from this list, which are all read-only by
// construction (every query below is a SELECT). This mirrors the
// discipline used throughout the AI Systems Roadmap of never letting
// model output become a code/query path — see ADR-019 and this
// package's own doc comment for why Phase J stayed deliberately
// narrow, and this file's addition is the one place that narrowness
// gets safely widened: more READ-ONLY intents, never open-ended
// execution.
var queryIntents = map[string]string{
	"new_players":      "Recent player signups: name, username, join time, home continent.",
	"top_players":      "Top players by the same score formula the Global Ranking panel uses.",
	"active_users":     "Count of users active (last_active) within a given window.",
	"totals":           "All-time totals: users, encampments, clans.",
	"economy_snapshot": "Average scrap/metal/crystal/dollars held across all encampments right now.",
	"combat_stats":     "Count of completed raids in a window, plus average attacker/defender losses.",
	"clan_stats":       "Total clans, average members per clan, how many are currently recruiting.",
	"world_state":      "Current world event (if any) on each of the four continents.",
	"recent_news":      "Most recent sector news headlines.",

	// Phase 7 milestone 3: "Add admin/dev-console metrics for discovery
	// rate, travel delays, stranded armies, convoy outcomes, field-battle
	// frequency, AI growth, loot mix, Crystal flow, and speed-up
	// expenditure." All nine, added together since they're one milestone.
	"discovery_rate":         "How many bases/expeditions were discovered in a window, broken down by discovery method (route contact vs. exploration vs. recon).",
	"travel_delays":          "Snapshot of how many currently-marching columns are halted right now and why (weather camp, road encounter, out of supplies).",
	"stranded_armies":        "Count and list of columns currently out of supplies and awaiting a resupply convoy or retreat order.",
	"convoy_outcomes":        "Reinforcement convoys dispatched in a window, broken down by outcome (delivered, missed contact, ambushed).",
	"field_battle_frequency": "How many road-vs-road and road-vs-base skirmishes were fought (not just passed peacefully) in a window.",
	"ai_growth":              "Current count and average level of the seeded AI civilizations (Phase 6), right now.",
	"loot_mix":               "Which resource type dominates raid loot in a window - percentage breakdown of everything captured across completed raids.",
	"crystal_flow":           "Total Crystal currently held across all encampments, plus total Crystal captured via raids in a window.",
	"speedup_expenditure":    "Count and total resource cost of emergency speed-ups and early camp-breaks used in a window (logging started with this feature - no data before it shipped).",
}

// IntentDescriptions renders the whitelist for the classification
// prompt (see nlquery.go's SystemPrompt) — keeping the human-readable
// description next to the map above means they can't drift apart.
func IntentDescriptions() string {
	var out string
	for _, name := range []string{"new_players", "top_players", "active_users", "totals", "economy_snapshot", "combat_stats", "clan_stats", "world_state", "recent_news",
		"discovery_rate", "travel_delays", "stranded_armies", "convoy_outcomes", "field_battle_frequency", "ai_growth", "loot_mix", "crystal_flow", "speedup_expenditure"} {
		out += fmt.Sprintf("- %s: %s\n", name, queryIntents[name])
	}
	return out
}

// IsKnownIntent reports whether name is in the whitelist. Any name the
// model returns that isn't in this list is rejected outright by
// nlquery.go, never executed as a fallback guess.
func IsKnownIntent(name string) bool {
	_, ok := queryIntents[name]
	return ok
}

// RunIntent executes exactly one whitelisted, already-parameterized
// query and renders the result as a plain-text data block for the
// model's second (answer) call. days and limit are pre-validated by
// the caller (see clampDays/clampLimit in nlquery.go) before reaching
// here.
func (co *Console) RunIntent(ctx context.Context, intent string, days, limit int) (string, error) {
	switch intent {
	case "new_players":
		since := windowSince(days)
		players, err := co.buildNewPlayers(ctx, since)
		if err != nil {
			return "", fmt.Errorf("devconsole: run new_players: %w", err)
		}
		var count int
		if err := co.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE registered_at >= $1`, since).Scan(&count); err != nil {
			return "", fmt.Errorf("devconsole: count new_players: %w", err)
		}
		out := fmt.Sprintf("New players in the last %d day(s): %d total\n", days, count)
		for _, p := range players {
			name := p.FirstName
			if p.Username != "" {
				name = fmt.Sprintf("%s (@%s)", name, p.Username)
			}
			continent := p.HomeContinent
			if continent == "" {
				continent = "no outpost yet"
			}
			out += fmt.Sprintf("  - %s, joined %s, home: %s\n", name, p.JoinedAt, continent)
		}
		return out, nil

	case "top_players":
		top, err := co.buildTopPlayersN(ctx, limit)
		if err != nil {
			return "", fmt.Errorf("devconsole: run top_players: %w", err)
		}
		out := fmt.Sprintf("Top %d players by all-time ranking score:\n", limit)
		for i, tp := range top {
			out += fmt.Sprintf("  %d. %s — Level %d, Score %.0f\n", i+1, tp.Name, tp.Level, tp.Score)
		}
		return out, nil

	case "active_users":
		since := windowSince(days)
		var count int
		if err := co.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE last_active >= $1`, since).Scan(&count); err != nil {
			return "", fmt.Errorf("devconsole: run active_users: %w", err)
		}
		return fmt.Sprintf("Active users (last_active) in the last %d day(s): %d\n", days, count), nil

	case "totals":
		var users, camps, clans int
		if err := co.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&users); err != nil {
			return "", fmt.Errorf("devconsole: run totals (users): %w", err)
		}
		if err := co.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM encampments`).Scan(&camps); err != nil {
			return "", fmt.Errorf("devconsole: run totals (encampments): %w", err)
		}
		_ = co.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM clans`).Scan(&clans) // clans table may not exist on older schemas; ignore error
		return fmt.Sprintf("All-time totals: %d users, %d encampments, %d clans\n", users, camps, clans), nil

	case "economy_snapshot":
		var avgScrap, avgMetal, avgCrystal, avgDollars float64
		err := co.DB.QueryRowContext(ctx, `
			SELECT COALESCE(AVG(scrap),0), COALESCE(AVG(metal),0), COALESCE(AVG(crystal),0), COALESCE(AVG(dollars),0)
			FROM resources`).Scan(&avgScrap, &avgMetal, &avgCrystal, &avgDollars)
		if err != nil {
			return "", fmt.Errorf("devconsole: run economy_snapshot: %w", err)
		}
		return fmt.Sprintf("Average held per encampment right now: %.0f scrap, %.0f metal, %.0f crystal, %.0f dollars\n",
			avgScrap, avgMetal, avgCrystal, avgDollars), nil

	case "combat_stats":
		since := windowSince(days)
		var raidCount int
		var avgAttackerLosses, avgDefenderLosses float64
		err := co.DB.QueryRowContext(ctx, `
			SELECT COUNT(*), COALESCE(AVG(attacker_losses),0), COALESCE(AVG(defender_losses),0)
			FROM raids
			WHERE state = 'completed' AND resolve_time >= $1`, since).Scan(&raidCount, &avgAttackerLosses, &avgDefenderLosses)
		if err != nil {
			return "", fmt.Errorf("devconsole: run combat_stats: %w", err)
		}
		return fmt.Sprintf("Completed raids in the last %d day(s): %d — average attacker losses %.1f, average defender losses %.1f\n",
			days, raidCount, avgAttackerLosses, avgDefenderLosses), nil

	case "clan_stats":
		var totalClans, recruitingClans int
		var avgMembers float64
		if err := co.DB.QueryRowContext(ctx, `SELECT COUNT(*), COUNT(*) FILTER (WHERE recruiting) FROM clans`).Scan(&totalClans, &recruitingClans); err != nil {
			return "", fmt.Errorf("devconsole: run clan_stats (clans): %w", err)
		}
		if totalClans > 0 {
			if err := co.DB.QueryRowContext(ctx, `SELECT COUNT(*)::float / NULLIF((SELECT COUNT(*) FROM clans),0) FROM user_clans`).Scan(&avgMembers); err != nil {
				return "", fmt.Errorf("devconsole: run clan_stats (avg members): %w", err)
			}
		}
		return fmt.Sprintf("Total clans: %d (%d currently recruiting), average members per clan: %.1f\n",
			totalClans, recruitingClans, avgMembers), nil

	case "world_state":
		activeByContinent := world.ActiveEventsByContinent(ctx, co.DB)
		out := "Current world event per continent:\n"
		for _, continent := range world.Continents {
			eventType := activeByContinent[continent]
			if eventType == "" {
				eventType = "nominal"
			}
			out += fmt.Sprintf("  - %s: %s\n", continent, eventType)
		}
		return out, nil

	case "recent_news":
		news, err := co.buildRecentNewsN(ctx, limit)
		if err != nil {
			return "", fmt.Errorf("devconsole: run recent_news: %w", err)
		}
		out := fmt.Sprintf("%d most recent sector news headlines:\n", len(news))
		for _, headline := range news {
			out += fmt.Sprintf("  - %s\n", headline)
		}
		return out, nil

	case "discovery_rate":
		since := windowSince(days)
		rows, err := co.DB.QueryContext(ctx, `
			SELECT discovery_method, COUNT(*)
			FROM encampment_discoveries
			WHERE discovered_at >= $1
			GROUP BY discovery_method
			ORDER BY COUNT(*) DESC`, since)
		if err != nil {
			return "", fmt.Errorf("devconsole: run discovery_rate: %w", err)
		}
		defer rows.Close()
		var total int
		out := ""
		for rows.Next() {
			var method string
			var count int
			if err := rows.Scan(&method, &count); err == nil {
				out += fmt.Sprintf("  - %s: %d\n", method, count)
				total += count
			}
		}
		return fmt.Sprintf("Discoveries in the last %d day(s): %d total\n%s", days, total, out), nil

	case "travel_delays":
		rows, err := co.DB.QueryContext(ctx, `
			SELECT movement_state, COUNT(*)
			FROM raids
			WHERE state IN ('marching', 'returning') AND movement_state != 'moving'
			GROUP BY movement_state
			ORDER BY COUNT(*) DESC`)
		if err != nil {
			return "", fmt.Errorf("devconsole: run travel_delays: %w", err)
		}
		defer rows.Close()
		out := "Currently-marching columns halted right now, by reason:\n"
		any := false
		for rows.Next() {
			var reason string
			var count int
			if err := rows.Scan(&reason, &count); err == nil {
				out += fmt.Sprintf("  - %s: %d\n", reason, count)
				any = true
			}
		}
		if !any {
			out += "  (none - every marching column is currently moving freely)\n"
		}
		return out, nil

	case "stranded_armies":
		rows, err := co.DB.QueryContext(ctx, `
			SELECT ea.name, r.resolve_time
			FROM raids r
			JOIN encampments ea ON ea.id = r.attacker_id
			WHERE r.movement_state = 'awaiting_reinforcement'
			ORDER BY r.resolve_time ASC
			LIMIT $1`, limit)
		if err != nil {
			return "", fmt.Errorf("devconsole: run stranded_armies: %w", err)
		}
		defer rows.Close()
		var count int
		out := ""
		for rows.Next() {
			var name string
			var resolveTime time.Time
			if err := rows.Scan(&name, &resolveTime); err == nil {
				out += fmt.Sprintf("  - %s, stranded since original ETA %s\n", name, resolveTime.UTC().Format("2006-01-02 15:04"))
				count++
			}
		}
		_ = co.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM raids WHERE movement_state = 'awaiting_reinforcement'").Scan(&count)
		return fmt.Sprintf("Stranded columns awaiting resupply or retreat right now: %d\n%s", count, out), nil

	case "convoy_outcomes":
		since := windowSince(days)
		rows, err := co.DB.QueryContext(ctx, `
			SELECT state, COUNT(*)
			FROM supply_convoys
			WHERE created_at >= $1 AND state != 'marching'
			GROUP BY state
			ORDER BY COUNT(*) DESC`, since)
		if err != nil {
			return "", fmt.Errorf("devconsole: run convoy_outcomes: %w", err)
		}
		defer rows.Close()
		out := ""
		total := 0
		for rows.Next() {
			var state string
			var count int
			if err := rows.Scan(&state, &count); err == nil {
				out += fmt.Sprintf("  - %s: %d\n", state, count)
				total += count
			}
		}
		return fmt.Sprintf("Reinforcement convoys resolved in the last %d day(s): %d total\n%s", days, total, out), nil

	case "field_battle_frequency":
		since := windowSince(days)
		var roadBattles, baseBattles, roadTotal, baseTotal int
		_ = co.DB.QueryRowContext(ctx, "SELECT COUNT(*) FILTER (WHERE outcome = 'battle'), COUNT(*) FROM road_encounters WHERE resolved_at >= $1", since).Scan(&roadBattles, &roadTotal)
		_ = co.DB.QueryRowContext(ctx, "SELECT COUNT(*) FILTER (WHERE outcome = 'battle'), COUNT(*) FROM road_base_encounters WHERE resolved_at >= $1", since).Scan(&baseBattles, &baseTotal)
		return fmt.Sprintf(
			"Road encounters resolved in the last %d day(s): %d total, %d turned into a fight (expedition-vs-expedition).\n"+
				"Road-vs-base encounters resolved: %d total, %d turned into a fight.\n",
			days, roadTotal, roadBattles, baseTotal, baseBattles), nil

	case "ai_growth":
		var count int
		var avgLevel float64
		err := co.DB.QueryRowContext(ctx, "SELECT COUNT(*), COALESCE(AVG(level),0) FROM encampments WHERE is_ai_faction = TRUE").Scan(&count, &avgLevel)
		if err != nil {
			return "", fmt.Errorf("devconsole: run ai_growth: %w", err)
		}
		return fmt.Sprintf("Seeded AI civilizations right now: %d, average level %.1f\n", count, avgLevel), nil

	case "loot_mix":
		since := windowSince(days)
		var scrap, metal, crystal, rations, electricity, hydrogen, neuro, dollars float64
		err := co.DB.QueryRowContext(ctx, `
			SELECT COALESCE(SUM(stolen_scrap),0), COALESCE(SUM(stolen_metal),0), COALESCE(SUM(stolen_crystal),0),
			       COALESCE(SUM(stolen_rations),0), COALESCE(SUM(stolen_electricity),0), COALESCE(SUM(stolen_hydrogen),0),
			       COALESCE(SUM(stolen_neuro_cores),0), COALESCE(SUM(stolen_dollars),0)
			FROM raids WHERE state = 'completed' AND resolve_time >= $1`, since).
			Scan(&scrap, &metal, &crystal, &rations, &electricity, &hydrogen, &neuro, &dollars)
		if err != nil {
			return "", fmt.Errorf("devconsole: run loot_mix: %w", err)
		}
		total := scrap + metal + crystal + rations + electricity + hydrogen + neuro + dollars
		pct := func(v float64) float64 {
			if total <= 0 {
				return 0
			}
			return 100 * v / total
		}
		return fmt.Sprintf(
			"Raid loot captured in the last %d day(s), by resource type:\n"+
				"  - Scrap: %.0f (%.1f%%)\n  - Metal: %.0f (%.1f%%)\n  - Crystal: %.0f (%.1f%%)\n"+
				"  - Rations: %.0f (%.1f%%)\n  - Electricity: %.0f (%.1f%%)\n  - Hydrogen: %.0f (%.1f%%)\n"+
				"  - Neuro Cores: %.0f (%.1f%%)\n  - Dollars: %.0f (%.1f%%)\n",
			days, scrap, pct(scrap), metal, pct(metal), crystal, pct(crystal), rations, pct(rations),
			electricity, pct(electricity), hydrogen, pct(hydrogen), neuro, pct(neuro), dollars, pct(dollars)), nil

	case "crystal_flow":
		since := windowSince(days)
		var heldNow, capturedInWindow float64
		_ = co.DB.QueryRowContext(ctx, "SELECT COALESCE(SUM(crystal),0) FROM resources").Scan(&heldNow)
		_ = co.DB.QueryRowContext(ctx, "SELECT COALESCE(SUM(stolen_crystal),0) FROM raids WHERE state = 'completed' AND resolve_time >= $1", since).Scan(&capturedInWindow)
		return fmt.Sprintf("Crystal currently held across all encampments: %.0f\nCrystal captured via raids in the last %d day(s): %.0f\n", heldNow, days, capturedInWindow), nil

	case "speedup_expenditure":
		since := windowSince(days)
		var count int
		var scrap, dollars, crystal float64
		err := co.DB.QueryRowContext(ctx, `
			SELECT COUNT(*), COALESCE(SUM(scrap_spent),0), COALESCE(SUM(dollars_spent),0), COALESCE(SUM(crystal_spent),0)
			FROM speedup_usage_log WHERE created_at >= $1`, since).Scan(&count, &scrap, &dollars, &crystal)
		if err != nil {
			return "", fmt.Errorf("devconsole: run speedup_expenditure: %w", err)
		}
		return fmt.Sprintf(
			"Emergency speed-ups / early camp-breaks used in the last %d day(s): %d\nTotal spent: %.0f Scrap, %.0f Dollars, %.0f Crystal\n(Logging for this metric started when it shipped - no data before that.)\n",
			days, count, scrap, dollars, crystal), nil

	default:
		// Unreachable in practice — nlquery.go rejects unknown
		// intents before calling RunIntent — but fail closed rather
		// than execute anything if it ever is.
		return "", fmt.Errorf("devconsole: unknown intent %q", intent)
	}
}

// buildTopPlayersN and buildRecentNewsN are limit-parameterized
// variants of BuildSnapshot's fixed-count helpers, added for
// RunIntent's use without changing the weekly-report path's existing
// behavior.
func (co *Console) buildTopPlayersN(ctx context.Context, limit int) ([]TopPlayer, error) {
	query := fmt.Sprintf(`
		SELECT e.name, e.level, %s AS score
		FROM encampments e
		ORDER BY score DESC
		LIMIT $1`, scoring.ScoreExpr)

	rows, err := co.DB.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var top []TopPlayer
	for rows.Next() {
		var tp TopPlayer
		if err := rows.Scan(&tp.Name, &tp.Level, &tp.Score); err != nil {
			return nil, err
		}
		top = append(top, tp)
	}
	return top, rows.Err()
}

func (co *Console) buildRecentNewsN(ctx context.Context, limit int) ([]string, error) {
	rows, err := co.DB.QueryContext(ctx, `SELECT headline FROM world_news ORDER BY logged_at DESC LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var headlines []string
	for rows.Next() {
		var headline string
		if err := rows.Scan(&headline); err != nil {
			return nil, err
		}
		headlines = append(headlines, headline)
	}
	return headlines, rows.Err()
}
