package handlers

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/NomadDigita/The-Vagabond/internal/db/schema"
	"github.com/NomadDigita/The-Vagabond/internal/game/scoring"
	_ "github.com/lib/pq"
)

// TestDisplayName_PrefixesAIFactions verifies the leaderboard's honest-
// labeling rule from AI_PARITY_AND_WORLD_NOTIFICATIONS_PLAN.md section
// 1.2: AI factions get a 🤖 prefix, humans don't, and both are HTML-escaped.
func TestDisplayName_PrefixesAIFactions(t *testing.T) {
	if got := displayName("Rogue Drone Nest", true); got != "🤖 Rogue Drone Nest" {
		t.Errorf("expected 🤖-prefixed name for an AI faction, got %q", got)
	}
	if got := displayName("Player One", false); got != "Player One" {
		t.Errorf("expected an unprefixed name for a human, got %q", got)
	}
	if got := displayName("<script>", true); got != "🤖 &lt;script&gt;" {
		t.Errorf("expected the name to still be HTML-escaped, got %q", got)
	}
}

func rankingTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("SCHEMA_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("SCHEMA_TEST_DATABASE_URL not set; skipping real-database ranking test")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("opening test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	for _, stmt := range schema.Statements() {
		if _, err := db.Exec(stmt); err != nil {
			t.Fatalf("applying schema: %v\n%s", err, stmt)
		}
	}
	if _, err := db.Exec(`TRUNCATE encampments, coordinates, users RESTART IDENTITY CASCADE`); err != nil {
		t.Fatalf("truncating tables: %v", err)
	}
	return db
}

func seedRankingEncampment(t *testing.T, db *sql.DB, telegramID int64, name string, isAIFaction bool, level int) {
	t.Helper()
	if _, err := db.Exec("INSERT INTO users (telegram_id) VALUES ($1) ON CONFLICT DO NOTHING", telegramID); err != nil {
		t.Fatalf("seeding user: %v", err)
	}
	var coordID string
	err := db.QueryRow(`
		INSERT INTO coordinates (x, y, biome, region, terrain) VALUES ($1, $1, 'plains', 'TestRegion', 'flat')
		RETURNING id`, telegramID).Scan(&coordID)
	if err != nil {
		t.Fatalf("seeding coordinate: %v", err)
	}
	_, err = db.Exec(`
		INSERT INTO encampments (user_id, name, level, coordinate_id, is_ai_faction)
		VALUES ($1, $2, $3, $4, $5)`, telegramID, name, level, coordID, isAIFaction)
	if err != nil {
		t.Fatalf("seeding encampment: %v", err)
	}
}

// TestTopPlayersQuery_IncludesAIFactionsRankedByScore proves the leaderboard
// fix: an AI faction with a higher score than a human now outranks that
// human in the "top players" query, instead of being filtered out.
func TestTopPlayersQuery_IncludesAIFactionsRankedByScore(t *testing.T) {
	db := rankingTestDB(t)

	seedRankingEncampment(t, db, 3001, "Human Camp", false, 5)
	seedRankingEncampment(t, db, 3002, "Rogue Drone Nest", true, 50)

	query := fmt.Sprintf(`
		SELECT e.name, e.is_ai_faction, %s AS score
		FROM encampments e
		ORDER BY score DESC
		LIMIT 15`, scoring.ScoreExpr)

	rows, err := db.Query(query)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	var names []string
	var aiFlags []bool
	for rows.Next() {
		var name string
		var isAI bool
		var score float64
		if err := rows.Scan(&name, &isAI, &score); err != nil {
			t.Fatalf("scan: %v", err)
		}
		names = append(names, name)
		aiFlags = append(aiFlags, isAI)
	}

	if len(names) != 2 {
		t.Fatalf("expected both a human and an AI faction in the results, got %v", names)
	}
	if names[0] != "Rogue Drone Nest" || !aiFlags[0] {
		t.Errorf("expected the higher-level AI faction to rank first, got %v", names)
	}
}
