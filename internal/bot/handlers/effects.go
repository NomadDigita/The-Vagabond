package handlers

import (
	"errors"

	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
	"gopkg.in/telebot.v3"
)

// This file adds support for Telegram's message-effect animations
// (Bot API 7.10+ `message_effect_id` parameter on sendMessage) - a
// small one-shot animation that plays for the recipient when a message
// arrives, the same effect a human gets from long-pressing a message
// and picking one from Telegram's own effect picker.
//
// 2026-08-06 direct request, after this project's telebot.v3 fork
// (go-telebot/telebot v3.3.8) was confirmed to have no typed field for
// it anywhere in SendOptions. Rather than vendoring a patched copy of
// the library or hand-rolling a bypass around it, this goes through
// Bot.Raw - telebot's own sanctioned escape hatch (see its doc comment
// in the vendored api.go: "Raw lets you call any method of Bot API
// manually... handles API errors, so you only need to unwrap result").
// That's the library's intended answer to exactly this situation - a
// real Bot API field the typed wrapper hasn't caught up to yet - not a
// workaround of it.

// Defined once in internal/engine/notifications (shared by both this
// synchronous path and the async tick-engine queuing path - see
// notifications.go/preferences.go) and re-exported here so existing/
// future call sites in this package can keep writing the shorter
// handlers.EffectFire etc.
const (
	EffectFire        = notifications.EffectFire
	EffectThumbsUp    = notifications.EffectThumbsUp
	EffectThumbsDown  = notifications.EffectThumbsDown
	EffectHeart       = notifications.EffectHeart
	EffectCelebration = notifications.EffectCelebration
	EffectPoop        = notifications.EffectPoop
)

// sendWithEffect sends text to the chat behind c with a message effect
// attached, otherwise behaving like c.Send(text, opts...) - the same
// telebot.ModeHTML/*telebot.ReplyMarkup option values already used
// everywhere else in this package work here unchanged. Effects are
// cosmetic and best-effort by design: if the raw call fails for any
// reason (unexpected Bot API change, rate limit, etc.) this silently
// falls back to a normal c.Send rather than losing the message
// entirely - the animation is a nice-to-have, the message itself never
// is.
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
		// Best-effort animation - fall back to a plain send so the
		// actual game content (raid result, boss kill, etc.) still
		// reaches the player even if the effect itself was rejected.
		return c.Send(text, opts...)
	}
	return nil
}
