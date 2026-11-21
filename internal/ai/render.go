package ai

import "strings"

func HTMLEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

func HTMLBold(s string) string   { return "<b>" + s + "</b>" }
func HTMLItalic(s string) string { return "<i>" + s + "</i>" }
func HTMLCode(s string) string   { return "<code>" + s + "</code>" }
func HTMLPre(s string) string    { return "<pre>" + s + "</pre>" }

func HTMLStrike(s string) string { return "<s>" + s + "</s>" }

func HTMLQuote(s string) string { return "<blockquote>" + s + "</blockquote>" }

func HTMLExpandableQuote(s string) string {
	return "<blockquote expandable>" + s + "</blockquote>"
}

func HTMLTable(headers []string, rows [][]string) string {
	const maxCellWidth = 18

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
			continue
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
