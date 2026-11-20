package handlers

import (
	"errors"

	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
	"gopkg.in/telebot.v3"
)

const (
	EffectFire        = notifications.EffectFire
	EffectThumbsUp    = notifications.EffectThumbsUp
	EffectThumbsDown  = notifications.EffectThumbsDown
	EffectHeart       = notifications.EffectHeart
	EffectCelebration = notifications.EffectCelebration
	EffectPoop        = notifications.EffectPoop
)

func sendRichMessage(c telebot.Context, htmlContent string, opts ...interface{}) error {
	chat := c.Chat()
	if chat == nil {
		return errors.New("no chat in context")
	}
	bot := c.Bot()
	if bot == nil {
		return errors.New("no bot in context")
	}

	payload := map[string]interface{}{
		"chat_id":      chat.ID,
		"rich_message": map[string]interface{}{"html": htmlContent},
	}
	for _, opt := range opts {
		if v, ok := opt.(*telebot.ReplyMarkup); ok && v != nil {
			payload["reply_markup"] = v
		}
	}

	if _, err := bot.Raw("sendRichMessage", payload); err != nil {
		fallbackOpts := append([]interface{}{telebot.ModeHTML}, filterReplyMarkup(opts)...)
		return c.Send(htmlContent, fallbackOpts...)
	}
	return nil
}

func filterReplyMarkup(opts []interface{}) []interface{} {
	var out []interface{}
	for _, opt := range opts {
		if v, ok := opt.(*telebot.ReplyMarkup); ok && v != nil {
			out = append(out, v)
		}
	}
	return out
}

func sendWithEffect(c telebot.Context, effectID string, text string, opts ...interface{}) error {
	chat := c.Chat()
	if chat == nil {
		return errors.New("no chat in context")
	}
	bot := c.Bot()
	if bot == nil {
		return errors.New("no bot in context")
	}

	payload := map[string]interface{}{
		"chat_id":           chat.ID,
		"text":              text,
		"message_effect_id": effectID,
	}
	for _, opt := range opts {
		switch v := opt.(type) {
		case telebot.ParseMode:
			if v != "" {
				payload["parse_mode"] = v
			}
		case *telebot.ReplyMarkup:
			if v != nil {
				payload["reply_markup"] = v
			}
		}
	}

	if _, err := bot.Raw("sendMessage", payload); err != nil {
		return c.Send(text, opts...)
	}
	return nil
}
