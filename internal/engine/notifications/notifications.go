package notifications

import (
	"context"
	"database/sql"
	"log"
	"time"

	"gopkg.in/telebot.v3"
)

type Dispatcher struct {
	DB       *sql.DB
	Bot      *telebot.Bot
	stopChan chan struct{}
}

func NewDispatcher(db *sql.DB, bot *telebot.Bot) *Dispatcher {
	return &Dispatcher{
		DB:       db,
		Bot:      bot,
		stopChan: make(chan struct{}),
	}
}

// Start boots the continuous poller background goroutine
func (d *Dispatcher) Start() {
	log.Println("Notification Dispatcher system online. Draining queues...")
	ticker := time.NewTicker(3 * time.Second) // Polls every 3s

	go func() {
		for {
			select {
			case <-ticker.C:
				d.drainQueue()
			case <-d.stopChan:
				ticker.Stop()
				log.Println("Notification Dispatcher stopped.")
				return
			}
		}
	}()
}

// Stop closes the background drain loops
func (d *Dispatcher) Stop() {
	close(d.stopChan)
}

func (d *Dispatcher) drainQueue() {
	ctx := context.Background()

	// Select unsent notifications
	query := `
		SELECT id, user_id, message 
		FROM notifications 
		WHERE is_sent = FALSE 
		ORDER BY queued_at ASC 
		LIMIT 10`

	rows, err := d.DB.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Dispatcher failed querying pending notifications: %v", err)
		return
	}
	defer rows.Close()

	type pending struct {
		id      string
		userID  int64
		message string
	}

	var queue []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.userID, &p.message); err == nil {
			queue = append(queue, p)
		}
	}
	rows.Close()

	for _, p := range queue {
		// Target Telegram user structure
		targetUser := &telebot.User{ID: p.userID}

		// Dispatch message through Telegram API
		_, err := d.Bot.Send(targetUser, p.message)
		if err != nil {
			log.Printf("Dispatcher failed to deliver notification to %d: %v", p.userID, err)
			continue
		}

		// Mark as sent in database
		_, err = d.DB.ExecContext(ctx, "UPDATE notifications SET is_sent = TRUE WHERE id = $1", p.id)
		if err != nil {
			log.Printf("Dispatcher failed updating persistence sentinel state for notification %s: %v", p.id, err)
		}
	}
}
