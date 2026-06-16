package realtime

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/lib/pq"
	"gopkg.in/telebot.v3"
)

type Listener struct {
	DBURL      string
	DB         *sql.DB
	Bot        *telebot.Bot
	pqListener *pq.Listener
	stopChan   chan struct{}
}

func NewListener(dbURL string, db *sql.DB, bot *telebot.Bot) *Listener {
	return &Listener{
		DBURL:    dbURL,
		DB:       db,
		Bot:      bot,
		stopChan: make(chan struct{}),
	}
}

// Start opens the database listener loop
func (l *Listener) Start() {
	// Report issues if listener connection drops
	reportEvent := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			log.Printf("Realtime Listener Connection State Change Warning: %v", err)
		}
	}

	// Initialize the connection
	l.pqListener = pq.NewListener(l.DBURL, 10*time.Second, time.Minute, reportEvent)
	err := l.pqListener.Listen("realtime_notification_event")
	if err != nil {
		log.Fatalf("Fatal: Failed to open realtime channels: %v", err)
	}

	log.Println("Realtime PostgreSQL Event Listener subscribed to channel: [realtime_notification_event]")

	go func() {
		for {
			select {
			case notification, ok := <-l.pqListener.Notify:
				if !ok {
					log.Println("Realtime notification channel closed.")
					return
				}
				if notification != nil {
					l.dispatchNotification(notification.Extra)
				}
			case <-l.stopChan:
				_ = l.pqListener.UnlistenAll()
				_ = l.pqListener.Close()
				return
			}
		}
	}()
}

// Stop shuts down the background listener safely
func (l *Listener) Stop() {
	close(l.stopChan)
}

func (l *Listener) dispatchNotification(notificationID string) {
	ctx := context.Background()

	// Query message contents and mark as sent in a single transaction
	tx, err := l.DB.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("Realtime dispatcher failed initiating transaction: %v", err)
		return
	}
	defer tx.Rollback()

	var userID int64
	var message string
	var isSent bool

	query := `
		SELECT user_id, message, is_sent 
		FROM notifications 
		WHERE id = $1 FOR UPDATE`

	err = tx.QueryRowContext(ctx, query, notificationID).Scan(&userID, &message, &isSent)
	if err != nil {
		log.Printf("Realtime dispatcher failed scanning notification %s: %v", notificationID, err)
		return
	}

	if isSent {
		// Already handled by poll backup
		return
	}

	// Dispatch message immediately
	targetUser := &telebot.User{ID: userID}
	_, err = l.Bot.Send(targetUser, message)
	if err != nil {
		log.Printf("Realtime dispatcher failed delivering message: %v", err)
		return
	}

	// Update sentinel status to true
	_, err = tx.ExecContext(ctx, "UPDATE notifications SET is_sent = TRUE WHERE id = $1", notificationID)
	if err != nil {
		log.Printf("Realtime dispatcher failed executing DB writeback: %v", err)
		return
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Realtime dispatcher failed committing transaction: %v", err)
	}
}
