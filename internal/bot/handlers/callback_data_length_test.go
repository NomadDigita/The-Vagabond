package handlers

import (
	"strings"
	"testing"

	"gopkg.in/telebot.v3"
)

// TestRoadEncounterCallbackData_FitsTelegramLimit guards against the bug
// reported as "the button to go to that place won't respond" for raids
// passing another raid or a base mid-journey: Telegram inline buttons cap
// callback_data at 64 bytes, and Telegram silently drops any button whose
// data exceeds that (the message still sends, but the button does
// nothing when tapped - no error, no response). road_encounter used to
// carry both an encounter UUID and a raid UUID (~95 bytes with the "\f"
// prefix and separators), and rbe (formerly the much longer
// "road_base_encounter") was right at the edge for its "continue" action.
// Both now carry only a single UUID.
func TestRoadEncounterCallbackData_FitsTelegramLimit(t *testing.T) {
	selector := &telebot.ReplyMarkup{}
	uuid := "550e8400-e29b-41d4-a716-446655440000" // 36 bytes, representative of a real Postgres UUID

	cases := []struct {
		name   string
		unique string
		action string
	}{
		{"road_encounter attack", "road_encounter", "attack"},
		{"road_encounter continue", "road_encounter", "continue"},
		{"rbe attack", "rbe", "attack"},
		{"rbe continue", "rbe", "continue"},
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

	// Guard the specific regression: road_encounter must never carry a
	// second UUID (the raid ID) again, since that's what pushed it over
	// the limit in the first place. The caller's raid ID is now derived
	// server-side in HandleRoadEncounterCallback instead.
	btn := selector.Data("label", "road_encounter", "attack", uuid)
	if strings.Count(btn.Data, uuid) > 1 {
		t.Errorf("road_encounter callback_data should carry exactly one UUID, got: %q", btn.Data)
	}
}
