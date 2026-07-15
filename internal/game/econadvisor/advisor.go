package econadvisor

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/ai"
)

// MemoryScope namespaces this feature's conversational history inside
// ai_memory, distinct from every other Phase B+ scope.
const MemoryScope = "economy_advisor"

// ErrNoEncampment is returned when the calling player has no
// registered base yet.
var ErrNoEncampment = errors.New("econadvisor: player has no encampment")

// resourceColumns lists every resources-table column this package
// knows about, explicitly, so a schema change is a deliberate one-line
// addition rather than a silent behavior change (same pattern as
// fleetcommander.unitColumns).
var resourceColumns = []string{
	"scrap", "rations", "electricity", "neuro_cores", "metal", "crystal", "hydrogen", "dollars", "ether",
}

// Advisor is the Phase D entry point.
type Advisor struct {
	DB *sql.DB
	AI *ai.Service
}

func New(db *sql.DB, service *ai.Service) *Advisor {
	return &Advisor{DB: db, AI: service}
}

// BuildSnapshot loads the current economic state of a player's base.
func (a *Advisor) BuildSnapshot(ctx context.Context, userID int64) (*Snapshot, error) {
	var s Snapshot
	if err := a.DB.QueryRowContext(ctx, `SELECT id, name, level FROM encampments WHERE user_id = $1`, userID).
		Scan(&s.EncampmentID, &s.Name, &s.Level); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNoEncampment
		}
		return nil, fmt.Errorf("econadvisor: load encampment: %w", err)
	}

	resQuery := fmt.Sprintf(`SELECT %s FROM resources WHERE encampment_id = $1`, joinColumns(resourceColumns))
	dest := make([]any, len(resourceColumns))
	vals := make([]float64, len(resourceColumns))
	for i := range vals {
		dest[i] = &vals[i]
	}
	if err := a.DB.QueryRowContext(ctx, resQuery, s.EncampmentID).Scan(dest...); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("econadvisor: load resources: %w", err)
	}
	s.Resources = make(map[string]float64, len(resourceColumns))
	for i, name := range resourceColumns {
		s.Resources[name] = vals[i]
	}

	modRows, err := a.DB.QueryContext(ctx, `SELECT type, level FROM modules WHERE encampment_id = $1`, s.EncampmentID)
	if err != nil {
		return nil, fmt.Errorf("econadvisor: load modules: %w", err)
	}
	defer modRows.Close()
	for modRows.Next() {
		var m ModuleState
		if err := modRows.Scan(&m.Type, &m.Level); err != nil {
			return nil, fmt.Errorf("econadvisor: scan module row: %w", err)
		}
		s.Modules = append(s.Modules, m)
	}
	if err := modRows.Err(); err != nil {
		return nil, err
	}

	// bank_accounts may not have a row yet for a brand-new player —
	// that's a valid all-zero state, not an error.
	err = a.DB.QueryRowContext(ctx, `
		SELECT balance, balance_cash, loan_amount, loan_cash
		FROM bank_accounts WHERE encampment_id = $1`, s.EncampmentID,
	).Scan(&s.BankBalance, &s.BankBalanceCash, &s.LoanAmount, &s.LoanCash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("econadvisor: load bank account: %w", err)
	}

	listingRows, err := a.DB.QueryContext(ctx, `
		SELECT item_type, quantity, price_dollars
		FROM market_exchange WHERE seller_id = $1 AND is_sold = FALSE`, s.EncampmentID)
	if err != nil {
		return nil, fmt.Errorf("econadvisor: load own listings: %w", err)
	}
	defer listingRows.Close()
	for listingRows.Next() {
		var l MarketListing
		if err := listingRows.Scan(&l.ItemType, &l.Quantity, &l.PriceDollars); err != nil {
			return nil, fmt.Errorf("econadvisor: scan listing row: %w", err)
		}
		s.OwnListings = append(s.OwnListings, l)
	}
	if err := listingRows.Err(); err != nil {
		return nil, err
	}

	statsRows, err := a.DB.QueryContext(ctx, `
		SELECT item_type, COUNT(*), AVG(price_dollars), MIN(price_dollars), MAX(price_dollars)
		FROM market_exchange
		WHERE is_sold = FALSE
		GROUP BY item_type`)
	if err != nil {
		return nil, fmt.Errorf("econadvisor: load market stats: %w", err)
	}
	defer statsRows.Close()
	for statsRows.Next() {
		var m MarketItemStats
		if err := statsRows.Scan(&m.ItemType, &m.ActiveCount, &m.AveragePrice, &m.MinPrice, &m.MaxPrice); err != nil {
			return nil, fmt.Errorf("econadvisor: scan market stats row: %w", err)
		}
		s.MarketStats = append(s.MarketStats, m)
	}
	if err := statsRows.Err(); err != nil {
		return nil, err
	}

	return &s, nil
}

func joinColumns(cols []string) string {
	out := ""
	for i, c := range cols {
		if i > 0 {
			out += ", "
		}
		out += c
	}
	return out
}

// Recommend produces a fresh AI recommendation for the given player's
// economy. It stores both turns in ai_memory under MemoryScope.
//
// Advisory-only: nothing in this method buys, sells, upgrades, or
// otherwise mutates any game table.
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
		Feature:     string(ai.FeatureEconomyAdvisor),
		UserID:      userID,
		System:      SystemPrompt,
		Messages:    []ai.Message{{Role: ai.RoleUser, Content: userPrompt}},
		MaxTokens:   1024,
		Temperature: 0.3,
		JSONMode:    true,
	})
	if err != nil {
		return nil, fmt.Errorf("econadvisor: ai completion failed: %w", err)
	}

	rec := ParseRecommendation(resp.Text)

	if a.AI.Memory != nil {
		_ = a.AI.Memory.Append(ctx, userID, MemoryScope, ai.Message{Role: ai.RoleAssistant, Content: resp.Text})
	}

	return rec, nil
}
