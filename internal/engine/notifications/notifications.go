package notifications

import (
	"context"
	"database/sql"
	"log"
	"runtime/debug"
	"strings"
	"time"

	"gopkg.in/telebot.v3"
)

const (
	EffectFire        = "5104841245755180586"
	EffectThumbsUp    = "5107584321108051014"
	EffectThumbsDown  = "5104858069142078462"
	EffectHeart       = "5159385139981059251"
	EffectCelebration = "5046509860389126442"
	EffectPoop        = "5046589136895476101"
)

type Dispatcher struct {
	DB       *sql.DB
	Bot      *telebot.Bot
	stopChan chan struct{}
}

const maxNotificationAttempts = 5

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

func (d *Dispatcher) Start() {
	log.Println("Notification Dispatcher system online. Draining queues...")
	ticker := time.NewTicker(3 * time.Second)

	go func() {
		for {
			select {
			case <-ticker.C:
				func() {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("Notification Dispatcher: PANIC (recovered), dispatcher stays alive: %v\n%s", r, debug.Stack())
						}
					}()
					d.drainQueue()
				}()
			case <-d.stopChan:
				ticker.Stop()
				log.Println("Notification Dispatcher stopped.")
				return
			}
		}
	}()
}

func (d *Dispatcher) Stop() {
	close(d.stopChan)
}

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

	if _, err := d.DB.ExecContext(ctx, "UPDATE notifications SET is_sent = TRUE WHERE is_sent = FALSE AND user_id <= 0"); err != nil {
		log.Printf("Dispatcher failed sweeping non-player (AI faction) notifications: %v", err)
	}

	query := `
		SELECT id, user_id, message, failed_attempts, COALESCE(effect_id, '')
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
		effectID       string
	}

	var queue []pending
	for rows.Next() {
		var p pending
		if err := rows.Scan(&p.id, &p.userID, &p.message, &p.failedAttempts, &p.effectID); err == nil {
			queue = append(queue, p)
		}
	}

	type dedupKey struct {
		userID  int64
		message string
	}
	groups := make(map[dedupKey][]string)
	effects := make(map[dedupKey]string)
	order := make([]dedupKey, 0, len(queue))
	for _, p := range queue {
		k := dedupKey{p.userID, p.message}
		if _, seen := groups[k]; !seen {
			order = append(order, k)
			effects[k] = p.effectID
		}
		groups[k] = append(groups[k], p.id)
	}

	for _, k := range order {
		ids := groups[k]
		targetUser := &telebot.User{ID: k.userID}

		isHTML := looksLikeHTML(k.message)
		opts := []interface{}{}
		if isHTML {
			opts = append(opts, telebot.ModeHTML)
		}

		var err error
		if effectID := effects[k]; effectID != "" {
			payload := map[string]interface{}{
				"chat_id":           k.userID,
				"text":              k.message,
				"message_effect_id": effectID,
			}
			if isHTML {
				payload["parse_mode"] = telebot.ModeHTML
			}
			if _, rawErr := d.Bot.Raw("sendMessage", payload); rawErr != nil {
				_, err = d.Bot.Send(targetUser, k.message, opts...)
			}
		} else {
			_, err = d.Bot.Send(targetUser, k.message, opts...)
		}
		if err != nil {
			log.Printf("Dispatcher failed to deliver notification to %d: %v", k.userID, err)
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

		for _, id := range ids {
			if _, err := d.DB.ExecContext(ctx, "UPDATE notifications SET is_sent = TRUE WHERE id = $1", id); err != nil {
				log.Printf("Dispatcher failed updating persistence sentinel state for notification %s: %v", id, err)
			}
		}
	}
}
