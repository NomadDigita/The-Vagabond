package handlers

import "strings"

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
