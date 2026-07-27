package notifications

import (
	"context"
	"database/sql"
	"log"
	"strings"
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

// looksLikeHTML reports whether a queued message was deliberately built
// with Telegram HTML tags (via the render helpers in battlereport.go /
// bot/handlers/render.go) as opposed to being a plain-text string that
// simply happens to contain some other "<" character. Checking for a
// real closing tag rather than a bare "<" keeps this from false-
// positiving on stray angle brackets in old plain messages.
func looksLikeHTML(msg string) bool {
	for _, tag := range []string{"</b>", "</i>", "</u>", "</code>", "</blockquote>", "tg-spoiler"} {
		if strings.Contains(msg, tag) {
			return true
		}
	}
	return false
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

	// Deduplication (Phase 7 milestone 2): a burst of the same event -
	// e.g. several near-simultaneous route-status pings - can queue the
	// identical (user_id, message) pair more than once before a player
	// has had a chance to read any of them. Rather than retrofitting
	// every one of this codebase's many INSERT INTO notifications call
	// sites to check for an existing duplicate before queuing, coalesce
	// here: send the message once per (user, text) pair still pending in
	// this batch, and mark every duplicate row sent alongside it. This
	// only merges rows that are byte-identical, so it can never conflate
	// two different alerts that happen to arrive close together.
	type dedupKey struct {
		userID  int64
		message string
	}
	groups := make(map[dedupKey][]string) // -> notification ids
	order := make([]dedupKey, 0, len(queue))
	for _, p := range queue {
		k := dedupKey{p.userID, p.message}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], p.id)
	}

	for _, k := range order {
		ids := groups[k]
		targetUser := &telebot.User{ID: k.userID}

		// Auto-detect HTML-formatted messages rather than assuming every
		// queued notification across the whole codebase is HTML-safe.
		// Only ~a handful of call sites (starting with battle reports)
		// have been audited and have their dynamic text escaped so far;
		// the rest still build plain strings the old way. Sending THOSE
		// with ModeHTML forced on would risk a "can't parse entities"
		// 400 error the moment a player-set name contains a raw &/</>.
		// A message that was actually built with the render helpers will
		// contain one of these tags; anything else is sent exactly as
		// before (plain text, zero behavior change).
		opts := []interface{}{}
		if looksLikeHTML(k.message) {
			opts = append(opts, telebot.ModeHTML)
		}
		_, err := d.Bot.Send(targetUser, k.message, opts...)
		if err != nil {
			log.Printf("Dispatcher failed to deliver notification to %d: %v", k.userID, err)
			continue
		}

		// Mark every duplicate in this batch as sent together, not just
		// the one that was actually delivered.
		for _, id := range ids {
			if _, err := d.DB.ExecContext(ctx, "UPDATE notifications SET is_sent = TRUE WHERE id = $1", id); err != nil {
				log.Printf("Dispatcher failed updating persistence sentinel state for notification %s: %v", id, err)
			}
		}
	}
}
