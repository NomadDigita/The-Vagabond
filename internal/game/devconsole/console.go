package devconsole

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
	"github.com/NomadDigita/The-Vagabond/internal/game/scoring"
)

const MemoryScope = "dev_console"

const newPlayerListCap = 30

const topPlayerCount = 5

const recentNewsCap = 10

type Console struct {
	DB *sql.DB
	AI *ai.Service
}

func New(db *sql.DB, service *ai.Service) *Console {
	return &Console{DB: db, AI: service}
}

func windowSince(windowDays int) time.Time {
	if windowDays <= 0 {
		windowDays = 7
	}
	return time.Now().UTC().AddDate(0, 0, -windowDays)
}

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
		MaxTokens:   4096,
		Temperature: 0.3,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("devconsole: ai completion failed: %w", err)
	}

	rec := ParseRecommendation(resp.Text, resp.StopReason)
	rec.Snapshot = snapshot

	if co.AI.Memory != nil {
		_ = co.AI.Memory.Append(ctx, callerUserID, MemoryScope, ai.Message{Role: ai.RoleAssistant, Content: resp.Text})
	}

	return rec, nil
}
