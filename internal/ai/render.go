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
