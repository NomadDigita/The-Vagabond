package ai

import "strings"

// This file holds small, dependency-free helpers shared by every AI
// feature's FormatForTelegram function (Governor, Fleet Commander,
// Economy/Research/Battle/Guild/Galaxy/NPC advisors, Dev Console).
// All nine of those packages already import internal/ai for
// WasTruncated/SanitizeJSONControlChars (ADR-015/016), so this is a
// natural shared home instead of duplicating the same six functions
// nine times over.
//
// IMPORTANT: every field on a Recommendation - Summary, action/target/
// reason strings, warnings, the raw fallback text, etc. - originates
// from an LLM's response text, not from our own game logic. That text
// is NOT guaranteed to be free of literal "<", ">", or "&" (a model
// can very plausibly write something like "trigger when HP < 30%").
// Every one of those fields MUST go through HTMLEscape before being
// wrapped in a tag, or Telegram will reject the whole message with a
// "can't parse entities" 400 error the first time a model happens to
// use a comparison operator in its own words.

// HTMLEscape makes a string safe to place inside a Telegram HTML-mode
// message.
func HTMLEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

// HTMLBold/HTMLItalic/HTMLCode/HTMLPre wrap already-escaped text in the
// matching Telegram HTML tag. HTMLPre renders as a full monospace block
// (Telegram's equivalent of a markdown ``` fence, which isn't itself a
// recognized HTML-mode tag) - used for raw/fallback AI text dumps.
func HTMLBold(s string) string   { return "<b>" + s + "</b>" }
func HTMLItalic(s string) string { return "<i>" + s + "</i>" }
func HTMLCode(s string) string   { return "<code>" + s + "</code>" }
func HTMLPre(s string) string    { return "<pre>" + s + "</pre>" }

// HTMLStrike renders already-escaped text with a strikethrough - handy
// for showing a superseded value next to its replacement (e.g. a
// balance-change "before" number).
func HTMLStrike(s string) string { return "<s>" + s + "</s>" }

// HTMLQuote wraps already-escaped (possibly multi-line) text in a
// plain Telegram blockquote - a visual quote bar down the left edge,
// always fully visible. Use for short quoted/flavor text.
func HTMLQuote(s string) string { return "<blockquote>" + s + "</blockquote>" }

// HTMLExpandableQuote wraps already-escaped (possibly multi-line) text
// in Telegram's collapsible "expandable_blockquote" (Bot API 7.3+):
// the client shows it collapsed with a tap-to-expand affordance,
// rather than dumping the whole thing inline. Built specifically for
// AI-generated narrative/reasoning fields, which can legitimately run
// to several sentences and would otherwise read as one dense wall of
// text sitting in the middle of an otherwise scannable report.
//
// Telegram doesn't take a separate "preview" string - the whole
// element is one blockquote that's either collapsed or expanded - so
// there's nothing to summarize here beyond passing the full text.
func HTMLExpandableQuote(s string) string {
	return "<blockquote expandable>" + s + "</blockquote>"
}

// HTMLTable renders a small monospace table wrapped in HTMLPre -
// Telegram has no native table entity, so <pre> (fixed-width font,
// whitespace preserved) is the only way to get columns to actually
// line up. See ADR-026 in PROJECT_MASTER_PLAN.md for why this exists
// as a shared helper rather than each caller hand-padding strings.
//
// Deliberately narrow: Telegram mobile clients wrap monospace text at
// roughly 34-40 characters depending on font size and device width,
// past which a "table" just becomes a staircase of wrapped lines -
// worse than no table at all. Column widths here are computed from
// the actual header/cell content (so a table of short values doesn't
// pad out to some fixed width for no reason), and any single cell
// longer than maxCellWidth is truncated with an ellipsis rather than
// being left to blow out the whole layout - callers should pass
// already-short values (unit names, percentages, counts), not prose.
//
// headers and each row must be the same length, or the row is
// skipped rather than producing a misaligned table (silently wrong
// output is worse than an incomplete one here).
//
// IMPORTANT for callers: clip any user-controlled cell content to a
// safe rune length yourself BEFORE calling ai.HTMLEscape on it, not
// after. If you escape first and let this function's own clipping cut
// the (now-escaped) string, it can slice straight through the middle
// of an entity like "&amp;", producing invalid HTML that Telegram
// will reject the entire message for. This function's own clipping
// exists as a backstop against unexpectedly long content, not as the
// primary truncation point for anything that needed escaping.
func HTMLTable(headers []string, rows [][]string) string {
	const maxCellWidth = 18 // keeps a 2-3 column table under Telegram's mobile wrap width

	clip := func(s string) string {
		r := []rune(s)
		if len(r) <= maxCellWidth {
			return s
		}
		if maxCellWidth <= 1 {
			return string(r[:maxCellWidth])
		}
		return string(r[:maxCellWidth-1]) + "…"
	}

	widths := make([]int, len(headers))
	for i, h := range headers {
		h = clip(h)
		if n := len([]rune(h)); n > widths[i] {
			widths[i] = n
		}
	}
	clippedRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		if len(row) != len(headers) {
			continue // malformed row - drop rather than misalign every column after it
		}
		clipped := make([]string, len(row))
		for i, cell := range row {
			clipped[i] = clip(cell)
			if n := len([]rune(clipped[i])); n > widths[i] {
				widths[i] = n
			}
		}
		clippedRows = append(clippedRows, clipped)
	}

	pad := func(s string, w int) string {
		n := w - len([]rune(s))
		if n <= 0 {
			return s
		}
		return s + strings.Repeat(" ", n)
	}

	var b strings.Builder
	for i, h := range headers {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(pad(clip(h), widths[i]))
	}
	b.WriteString("\n")
	for i, w := range widths {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(strings.Repeat("-", w))
	}
	for _, row := range clippedRows {
		b.WriteString("\n")
		for i, cell := range row {
			if i > 0 {
				b.WriteString(" ")
			}
			b.WriteString(pad(cell, widths[i]))
		}
	}
	return HTMLPre(b.String())
}
