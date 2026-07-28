package agent

import "strings"

// htmlEscapeAgent makes a string safe to place inside a Telegram
// HTML-mode message. Duplicated locally (rather than imported from
// bot/handlers or internal/engine/tick) to keep this package
// dependency-free, per the repo's package-per-feature convention. Any
// player-set name (encampment, hero, clan) interpolated into a
// notification message MUST go through this first.
func htmlEscapeAgent(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// htmlBoldAgent/htmlCodeAgent wrap already-safe text in the matching
// Telegram HTML tag. Duplicated locally (see htmlEscapeAgent doc) to
// keep this package dependency-free.
func htmlBoldAgent(s string) string { return "<b>" + s + "</b>" }
func htmlCodeAgent(s string) string { return "<code>" + s + "</code>" }
