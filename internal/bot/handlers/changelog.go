package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/NomadDigita/The-Vagabond/internal/bot/keyboards"
	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
	"gopkg.in/telebot.v3"
)

type ChangelogHandler struct {
	DB       *sql.DB
	AdminIDs []int64
}

func NewChangelogHandler(db *sql.DB, adminIDs []int64) *ChangelogHandler {
	return &ChangelogHandler{DB: db, AdminIDs: adminIDs}
}

func (h *ChangelogHandler) IsAdmin(senderID int64) bool {
	for _, id := range h.AdminIDs {
		if id == senderID {
			return true
		}
	}
	return false
}

const changelogPageSize = 5

var changelogValidCategories = map[string]bool{"feature": true, "fix": true, "balance": true}

func changelogCategoryIcon(category string) string {
	switch category {
	case "feature":
		return "✨"
	case "fix":
		return "🛠️"
	case "balance":
		return "⚖️"
	default:
		return "📰"
	}
}

func (h *ChangelogHandler) doPublishChangelog(ctx context.Context, category, title, body string) (string, error) {
	category = strings.ToLower(strings.TrimSpace(category))
	title = strings.TrimSpace(title)
	body = strings.TrimSpace(body)

	if !changelogValidCategories[category] {
		return "⚠️ Category must be one of: feature, fix, balance.", errors.New("invalid changelog category")
	}
	if title == "" {
		return "⚠️ Title can't be empty.", errors.New("empty changelog title")
	}
	if body == "" {
		return "⚠️ Body can't be empty.", errors.New("empty changelog body")
	}

	var entryID string
	if err := h.DB.QueryRowContext(ctx,
		"INSERT INTO changelog_entries (category, title, body) VALUES ($1, $2, $3) RETURNING id",
		category, title, body).Scan(&entryID); err != nil {
		return "⚠️ Error publishing changelog entry.", err
	}

	broadcast := fmt.Sprintf("%s %s\n%s\n%s\n%s",
		changelogCategoryIcon(category), htmlBold(strings.ToUpper(category)),
		htmlBold(htmlEscape(title)), divider, htmlEscape(body))
	if err := notifications.QueueToAllPlayers(ctx, h.DB, broadcast, "general"); err != nil {
		return "📰 Changelog entry saved, but broadcasting to players failed - check logs. Players can still find it via /changelog.", err
	}

	return "📰 " + htmlBold("CHANGELOG PUBLISHED") + " and broadcast to every survivor.", nil
}

type changelogEntry struct {
	id          string
	category    string
	title       string
	body        string
	publishedAt time.Time
}

func (h *ChangelogHandler) fetchChangelogPage(ctx context.Context, userID int64) (entries []changelogEntry, caughtUp bool, err error) {
	rows, err := h.DB.QueryContext(ctx, `
		SELECT id, category, title, body, published_at
		FROM changelog_entries
		WHERE id NOT IN (SELECT entry_id FROM changelog_reads WHERE user_id = $1)
		ORDER BY published_at ASC
		LIMIT $2`, userID, changelogPageSize)
	if err != nil {
		return nil, false, err
	}
	for rows.Next() {
		var e changelogEntry
		if scanErr := rows.Scan(&e.id, &e.category, &e.title, &e.body, &e.publishedAt); scanErr == nil {
			entries = append(entries, e)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, false, err
	}

	if len(entries) > 0 {
		return entries, false, nil
	}

	rows, err = h.DB.QueryContext(ctx, `
		SELECT id, category, title, body, published_at
		FROM changelog_entries
		ORDER BY published_at DESC
		LIMIT $1`, changelogPageSize)
	if err != nil {
		return nil, true, err
	}
	defer rows.Close()
	for rows.Next() {
		var e changelogEntry
		if scanErr := rows.Scan(&e.id, &e.category, &e.title, &e.body, &e.publishedAt); scanErr == nil {
			entries = append(entries, e)
		}
	}
	return entries, true, rows.Err()
}

func (h *ChangelogHandler) markChangelogEntriesRead(ctx context.Context, userID int64, entries []changelogEntry) error {
	for _, e := range entries {
		if _, err := h.DB.ExecContext(ctx,
			"INSERT INTO changelog_reads (user_id, entry_id) VALUES ($1, $2) ON CONFLICT DO NOTHING",
			userID, e.id); err != nil {
			return err
		}
	}
	return nil
}

func renderChangelogText(entries []changelogEntry, caughtUp bool) string {
	header := "📰 " + htmlBold("CHANGELOG") + "\n" + divider + "\n"
	if caughtUp {
		header += htmlItalic("You're all caught up! Here's the most recent history:") + "\n\n"
	}
	if len(entries) == 0 {
		return header + htmlItalic("Nothing published yet.") + "\n" + divider
	}

	text := header
	for _, e := range entries {
		text += fmt.Sprintf("%s %s — %s\n%s\n%s\n\n",
			changelogCategoryIcon(e.category), htmlBold(htmlEscape(e.title)),
			htmlCode(e.publishedAt.UTC().Format("Jan 2, 2006")),
			htmlEscape(e.body), divider)
	}
	return strings.TrimSuffix(text, divider+"\n\n") + divider
}

func (h *ChangelogHandler) HandleChangelogPanel(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		return errors.New("invalid sender context")
	}
	ctx := context.Background()

	entries, caughtUp, err := h.fetchChangelogPage(ctx, sender.ID)
	if err != nil {
		return c.Send("⚠️ Error loading the changelog.")
	}
	if !caughtUp {
		if err := h.markChangelogEntriesRead(ctx, sender.ID, entries); err != nil {
			return c.Send("⚠️ Error loading the changelog.")
		}
	}

	panelText := renderChangelogText(entries, caughtUp)

	var remainingUnread int
	_ = h.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM changelog_entries WHERE id NOT IN (SELECT entry_id FROM changelog_reads WHERE user_id = $1)",
		sender.ID).Scan(&remainingUnread)

	var selector *telebot.ReplyMarkup
	if remainingUnread > 0 {
		selector = &telebot.ReplyMarkup{}
		selector.Inline(selector.Row(selector.Data("📰 Show 5 More", "changelog_more")))
	}

	return sendPanelWithNavHTML(c, navCaptionProfile, keyboards.ProfileNavigation(), panelText, selector)
}

func (h *ChangelogHandler) HandleChangelogMoreCallback(c telebot.Context) error {
	_ = c.Respond()
	return h.HandleChangelogPanel(c)
}

func (h *ChangelogHandler) HandlePublishChangelogPendingInput(c telebot.Context) (string, error) {
	lines := strings.SplitN(c.Text(), "\n", 3)
	if len(lines) < 3 {
		return "⚠️ Expected 3 lines: category, title, then body - action cancelled, tap Publish Changelog again to retry.", errors.New("malformed changelog input")
	}
	return h.doPublishChangelog(context.Background(), lines[0], lines[1], lines[2])
}
