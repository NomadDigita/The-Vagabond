// styled.go adds support for Telegram Bot API 9.4's button "style"
// field (added Feb 9, 2026: bots can now mark a button primary/success/
// danger and Telegram renders it blue/green/red).
//
// telebot.v3 (the library this project is pinned to) doesn't model this
// field yet - its InlineButton struct has no Style property, and its
// json.Marshaler would silently drop anything bolted on. So this
// bypasses the typed button path entirely for styled sends: it builds
// the raw sendMessage JSON by hand and fires it through
// telebot.Bot.Raw(), which accepts any interface{} and POSTs it directly
// to the Bot API - no library upgrade required, and nothing here reaches
// into telebot's internals.
//
// The one thing that has to be replicated exactly is telebot's
// callback_data format, so a styled button's press still reaches the
// same bot.Handle("\funique", ...) registrations every other button in
// this codebase already uses. See options.go's processButtons in
// telebot.v3@v3.3.8: callback_data is always "\f" + unique, or
// "\f" + unique + "|" + data if data is non-empty. styledCallbackData
// below reproduces that byte-for-byte.
package keyboards

import (
	"bytes"
	"encoding/json"
	"fmt"

	"gopkg.in/telebot.v3"
)

// ButtonStyle is one of the three colors Telegram actually offers -
// there's no arbitrary palette, just these three (see Bot API 9.4's
// KeyboardButtonStyle: bg_primary, bg_danger, bg_success), so callers
// pick one of these constants rather than a free-form string.
type ButtonStyle string

const (
	// StyleDanger renders red - for destructive or hostile actions
	// (Attack, Abort, Kick).
	StyleDanger ButtonStyle = "danger"
	// StyleSuccess renders green - for safe, confirming, or peaceful
	// actions (Continue, Join, Confirm, Recover).
	StyleSuccess ButtonStyle = "success"
	// StylePrimary renders blue - for neutral navigation or informational
	// actions that aren't a destructive/safe binary choice.
	StylePrimary ButtonStyle = "primary"
)

// StyledBtn pairs a normal telebot.Btn (built exactly the way every
// other button in this codebase already is, e.g. via
// selector.Data(text, unique, args...)) with a color. Embedding Btn
// means callers don't learn a second button-building API - they just
// wrap the Btn they already have.
type StyledBtn struct {
	telebot.Btn
	Style ButtonStyle
}

// Styled wraps an existing Btn with a color. Typical use:
//
//	btnAttack := keyboards.Styled(selector.Data("⚔️ Attack", "road_encounter", "attack", encID, raidID), keyboards.StyleDanger)
//	btnContinue := keyboards.Styled(selector.Data("➡️ Continue", "road_encounter", "continue", encID, raidID), keyboards.StyleSuccess)
func Styled(btn telebot.Btn, style ButtonStyle) StyledBtn {
	return StyledBtn{Btn: btn, Style: style}
}

// rawInlineButton is the literal Bot API 9.4 wire shape for one inline
// button, including the style field telebot.InlineButton doesn't have
// yet. Field names/tags match
// https://core.telegram.org/bots/api#inlinekeyboardbutton plus the new
// style addition exactly.
type rawInlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
	Style        string `json:"style,omitempty"`
}

// styledCallbackData reproduces telebot.v3@v3.3.8's options.go
// processButtons formatting exactly, so a press on a styled button
// still matches the same bot.Handle("\f<unique>", ...) registration a
// non-styled button with the same Unique/Data would.
func styledCallbackData(b telebot.Btn) string {
	if b.Unique == "" {
		return ""
	}
	if b.Data == "" {
		return "\f" + b.Unique
	}
	return "\f" + b.Unique + "|" + b.Data
}

func toRawButton(b StyledBtn) rawInlineButton {
	return rawInlineButton{
		Text:         b.Text,
		CallbackData: styledCallbackData(b.Btn),
		URL:          b.URL,
		Style:        string(b.Style),
	}
}

func toRawRows(rows [][]StyledBtn) [][]rawInlineButton {
	raw := make([][]rawInlineButton, len(rows))
	for i, row := range rows {
		r := make([]rawInlineButton, len(row))
		for j, btn := range row {
			r[j] = toRawButton(btn)
		}
		raw[i] = r
	}
	return raw
}

// rawSendMessage is the minimal subset of sendMessage's parameters this
// package needs - just enough to carry text, HTML formatting, and a
// styled inline keyboard. Everything else behaves exactly like a normal
// telebot c.Send(text, telebot.ModeHTML) call.
type rawSendMessage struct {
	ChatID      string `json:"chat_id"`
	Text        string `json:"text"`
	ParseMode   string `json:"parse_mode,omitempty"`
	ReplyMarkup struct {
		InlineKeyboard [][]rawInlineButton `json:"inline_keyboard"`
	} `json:"reply_markup"`
}

// SendStyled sends text with a colored inline keyboard to whatever
// c.Chat() resolves to - the same recipient c.Send() would use. Text is
// always sent as HTML (every call site building StyledBtn rows also
// wants htmlBold/htmlCode-formatted text, matching this game's existing
// panel style), so callers don't need to pass a parse mode.
func SendStyled(c telebot.Context, text string, rows [][]StyledBtn) error {
	bot := c.Bot()
	if bot == nil {
		return fmt.Errorf("styled keyboard: no bot on context")
	}
	chat := c.Chat()
	if chat == nil {
		return fmt.Errorf("styled keyboard: no chat on context")
	}

	var payload rawSendMessage
	payload.ChatID = chat.Recipient()
	payload.Text = text
	payload.ParseMode = "HTML"
	payload.ReplyMarkup.InlineKeyboard = toRawRows(rows)

	_, err := bot.Raw("sendMessage", payload)
	return err
}

// marshalRowsForTest exposes the exact outgoing inline_keyboard JSON for
// a row of styled buttons without needing a live *telebot.Bot/Raw()
// round trip - styled_test.go uses this to verify the wire shape (field
// names, omitted-when-empty behavior, callback_data format) directly.
func marshalRowsForTest(rows [][]StyledBtn) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(toRawRows(rows)); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
