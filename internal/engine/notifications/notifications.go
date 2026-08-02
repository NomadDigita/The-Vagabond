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

// maxNotificationAttempts is how many times drainQueue retries a
// notification before giving up on it - see notifications' schema
// comment on failed_attempts for why this exists: without a cap, one
// permanently-undeliverable message can occupy a slot in every future
// LIMIT-10 batch forever, starving every genuinely new notification
// behind it for every player, not just the one who can't receive it.
const maxNotificationAttempts = 5

// recordFailedAttempt increments a notification's failed_attempts and
// reports whether the caller should give up on it (in which case it's
// marked is_sent = TRUE so it stops occupying a slot in every future
// drainQueue batch) rather than retry it again. Pulled out as its own
// function specifically so this has a unit-testable seam independent of
// needing a real, network-connected *telebot.Bot to exercise drainQueue's
// send-failure path end-to-end.
func recordFailedAttempt(ctx context.Context, db querier, id string) (gaveUp bool, err error) {
	var attempts int
	if err := db.QueryRowContext(ctx, "UPDATE notifications SET failed_attempts = failed_attempts + 1 WHERE id = $1 RETURNING failed_attempts", id).Scan(&attempts); err != nil {
		return false, err
	}
	if attempts < maxNotificationAttempts {
		return false, nil
	}
	if _, err := db.ExecContext(ctx, "UPDATE notifications SET is_sent = TRUE WHERE id = $1", id); err != nil {
		return false, err
	}
	return true, nil
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

	// AI factions have synthetic negative user_id rows (see isRealPlayer
	// in internal/engine/tick/engine.go and cmd/bot/main.go's AI
	// civilization seeding) with no Telegram session behind them at all.
	// Most of this codebase's ~50 "INSERT INTO notifications" call sites
	// (construction-complete, combat reports, etc.) don't check
	// isRealPlayer before queuing - confirmed live via Render logs
	// (2026-08-02): "Dispatcher giving up on notification ... to -908002
	// after 5 failed attempts" for a routine construction-complete alert.
	// Rather than retrofitting every insert site (same reasoning as the
	// dedup step below), sweep any that already got queued before ever
	// attempting delivery, so they don't burn 5 guaranteed-failed
	// Telegram calls apiece or sit in the table forever.
	if _, err := d.DB.ExecContext(ctx, "UPDATE notifications SET is_sent = TRUE WHERE is_sent = FALSE AND user_id <= 0"); err != nil {
		log.Printf("Dispatcher failed sweeping non-player (AI faction) notifications: %v", err)
	}

	// Select unsent notifications
	query := `
		SELECT id, user_id, message, failed_attempts 
		FROM notifications 
		WHERE is_sent = FALSE AND user_id > 0
		ORDER BY queued_at ASC 
		LIMIT 10`

	rows, err := d.DB.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Dispatcher failed querying pending notifications: %v", err)
		return
	}
	defer rows.Close()

	type pending struct {
		id             string
		userID         int64
		message        string
		failedAttempts int
	}

	var queue []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.userID, &p.message, &p.failedAttempts); err == nil {
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
			// A permanently-undeliverable message (blocked bot, malformed
			// HTML, oversized text, stale chat) must not retry forever:
			// with a fixed LIMIT 10 oldest-first batch, enough stuck rows
			// occupy every slot in every future batch and starve every
			// genuinely new notification behind them - for any player,
			// any feature. Give up after maxNotificationAttempts rather
			// than silently blocking the whole queue.
			for _, id := range ids {
				gaveUp, recordErr := recordFailedAttempt(ctx, d.DB, id)
				if recordErr != nil {
					log.Printf("Dispatcher failed recording delivery failure for notification %s: %v", id, recordErr)
					continue
				}
				if gaveUp {
					log.Printf("Dispatcher giving up on notification %s to %d after %d failed attempts: %.200s", id, k.userID, maxNotificationAttempts, k.message)
				}
			}
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
