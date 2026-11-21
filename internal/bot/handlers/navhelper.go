package handlers

import "gopkg.in/telebot.v3"

const (
	navCaptionMain    = "🏠 Returning to Outpost HQ..."
	navCaptionCamp    = "🏕️ Accessing Outpost Command..."
	navCaptionCombat  = "⚔️ Accessing Tactical Combat..."
	navCaptionEconomy = "💰 Accessing Economic Systems..."
	navCaptionProfile = "📊 Accessing Player Profile..."
	navCaptionAdmin   = "🏛️ Accessing Admin Override Terminal..."
)

func sendPanelWithNav(c telebot.Context, navCaption string, nav *telebot.ReplyMarkup, text string, selector *telebot.ReplyMarkup) error {
	if _, err := c.Bot().Send(c.Recipient(), navCaption, nav); err != nil {
		return err
	}
	return c.Send(text, selector)
}

func sendPanelWithNavHTML(c telebot.Context, navCaption string, nav *telebot.ReplyMarkup, text string, selector *telebot.ReplyMarkup) error {
	if _, err := c.Bot().Send(c.Recipient(), navCaption, nav); err != nil {
		return err
	}
	return c.Send(text, telebot.ModeHTML, selector)
}

func sendPanelWithNavRich(c telebot.Context, navCaption string, nav *telebot.ReplyMarkup, text string, selector *telebot.ReplyMarkup) error {
	if _, err := c.Bot().Send(c.Recipient(), navCaption, nav); err != nil {
		return err
	}
	return sendRichMessage(c, text, selector)
}

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
	case "bloom":
		return "🌸 Wasteland Bloom - all passive resource generation +15%."
	default:
		return "☀️ Nominal - no active debuffs."
	}
}
