package handlers

import (
	"fmt"
	"strings"
	"time"
)

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

func htmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
	)
	return r.Replace(s)
}

func htmlBold(s string) string { return "<b>" + s + "</b>" }

func htmlItalic(s string) string { return "<i>" + s + "</i>" }

func htmlCode(s string) string { return "<code>" + s + "</code>" }

func htmlUnderline(s string) string { return "<u>" + s + "</u>" }

func htmlQuote(s string) string { return "<blockquote>" + s + "</blockquote>" }

func htmlSpoiler(s string) string { return "<span class=\"tg-spoiler\">" + s + "</span>" }

const divider = "┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈┈"

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
