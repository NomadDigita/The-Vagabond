package keyboards

import (
	"bytes"
	"encoding/json"
	"fmt"

	"gopkg.in/telebot.v3"
)

type ButtonStyle string

const (
	StyleDanger  ButtonStyle = "danger"
	StyleSuccess ButtonStyle = "success"
	StylePrimary ButtonStyle = "primary"
)

type StyledBtn struct {
	telebot.Btn
	Style ButtonStyle
}

func Styled(btn telebot.Btn, style ButtonStyle) StyledBtn {
	return StyledBtn{Btn: btn, Style: style}
}

type rawInlineButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data,omitempty"`
	URL          string `json:"url,omitempty"`
	Style        string `json:"style,omitempty"`
}

func styledCallbackData(b telebot.Btn) string {
	if b.Unique == "" {
		return ""
	}
	if b.Data == "" {
		return "\f" + b.Unique
	}
	return "\f" + b.Unique + "|" + b.Data
}

const telegramCallbackDataLimit = 64

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

type rawSendMessage struct {
	ChatID      string `json:"chat_id"`
	Text        string `json:"text"`
	ParseMode   string `json:"parse_mode,omitempty"`
	ReplyMarkup struct {
		InlineKeyboard [][]rawInlineButton `json:"inline_keyboard"`
	} `json:"reply_markup"`
}

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

	for _, row := range payload.ReplyMarkup.InlineKeyboard {
		for _, btn := range row {
			if n := len(btn.CallbackData); n > telegramCallbackDataLimit {
				return fmt.Errorf("styled keyboard: button %q has a %d-byte callback_data (Telegram's limit is %d) - this button would silently never respond", btn.Text, n, telegramCallbackDataLimit)
			}
		}
	}

	_, err := bot.Raw("sendMessage", payload)
	return err
}

func marshalRowsForTest(rows [][]StyledBtn) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(toRawRows(rows)); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}
