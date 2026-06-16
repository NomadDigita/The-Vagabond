package tick

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/engine/resource"
	"github.com/NomadDigita/The-Vagabond/internal/engine/starvation"
)

// Engine orchestrates asynchronous tick cycles across the database structures.
type Engine struct {
	DB                *sql.DB
	TickInterval      time.Duration
	stopChan          chan struct{}
	resourceProcessor *resource.Processor
	starvationEngine  *starvation.Engine
}

// NewEngine builds a configured instance of the tick processor with injected dependencies.
func NewEngine(db *sql.DB, interval time.Duration) *Engine {
	return &Engine{
		DB:                db,
		TickInterval:      interval,
		stopChan:          make(chan struct{}),
		resourceProcessor: resource.NewProcessor(db),
		starvationEngine:  starvation.NewEngine(db),
	}
}

// Start starts the background tick loop inside a dedicated goroutine.
func (e *Engine) Start() {
	log.Printf("Tick Engine initialized. Run interval: %v", e.TickInterval)
	ticker := time.NewTicker(e.TickInterval)

	go func() {
		for {
			select {
			case <-ticker.C:
				e.ProcessTick()
			case <-e.stopChan:
				ticker.Stop()
				log.Println("Tick Engine background goroutine stopped.")
				return
			}
		}
	}()
}

// Stop safely shuts down the background loop.
func (e *Engine) Stop() {
	close(e.stopChan)
}

// ProcessTick runs the sequentially ordered logic passes.
func (e *Engine) ProcessTick() {
	start := time.Now()
	log.Println("⌛ Processing master game tick pass...")

	ctx := context.Background()

	// Initialize the database transaction
	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("Tick Engine failed to initiate database transaction: %v", err)
		return
	}
	defer tx.Rollback()

	// Pass 1: Run resource production, consumption, and caps processor
	if err := e.resourceProcessor.RunResourcePass(ctx, tx); err != nil {
		log.Printf("Error during Tick Resource Pass execution: %v", err)
		return
	}

	// Pass 2: Run starvation, morale decay, and desertion checks
	if err := e.starvationEngine.RunStarvationPass(ctx, tx); err != nil {
		log.Printf("Error during Tick Starvation Pass execution: %v", err)
		return
	}

	// Pass 3: CONSTRUCTION UPGRADE RESOLUTION PASS
	if err := e.resolveCompletedUpgrades(ctx, tx); err != nil {
		log.Printf("Error during Construction Upgrade Pass execution: %v", err)
		return
	}

	// Pass 4: Clean up expired world events
	deleteExpiredEvents := `
		DELETE FROM world_events 
		WHERE expires_at < CURRENT_TIMESTAMP`
	_, err = tx.ExecContext(ctx, deleteExpiredEvents)
	if err != nil {
		log.Printf("Tick Engine failed cleaning expired world events: %v", err)
		return
	}

	// Commit calculations to database
	if err := tx.Commit(); err != nil {
		log.Printf("Tick Engine failed to commit transaction updates: %v", err)
		return
	}

	log.Printf("Tick pass successfully calculated and committed. Duration: %s", time.Since(start))
}

func (e *Engine) resolveCompletedUpgrades(ctx context.Context, tx *sql.Tx) error {
	// 1. Scan for modules where upgrade timers have expired
	query := `
		SELECT m.id, e.user_id, e.name, m.type, m.level
		FROM modules m
		JOIN encampments e ON e.id = m.encampment_id
		WHERE m.is_upgrading = TRUE AND m.upgrade_ready_at <= CURRENT_TIMESTAMP`

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed selecting completed upgrades: %w", err)
	}
	defer rows.Close()

	type completedUpgrade struct {
		id       string
		userID   int64
		campName string
		modType  string
		oldLvl   int
	}

	var completed []completedUpgrade
	for rows.Next() {
		var c completedUpgrade
		if err := rows.Scan(&c.id, &c.userID, &c.campName, &c.modType, &c.oldLvl); err == nil {
			completed = append(completed, c)
		}
	}
	rows.Close()

	// 2. Increment levels and push notifications to completed players
	for _, c := range completed {
		newLvl := c.oldLvl + 1
		updateQuery := `
			UPDATE modules 
			SET level = $1, is_upgrading = FALSE, upgrade_ready_at = NULL 
			WHERE id = $2`
		_, err := tx.ExecContext(ctx, updateQuery, newLvl, c.id)
		if err != nil {
			log.Printf("Failed executing module upgrade writeback: %v", err)
			continue
		}

		// Queue success push notification alert
		alertMsg := fmt.Sprintf(
			"🏗️ CONSTRUCTION COMPLETE\n\n"+
				"Outpost: %s\n"+
				"Upgrade completed successfully!\n"+
				"Your [%s] has successfully reached Level %d.",
			c.campName, c.modType, newLvl,
		)

		insertNotification := `
			INSERT INTO notifications (user_id, message, is_sent) 
			VALUES ($1, $2, FALSE)`
		_, err = tx.ExecContext(ctx, insertNotification, c.userID, alertMsg)
		if err != nil {
			log.Printf("Failed queuing upgrade completed push alert: %v", err)
		}
		log.Printf("Camp module upgrade resolved: %s upgraded %s to level %d.", c.campName, c.modType, newLvl)
	}

	return nil
}
