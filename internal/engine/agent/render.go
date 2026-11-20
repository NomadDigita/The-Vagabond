package agent

import "strings"

func htmlEscapeAgent(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

func htmlBoldAgent(s string) string { return "<b>" + s + "</b>" }
func htmlCodeAgent(s string) string { return "<code>" + s + "</code>" }
