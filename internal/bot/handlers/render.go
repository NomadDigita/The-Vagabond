package handlers

import (
	"fmt"
	"strings"
	"time"
)

// formatDuration renders a duration the way a player actually thinks
// about time - "2h 15m" or "3d 6h", not "8100s" - by picking the two
// largest non-zero units (seconds only appear on their own once under a
// minute). Negative or zero durations always show "0s" rather than a
// negative number, since every caller uses this for a countdown that's
// either still running or has already elapsed.
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	total := int64(d.Seconds())

	weeks := total / (7 * 24 * 3600)
	total %= 7 * 24 * 3600
	days := total / (24 * 3600)
	total %= 24 * 3600
	hours := total / 3600
	total %= 3600
	minutes := total / 60
	seconds := total % 60

	switch {
	case weeks > 0:
		return fmt.Sprintf("%dw %dd", weeks, days)
	case days > 0:
		return fmt.Sprintf("%dd %dh", days, hours)
	case hours > 0:
		return fmt.Sprintf("%dh %dm", hours, minutes)
	case minutes > 0:
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	default:
		return fmt.Sprintf("%ds", seconds)
	}
}

// This file holds small, dependency-free helpers used to render richer
// Telegram messages via HTML parse mode (bold/italic/underline/code/
// blockquote) instead of plain text. Telegram's Bot API HTML mode only
// recognizes a small whitelist of tags and requires the three reserved
// characters "&", "<", ">" to be escaped in any text that isn't meant
// to be a tag - that includes ALL user-supplied strings (descriptions,
// custom names, feedback text, chat messages) since a raw "<" or "&" in
// user text will make Telegram reject the whole message with a
// "can't parse entities" 400 error. Game-generated strings built purely
// from our own constants/numbers don't need escaping, but anything that
// passed through user input MUST go through htmlEscape first.
//
// Usage pattern for an existing plain c.Send(text) call site:
//
//	return c.Send(htmlBold("TITLE")+"\n"+body, telebot.ModeHTML)
//
// with any interpolated user text wrapped as htmlEscape(userText).

// htmlEscape makes a string safe to place inside an HTML-parse-mode
// Telegram message. Order matters: "&" must be escaped first, or it
// would double-escape the "&" produced by escaping "<"/">".
func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return r.Replace(s)
}

// htmlBold wraps already-safe text in a bold tag.
func htmlBold(s string) string { return "<b>" + s + "</b>" }

// htmlItalic wraps already-safe text in an italic tag.
func htmlItalic(s string) string { return "<i>" + s + "</i>" }

// htmlCode wraps already-safe text in an inline monospace tag - handy
// for numeric readouts (resource counts, coordinates, percentages)
// so they line up visually.
func htmlCode(s string) string { return "<code>" + s + "</code>" }

// htmlUnderline wraps already-safe text in an underline tag.
func htmlUnderline(s string) string { return "<u>" + s + "</u>" }

// htmlQuote wraps already-safe (possibly multi-line) text in a
// Telegram "blockquote" - useful for flavor text / lore lines so they
// visually separate from the mechanical readout above them.
func htmlQuote(s string) string { return "<blockquote>" + s + "</blockquote>" }

// htmlSpoiler wraps text in a tap-to-reveal spoiler tag - used for
// things a player might want to hide at a glance (e.g. surprise loot).
func htmlSpoiler(s string) string { return "<span class=\"tg-spoiler\">" + s + "</span>" }

// divider is a soft section separator used between blocks in longer
// panel messages so they don't read as one dense wall of text.
const divider = "┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈"

// resourceEmoji gives a consistent icon per resource/item type across
// every panel that lists tradeable stock (Exchange, Vault, Silo,
// Deconstruct, etc.) instead of each screen picking its own.
func resourceEmoji(itemType string) string {
	switch strings.ToLower(itemType) {
	case "metal":
		return "🔩"
	case "crystal":
		return "🔮"
	case "scrap":
		return "⚙️"
	case "dollars", "cash":
		return "💵"
	case "hydrogen":
		return "🎈"
	case "rations":
		return "🍖"
	case "soldiers", "soldier":
		return "🪖"
	case "mechs", "mech":
		return "🤖"
	default:
		return "📦"
	}
}
