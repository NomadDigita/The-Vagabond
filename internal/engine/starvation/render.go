package starvation

import "strings"

// htmlEscapeStarvation makes a string safe to place inside a Telegram
// HTML-mode message. Duplicated locally (rather than imported from
// bot/handlers or internal/engine/tick) to keep this package
// dependency-free, per the repo's package-per-feature convention. Any
// player-set name (encampment, hero, clan) interpolated into a
// notification message MUST go through this first.
//
// Note: the world_news headline this package writes for a Ghost Mode
// collapse does NOT need this treatment - internal/bot/handlers/world.go
// htmlEscape()s the entire aggregated news feed text at render time, so
// a raw player name embedded in a headline is already safe by the time
// it reaches a player's screen. Only the direct "INSERT INTO
// notifications" alert below is sent without any such downstream
// escaping and must be escaped here.
func htmlEscapeStarvation(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// htmlBoldStarvation/htmlCodeStarvation wrap already-safe text in the
// matching Telegram HTML tag. Duplicated locally (see
// htmlEscapeStarvation doc) to keep this package dependency-free.
func htmlBoldStarvation(s string) string { return "<b>" + s + "</b>" }
func htmlCodeStarvation(s string) string { return "<code>" + s + "</code>" }
