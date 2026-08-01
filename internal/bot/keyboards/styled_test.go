package keyboards

import (
	"encoding/json"
	"regexp"
	"testing"

	"gopkg.in/telebot.v3"
)

// telebotCallbackRegex is copied verbatim from telebot.v3@v3.3.8's
// bot.go (the unexported cbackRx the library uses to recognize an
// incoming callback query and route it to a bot.Handle("\f<unique>", ...)
// registration). This test proves a styled button's callback_data still
// matches that exact pattern, so pressing one reaches the same handler
// a non-styled button with the same Unique/Data would - no changes
// needed anywhere else in the codebase.
var telebotCallbackRegex = regexp.MustCompile(`^\f([-\w]+)(\|(.+))?$`)

func TestStyledCallbackData_MatchesTelebotsOwnRoutingFormat(t *testing.T) {
	cases := []struct {
		name       string
		btn        telebot.Btn
		wantUnique string
		wantData   string
	}{
		{
			name:       "unique with a single data argument",
			btn:        telebot.Btn{Unique: "road_encounter", Data: "attack"},
			wantUnique: "road_encounter",
			wantData:   "attack",
		},
		{
			name:       "unique with multiple pipe-joined arguments",
			btn:        telebot.Btn{Unique: "road_encounter", Data: "attack|enc123|raid456"},
			wantUnique: "road_encounter",
			wantData:   "attack|enc123|raid456",
		},
		{
			name:       "unique with no data at all",
			btn:        telebot.Btn{Unique: "launch_interceptor"},
			wantUnique: "launch_interceptor",
			wantData:   "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := styledCallbackData(c.btn)

			m := telebotCallbackRegex.FindStringSubmatch(got)
			if m == nil {
				t.Fatalf("styledCallbackData(%+v) = %q, does not match telebot's own callback regex", c.btn, got)
			}
			if m[1] != c.wantUnique {
				t.Errorf("expected unique %q, got %q (from %q)", c.wantUnique, m[1], got)
			}
			if m[3] != c.wantData {
				t.Errorf("expected data %q, got %q (from %q)", c.wantData, m[3], got)
			}
		})
	}
}

func TestStyledCallbackData_EmptyForButtonsWithNoUnique(t *testing.T) {
	// A URL button (or any button that isn't a callback button at all)
	// must not get a synthesized callback_data - that would make
	// Telegram treat it as a callback button instead of a URL button.
	got := styledCallbackData(telebot.Btn{URL: "https://example.com"})
	if got != "" {
		t.Errorf("expected empty callback_data for a button with no Unique, got %q", got)
	}
}

func TestSendStyled_WireFormatIncludesStyleField(t *testing.T) {
	btnAttack := Styled(telebot.Btn{Unique: "road_encounter", Text: "⚔️ Attack", Data: "attack|enc1|raid1"}, StyleDanger)
	btnContinue := Styled(telebot.Btn{Unique: "road_encounter", Text: "➡️ Continue", Data: "continue|enc1|raid1"}, StyleSuccess)

	raw, err := marshalRowsForTest([][]StyledBtn{{btnAttack, btnContinue}})
	if err != nil {
		t.Fatalf("marshalRowsForTest: %v", err)
	}

	var decoded [][]map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decoding wire JSON: %v\nraw: %s", err, raw)
	}

	if len(decoded) != 1 || len(decoded[0]) != 2 {
		t.Fatalf("expected one row of two buttons, got %+v", decoded)
	}

	attack := decoded[0][0]
	if attack["style"] != "danger" {
		t.Errorf("expected attack button style 'danger', got %v", attack["style"])
	}
	if attack["text"] != "⚔️ Attack" {
		t.Errorf("expected attack button text preserved, got %v", attack["text"])
	}
	if attack["callback_data"] != "\froad_encounter|attack|enc1|raid1" {
		t.Errorf("expected correctly-formatted callback_data, got %v", attack["callback_data"])
	}

	cont := decoded[0][1]
	if cont["style"] != "success" {
		t.Errorf("expected continue button style 'success', got %v", cont["style"])
	}
}

func TestSendStyled_URLButtonHasNoCallbackDataOrStyleLeak(t *testing.T) {
	btnLink := Styled(telebot.Btn{Text: "🔗 Learn More", URL: "https://example.com"}, StylePrimary)

	raw, err := marshalRowsForTest([][]StyledBtn{{btnLink}})
	if err != nil {
		t.Fatalf("marshalRowsForTest: %v", err)
	}

	var outer [][]map[string]interface{}
	if err := json.Unmarshal(raw, &outer); err != nil {
		t.Fatalf("decoding wire JSON: %v\nraw: %s", err, raw)
	}
	rows := outer[0]

	if _, hasCallback := rows[0]["callback_data"]; hasCallback {
		t.Errorf("expected no callback_data field for a URL button, got %v", rows[0]["callback_data"])
	}
	if rows[0]["url"] != "https://example.com" {
		t.Errorf("expected the URL to be preserved, got %v", rows[0]["url"])
	}
	if rows[0]["style"] != "primary" {
		t.Errorf("expected style to still apply to a URL button, got %v", rows[0]["style"])
	}
}

func TestButtonStyleConstants_MatchTelegramBotAPI94Values(t *testing.T) {
	// Bot API 9.4's actual accepted values, per
	// https://core.telegram.org/bots/api#inlinekeyboardbutton - if these
	// ever drift from what Telegram expects, every styled button in the
	// game silently renders with no color at all (an unrecognized style
	// value is simply ignored, not rejected), so pin them explicitly.
	if StyleDanger != "danger" {
		t.Errorf("StyleDanger = %q, want \"danger\"", StyleDanger)
	}
	if StyleSuccess != "success" {
		t.Errorf("StyleSuccess = %q, want \"success\"", StyleSuccess)
	}
	if StylePrimary != "primary" {
		t.Errorf("StylePrimary = %q, want \"primary\"", StylePrimary)
	}
}
