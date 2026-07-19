package devconsole

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
	"github.com/NomadDigita/The-Vagabond/internal/game/scoring"
)

// MemoryScope namespaces this feature's conversational history inside
// ai_memory, distinct from every other Phase B+ scope.
const MemoryScope = "dev_console"

// newPlayerListCap bounds how many individual new-player rows are
// listed in the prompt (Snapshot.NewPlayerCount still carries the true
// total even if the list itself is capped) — a signup surge shouldn't
// blow up the prompt size or the token bill for a report that's meant
// to be a summary, not a full export.
const newPlayerListCap = 30

// topPlayerCount mirrors the "Top Skilled"/ranking panel's usual
// depth for a quick read, not the full 15 HandleRankingPanel shows —
// this report is a summary, not a leaderboard reprint.
const topPlayerCount = 5

// recentNewsCap mirrors the same 5-headline depth
// HandleWorldFeed/galaxyadvisor already use elsewhere, filtered to the
// reporting window instead of an unconditional "last 5 ever".
const recentNewsCap = 10

// Console is the Phase J entry point. Unlike every other Phase B-I
// package, this one has no per-player ErrNoEncampment-style guard —
// it reports on the whole game, not one player's own state. Admin
// gating happens in the handler layer (matching
// internal/bot/handlers/admin.go's existing IsAdmin pattern), not
// here — this package doesn't know about Telegram admin IDs at all.
type Console struct {
	DB *sql.DB
	AI *ai.Service
}

func New(db *sql.DB, service *ai.Service) *Console {
	return &Console{DB: db, AI: service}
}

// windowSince returns the cutoff time for a "last N days" window,
// defaulting to 7 if windowDays isn't positive. Shared by
// BuildSnapshot and RunIntent so "last N days" always means the same
// thing everywhere in this package.
func windowSince(windowDays int) time.Time {
	if windowDays <= 0 {
		windowDays = 7
	}
	return time.Now().UTC().AddDate(0, 0, -windowDays)
}

// BuildSnapshot loads new-player signups, top players, active-user
// count, and recent world news for the last windowDays days.
func (co *Console) BuildSnapshot(ctx context.Context, windowDays int) (*Snapshot, error) {
	if windowDays <= 0 {
		windowDays = 7
	}
	since := windowSince(windowDays)

	s := Snapshot{WindowDays: windowDays}

	if err := co.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&s.TotalUsersAllTime); err != nil {
		return nil, fmt.Errorf("devconsole: count total users: %w", err)
	}

	if err := co.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE last_active >= $1`, since).Scan(&s.ActiveUserCount); err != nil {
		return nil, fmt.Errorf("devconsole: count active users: %w", err)
	}

	if err := co.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE registered_at >= $1`, since).Scan(&s.NewPlayerCount); err != nil {
		return nil, fmt.Errorf("devconsole: count new players: %w", err)
	}

	newPlayers, err := co.buildNewPlayers(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("devconsole: load new players: %w", err)
	}
	s.NewPlayers = newPlayers

	topPlayers, err := co.buildTopPlayers(ctx)
	if err != nil {
		return nil, fmt.Errorf("devconsole: load top players: %w", err)
	}
	s.TopPlayers = topPlayers

	news, err := co.buildRecentNews(ctx, since)
	if err != nil {
		return nil, fmt.Errorf("devconsole: load recent news: %w", err)
	}
	s.RecentWorldNews = news

	return &s, nil
}

// buildNewPlayers reads the newPlayerListCap most recently registered
// users in the window, LEFT JOINed to their encampment (if any) so a
// player who registered but never finished onboarding still shows up
// — with an honest "no outpost yet" home continent rather than being
// silently dropped or guessed at.
func (co *Console) buildNewPlayers(ctx context.Context, since time.Time) ([]NewPlayer, error) {
	rows, err := co.DB.QueryContext(ctx, `
		SELECT u.username, u.first_name, u.registered_at, COALESCE(c.region, '')
		FROM users u
		LEFT JOIN encampments e ON e.user_id = u.telegram_id
		LEFT JOIN coordinates c ON c.id = e.coordinate_id
		WHERE u.registered_at >= $1
		ORDER BY u.registered_at DESC
		LIMIT $2`, since, newPlayerListCap)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var players []NewPlayer
	for rows.Next() {
		var p NewPlayer
		var joinedAt time.Time
		if err := rows.Scan(&p.Username, &p.FirstName, &joinedAt, &p.HomeContinent); err != nil {
			return nil, err
		}
		p.JoinedAt = joinedAt.UTC().Format("2006-01-02 15:04 UTC")
		players = append(players, p)
	}
	return players, rows.Err()
}

// buildTopPlayers reuses the exact same scoring.ScoreExpr formula the
// player-facing Global Ranking panel already uses, so this report's
// "top players" never disagrees with what players themselves see on
// /ranking.
func (co *Console) buildTopPlayers(ctx context.Context) ([]TopPlayer, error) {
	query := fmt.Sprintf(`
		SELECT e.name, e.level, %s AS score
		FROM encampments e
		ORDER BY score DESC
		LIMIT $1`, scoring.ScoreExpr)

	rows, err := co.DB.QueryContext(ctx, query, topPlayerCount)
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

func (co *Console) buildRecentNews(ctx context.Context, since time.Time) ([]string, error) {
	rows, err := co.DB.QueryContext(ctx, `
		SELECT headline FROM world_news
		WHERE logged_at >= $1
		ORDER BY logged_at DESC
		LIMIT $2`, since, recentNewsCap)
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

// Recommend produces a fresh AI activity-report summary for the last
// windowDays days. callerUserID is the requesting admin's Telegram ID,
// used only for ai_memory attribution/cost tracking the same way
// every other Phase B-I package attributes its calls — this method
// itself does not check whether callerUserID is actually an admin
// (see Console's doc comment for why that's the handler's job).
//
// Read-only: nothing in this method changes any player, setting, or
// game data — it only reads and reports.
func (co *Console) Recommend(ctx context.Context, callerUserID int64, windowDays int) (*Recommendation, error) {
	snapshot, err := co.BuildSnapshot(ctx, windowDays)
	if err != nil {
		return nil, err
	}

	userPrompt := BuildUserPrompt(*snapshot)

	if co.AI.Memory != nil {
		_ = co.AI.Memory.Append(ctx, callerUserID, MemoryScope, ai.Message{Role: ai.RoleUser, Content: userPrompt})
	}

	resp, err := co.AI.Complete(ctx, ai.CompletionRequest{
		Feature:     string(ai.FeatureDevConsole),
		UserID:      callerUserID,
		System:      SystemPrompt,
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: userPrompt}},
		MaxTokens:   2048,
		Temperature: 0.3,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("devconsole: ai completion failed: %w", err)
	}

	rec := ParseRecommendation(resp.Text)

	if co.AI.Memory != nil {
		_ = co.AI.Memory.Append(ctx, callerUserID, MemoryScope, ai.Message{Role: ai.RoleAssistant, Content: resp.Text})
	}

	return rec, nil
}
