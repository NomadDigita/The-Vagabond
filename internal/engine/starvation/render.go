package starvation

import "strings"

func htmlEscapeStarvation(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

func htmlBoldStarvation(s string) string { return "<b>" + s + "</b>" }
func htmlCodeStarvation(s string) string { return "<code>" + s + "</code>" }
