package devconsole

import (
	"context"
	"fmt"

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
}

// IntentDescriptions renders the whitelist for the classification
// prompt (see nlquery.go's SystemPrompt) — keeping the human-readable
// description next to the map above means they can't drift apart.
func IntentDescriptions() string {
	var out string
	for _, name := range []string{"new_players", "top_players", "active_users", "totals", "economy_snapshot", "combat_stats", "clan_stats", "world_state", "recent_news"} {
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
