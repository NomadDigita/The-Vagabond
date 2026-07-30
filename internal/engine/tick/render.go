package tick

import (
	"fmt"
	"strings"
	"time"
)

// formatDurationTick mirrors bot/handlers/render.go's formatDuration -
// duplicated locally for the same dependency-free reasons as
// htmlEscapeTick above. Renders "2h 15m" / "3d 6h" instead of a raw
// second count.
func formatDurationTick(d time.Duration) string {
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

// htmlEscapeTick makes a string safe to place inside a Telegram
// HTML-mode message. Duplicated locally (rather than imported from
// bot/handlers or battlereport) to keep this package dependency-free,
// per the repo's package-per-feature convention. Any player-set name
// (encampment, hero, clan) interpolated into a notification message
// MUST go through this first.
func htmlEscapeTick(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// htmlBoldTick/htmlItalicTick/htmlCodeTick wrap already-safe text in the
// matching Telegram HTML tag. Duplicated locally (see htmlEscapeTick doc)
// to keep this package dependency-free.
func htmlBoldTick(s string) string   { return "<b>" + s + "</b>" }
func htmlItalicTick(s string) string { return "<i>" + s + "</i>" }
func htmlCodeTick(s string) string   { return "<code>" + s + "</code>" }

// resourceEmojiTick gives a consistent icon per resource type across
// every tick-engine notification (mining, raid loot, boss loot, etc.)
func resourceEmojiTick(resType string) string {
	switch strings.ToLower(resType) {
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
	case "electricity":
		return "⚡"
	case "neuro_cores", "neuro cores":
		return "🧠"
	default:
		return "📦"
	}
}
