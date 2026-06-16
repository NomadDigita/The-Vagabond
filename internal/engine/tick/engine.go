package tick

import (
	"context"
	"database/sql"
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

	// Pass 3: Clean up expired world events
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
