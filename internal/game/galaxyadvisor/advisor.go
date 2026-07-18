package galaxyadvisor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
	"github.com/NomadDigita/The-Vagabond/internal/engine/world"
)

// MemoryScope namespaces this feature's conversational history inside
// ai_memory, distinct from every other Phase B+ scope.
const MemoryScope = "galaxy_advisor"

// recentNewsLimit mirrors the 5-headline limit
// internal/bot/handlers/world.go's HandleWorldFeed already uses for
// its own news panel, so this advisor sees exactly what the player
// would see on that screen — not a different, inconsistent slice of
// history.
const recentNewsLimit = 5

// ErrNoEncampment is returned when the calling player has no
// registered base yet (and therefore no continent to advise on).
var ErrNoEncampment = errors.New("galaxyadvisor: player has no encampment")

// Advisor is the Phase H entry point.
type Advisor struct {
	DB *sql.DB
	AI *ai.Service
}

func New(db *sql.DB, service *ai.Service) *Advisor {
	return &Advisor{DB: db, AI: service}
}

// BuildSnapshot loads the calling player's home continent, every
// continent's current world event (via the shared
// internal/engine/world helpers the tick engine and world-feed panel
// already use), and the most recent sector news headlines.
func (a *Advisor) BuildSnapshot(ctx context.Context, userID int64) (*Snapshot, error) {
	var s Snapshot
	if err := a.DB.QueryRowContext(ctx, `
		SELECT c.region
		FROM encampments e
		JOIN coordinates c ON c.id = e.coordinate_id
		WHERE e.user_id = $1`, userID).Scan(&s.HomeContinent); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoEncampment
		}
		return nil, fmt.Errorf("galaxyadvisor: load home continent: %w", err)
	}

	activeByContinent := world.ActiveEventsByContinent(ctx, a.DB)
	for _, continent := range world.Continents {
		eventType := activeByContinent[continent]
		if eventType == "" {
			eventType = "nominal"
		}
		s.Continents = append(s.Continents, ContinentStatus{Continent: continent, EventType: eventType})
	}

	news, err := a.buildRecentNews(ctx)
	if err != nil {
		return nil, fmt.Errorf("galaxyadvisor: load recent news: %w", err)
	}
	s.RecentNews = news

	return &s, nil
}

func (a *Advisor) buildRecentNews(ctx context.Context) ([]string, error) {
	rows, err := a.DB.QueryContext(ctx, `
		SELECT headline FROM world_news ORDER BY logged_at DESC LIMIT $1`, recentNewsLimit)
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

// Recommend produces a fresh AI briefing for the given player. It
// stores both turns in ai_memory under MemoryScope.
//
// Read-only: nothing in this method changes any world event, marches
// any fleet, or queues any construction — it only reads and advises.
func (a *Advisor) Recommend(ctx context.Context, userID int64) (*Recommendation, error) {
	snapshot, err := a.BuildSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}

	userPrompt := BuildUserPrompt(*snapshot)

	if a.AI.Memory != nil {
		_ = a.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleUser, Content: userPrompt})
	}

	resp, err := a.AI.Complete(ctx, ai.CompletionRequest{
		Feature:     string(ai.FeatureDynamicGalaxy),
		UserID:      userID,
		System:      SystemPrompt,
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: userPrompt}},
		MaxTokens:   2048,
		Temperature: 0.3,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("galaxyadvisor: ai completion failed: %w", err)
	}

	rec := ParseRecommendation(resp.Text)

	if a.AI.Memory != nil {
		_ = a.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleAssistant, Content: resp.Text})
	}

	return rec, nil
}
