package handlers

import (
	"strings"
	"testing"

	"gopkg.in/telebot.v3"
)

func TestRoadEncounterCallbackData_FitsTelegramLimit(t *testing.T) {
	selector := &telebot.ReplyMarkup{}
	uuid := "550e8400-e29b-41d4-a716-446655440000"

	cases := []struct {
		name   string
		unique string
		action string
	}{
		{"road_encounter attack", "road_encounter", "attack"},
		{"road_encounter continue", "road_encounter", "continue"},
		{"rbe attack", "rbe", "attack"},
		{"rbe continue", "rbe", "continue"},
		{"diplo_respond accept", "diplo_respond", "accept"},
		{"diplo_respond reject", "diplo_respond", "reject"},
		{"exp_action speed", "exp_action", "speed"},
		{"exp_action abort", "exp_action", "abort"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			btn := selector.Data("label", tc.unique, tc.action, uuid)
			full := "\f" + btn.Unique + "|" + btn.Data
			if n := len(full); n > 64 {
				t.Errorf("%s: callback_data is %d bytes (max 64) - Telegram will silently drop this button: %q", tc.name, n, full)
			}
		})
	}

	btn := selector.Data("label", "road_encounter", "attack", uuid)
	if strings.Count(btn.Data, uuid) > 1 {
		t.Errorf("road_encounter callback_data should carry exactly one UUID, got: %q", btn.Data)
	}
}

func TestClanKickPromoteCallbackData_FitsTelegramLimit(t *testing.T) {
	selector := &telebot.ReplyMarkup{}
	userID := "123456789012"

	for _, unique := range []string{"clan_kick", "clan_promote"} {
		t.Run(unique, func(t *testing.T) {
			btn := selector.Data("label", unique, userID)
			full := "\f" + btn.Unique + "|" + btn.Data
			if n := len(full); n > 64 {
				t.Errorf("%s: callback_data is %d bytes (max 64) - Telegram will silently drop this button: %q", unique, n, full)
			}
		})
	}
}

func TestClanApplicationCallbackData_HasMargin(t *testing.T) {
	selector := &telebot.ReplyMarkup{}
	uuid := "550e8400-e29b-41d4-a716-446655440000"
	userID := "123456789012"

	const marginBudget = 60

	for _, unique := range []string{"cl_acc", "cl_rej"} {
		t.Run(unique, func(t *testing.T) {
			btn := selector.Data("label", unique, userID, uuid)
			full := "\f" + btn.Unique + "|" + btn.Data
			if n := len(full); n > 64 {
				t.Fatalf("%s: callback_data is %d bytes (max 64) - Telegram will silently drop this button: %q", unique, n, full)
			} else if n > marginBudget {
				t.Errorf("%s: callback_data is %d bytes - within Telegram's 64-byte cap but with less than the desired margin (budget %d); consider a shorter prefix", unique, n, marginBudget)
			}
		})
	}
}
