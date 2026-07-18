package handlers

import "gopkg.in/telebot.v3"

// Standard short captions for the nav-transition message sendPanelWithNav
// sends ahead of the real panel content. Kept centralized so every call
// site announces the same way instead of inventing its own wording.
const (
	navCaptionMain    = "🏠 Returning to Outpost HQ..."
	navCaptionCamp    = "🏕️ Accessing Outpost Command..."
	navCaptionCombat  = "⚔️ Accessing Tactical Combat..."
	navCaptionEconomy = "💰 Accessing Economic Systems..."
)

// sendPanelWithNav sends a panel that needs both its own inline action
// buttons AND a persistent bottom navigation bar update.
//
// Telegram's Bot API only allows ONE reply_markup per message - it's an
// inline keyboard, OR a persistent ReplyKeyboardMarkup, never both at
// once. telebot's option parser reflects that: when multiple
// *telebot.ReplyMarkup values are passed to c.Send, the LAST one simply
// overwrites the others rather than merging, so `c.Send(text, selector,
// someNavKeyboard)` silently discards selector's inline buttons every
// time. (See SPACEHUNT_PHASE7_LOG.md item 3's post-mortem for the full
// incident writeup - this shipped broken across a number of panels
// before being caught.)
//
// The correct way to get both onscreen is two messages: a short one
// that plants the new persistent keyboard, then the real panel content
// with just its inline selector attached (which leaves the
// just-planted persistent keyboard alone, since Telegram only replaces
// it when a NEW reply_markup of that kind is sent).
func sendPanelWithNav(c telebot.Context, navCaption string, nav *telebot.ReplyMarkup, text string, selector *telebot.ReplyMarkup) error {
	if _, err := c.Bot().Send(c.Recipient(), navCaption, nav); err != nil {
		return err
	}
	return c.Send(text, selector)
}

// weatherLine gives the Wasteland Radio feed's one-line-per-continent
// description for a given active event type ("" or "nominal" for clear
// conditions). See internal/engine/world/weather.go's eventHeadline for
// the matching news-flash wording, and each mechanical consumer
// (internal/engine/tick/engine.go, internal/engine/resource/resource.go,
// this package's combat.go/camp.go) for where the effects actually
// apply.
func weatherLine(eventType string) string {
	switch eventType {
	case "solar_flare":
		return "⚡ Solar Flare - generators at 200%, targeting scrambled."
	case "radiation_storm":
		return "☢️ Radiation Storm - offense -25%, solar power -50%."
	case "acid_rain":
		return "🌧️ Acid Rain - build times doubled, Mechs -50% defense."
	case "emp":
		return "🌩️ EMP - automation and electricity generation offline."
	case "supply_crisis":
		return "📉 Supply Crisis - Market Exchange sale prices depressed."
	case "disease":
		return "🦠 Disease Outbreak - rations consumption elevated."
	case "sandstorm":
		return "🌪️ Sandstorm - march times and accuracy degraded."
	default:
		return "☀️ Nominal - no active debuffs."
	}
}
