package tick

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// Engine orchestrates asynchronous tick cycles across the database structures.
type Engine struct {
	DB           *sql.DB
	TickInterval time.Duration
	stopChan     chan struct{}
}

// NewEngine builds a configured instance of the tick processor.
func NewEngine(db *sql.DB, interval time.Duration) *Engine {
	return &Engine{
		DB:           db,
		TickInterval: interval,
		stopChan:     make(chan struct{}),
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
	log.Println("⌛ Processing game tick pass...")

	ctx := context.Background()

	// Execute resource calculations inside an atomic transaction
	tx, err := e.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("Tick Engine failed to initiate database transaction: %v", err)
		return
	}
	defer tx.Rollback()

	// 1. RESOURCE PASS: Generate passive hourly rates converted to tick increments
	// Base Tick Rates (Assuming 60s ticks):
	// Scrap: +0.25, Rations: +0.10, Energy: +0.05
	queryResources := `
		SELECT r.encampment_id, r.scrap, r.rations, r.energy, r.neuro_cores 
		FROM resources r`

	rows, err := tx.QueryContext(ctx, queryResources)
	if err != nil {
		log.Printf("Tick Engine Resource Pass select query failed: %v", err)
		return
	}
	defer rows.Close()

	type resourceUpdate struct {
		encampmentID string
		scrap        float64
		rations      float64
		energy       float64
	}

	var updates []resourceUpdate

	for rows.Next() {
		var u resourceUpdate
		var neuro float64 // Placeholder reading
		err := rows.Scan(&u.encampmentID, &u.scrap, &u.rations, &u.energy, &neuro)
		if err != nil {
			log.Printf("Failed scanning resource row: %v", err)
			continue
		}

		// Calculate passive generation rates
		// This can eventually scale relative to facility module levels
		u.scrap += 0.25
		u.rations += 0.10
		u.energy += 0.05

		updates = append(updates, u)
	}

	// Batch update the modified calculations back to the database
	updateStmt := `
		UPDATE resources 
		SET scrap = $1, rations = $2, energy = $3, last_ticked_at = CURRENT_TIMESTAMP 
		WHERE encampment_id = $4`

	for _, update := range updates {
		_, err := tx.ExecContext(ctx, updateStmt, update.scrap, update.rations, update.energy, update.encampmentID)
		if err != nil {
			log.Printf("Failed writing batch resource tick updates for %s: %v", update.encampmentID, err)
			return
		}
	}

	// 2. WORLD EVENT PASS: Clean up expired events and run updates
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
