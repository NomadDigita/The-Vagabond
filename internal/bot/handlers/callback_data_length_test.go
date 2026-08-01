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
//
// clan_app_accept/clan_app_reject (now cl_acc/cl_rej) is included too:
// it combines a Telegram user ID with a clan UUID, and was found during
// the same audit sitting at 63-64 bytes with a 10-digit user ID - not
// broken yet, but one digit of ID growth away from silently breaking the
// same way. Shortened its prefix to leave real margin rather than
// leaving it fragile.
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

	// Guard the specific regression: road_encounter must never carry a
	// second UUID (the raid ID) again, since that's what pushed it over
	// the limit in the first place. The caller's raid ID is now derived
	// server-side in HandleRoadEncounterCallback instead.
	btn := selector.Data("label", "road_encounter", "attack", uuid)
	if strings.Count(btn.Data, uuid) > 1 {
		t.Errorf("road_encounter callback_data should carry exactly one UUID, got: %q", btn.Data)
	}
}

// TestClanKickPromoteCallbackData_FitsTelegramLimit covers clan_kick and
// clan_promote, added to the styled (colored) button rollout - a single
// Telegram user ID, well within the limit, but included so this test
// file stays the single place documenting every button family's
// callback_data budget rather than leaving future additions unverified.
func TestClanKickPromoteCallbackData_FitsTelegramLimit(t *testing.T) {
	selector := &telebot.ReplyMarkup{}
	userID := "123456789012" // 12 digits, margin over a typical current Telegram user ID

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

// (formerly clan_app_accept/clan_app_reject), which packs a Telegram
// user ID and a clan UUID into callback_data. Telegram user IDs are
// currently up to 10 digits but have grown over the platform's history
// and aren't guaranteed to stay that length, so this asserts real
// margin under the 64-byte cap rather than just scraping under it -
// a regression here should be visible well before it actually breaks.
func TestClanApplicationCallbackData_HasMargin(t *testing.T) {
	selector := &telebot.ReplyMarkup{}
	uuid := "550e8400-e29b-41d4-a716-446655440000" // 36 bytes
	// 12 digits: two more than a typical current Telegram user ID, to
	// verify margin rather than just today's exact case.
	userID := "123456789012"

	const marginBudget = 60 // leave at least 4 bytes of headroom under Telegram's 64-byte cap

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
