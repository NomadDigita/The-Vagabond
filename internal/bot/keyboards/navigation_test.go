package keyboards

import (
	"testing"

	"gopkg.in/telebot.v3"
)

var allowedSharedText = map[string]bool{
	"⬅️ Back to HQ": true,
}

func allKeyboards() map[string]*telebot.ReplyMarkup {
	return map[string]*telebot.ReplyMarkup{
		"MainNavigation":     MainNavigation(),
		"CampNavigation":     CampNavigation(),
		"CombatNavigation":   CombatNavigation(),
		"EconomyNavigation":  EconomyNavigation(),
		"WorkshopNavigation": WorkshopNavigation(),
		"JobsNavigation":     JobsNavigation(),
		"AdvisorsNavigation": AdvisorsNavigation(),
		"ProfileNavigation":  ProfileNavigation(),
		"AdminNavigation":    AdminNavigation(),
	}
}

func TestNoButtonTextCollisionsAcrossKeyboards(t *testing.T) {
	seenOn := map[string][]string{}

	for name, kb := range allKeyboards() {
		for _, row := range kb.ReplyKeyboard {
			for _, btn := range row {
				seenOn[btn.Text] = append(seenOn[btn.Text], name)
			}
		}
	}

	for text, appearedOn := range seenOn {
		if len(appearedOn) <= 1 {
			continue
		}
		if allowedSharedText[text] {
			continue
		}
		t.Errorf("button text %q appears on multiple keyboards (%v) but isn't in allowedSharedText - "+
			"telebot routes reply-keyboard taps by exact text match globally, so only one of these will ever actually fire",
			text, appearedOn)
	}
}

func TestNoButtonTextDuplicatedWithinASingleKeyboard(t *testing.T) {
	for name, kb := range allKeyboards() {
		seen := map[string]bool{}
		for _, row := range kb.ReplyKeyboard {
			for _, btn := range row {
				if seen[btn.Text] {
					t.Errorf("%s has button text %q more than once", name, btn.Text)
				}
				seen[btn.Text] = true
			}
		}
	}
}

func TestJobsNavigationHasBackButton(t *testing.T) {
	kb := JobsNavigation()
	found := false
	for _, row := range kb.ReplyKeyboard {
		for _, btn := range row {
			if btn.Text == "⬅️ Back to HQ" {
				found = true
			}
		}
	}
	if !found {
		t.Error("JobsNavigation is missing the standard '⬅️ Back to HQ' button")
	}
}

func TestMainNavigationHasOddJobsEntry(t *testing.T) {
	kb := MainNavigation()
	found := false
	for _, row := range kb.ReplyKeyboard {
		for _, btn := range row {
			if btn.Text == "🛠️ Odd Jobs" {
				found = true
			}
		}
	}
	if !found {
		t.Error("MainNavigation is missing the '🛠️ Odd Jobs' entry point into JobsNavigation")
	}
}

func TestAdvisorsNavigationHasBackButton(t *testing.T) {
	kb := AdvisorsNavigation()
	found := false
	for _, row := range kb.ReplyKeyboard {
		for _, btn := range row {
			if btn.Text == "⬅️ Back to HQ" {
				found = true
			}
		}
	}
	if !found {
		t.Error("AdvisorsNavigation is missing the standard '⬅️ Back to HQ' button")
	}
}

func TestMainNavigationHasAdvisorsEntry(t *testing.T) {
	kb := MainNavigation()
	found := false
	for _, row := range kb.ReplyKeyboard {
		for _, btn := range row {
			if btn.Text == "🎓 AI Advisors" {
				found = true
			}
		}
	}
	if !found {
		t.Error("MainNavigation is missing the '🎓 AI Advisors' entry point into AdvisorsNavigation")
	}
}

func TestProfileNavigationHasBackButton(t *testing.T) {
	kb := ProfileNavigation()
	found := false
	for _, row := range kb.ReplyKeyboard {
		for _, btn := range row {
			if btn.Text == "⬅️ Back to HQ" {
				found = true
			}
		}
	}
	if !found {
		t.Error("ProfileNavigation is missing the standard '⬅️ Back to HQ' button")
	}
}

func TestMainNavigationHasProfileEntry(t *testing.T) {
	kb := MainNavigation()
	found := false
	for _, row := range kb.ReplyKeyboard {
		for _, btn := range row {
			if btn.Text == "📊 Player Profile" {
				found = true
			}
		}
	}
	if !found {
		t.Error("MainNavigation is missing the '📊 Player Profile' entry point into ProfileNavigation")
	}
}
