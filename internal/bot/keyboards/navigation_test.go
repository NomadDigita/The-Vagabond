package keyboards

import (
	"testing"

	"gopkg.in/telebot.v3"
)

// allowedSharedText is the small set of button labels that are
// deliberately identical across multiple keyboards, because they're
// meant to route to the exact same handler regardless of which child
// menu you tapped it from (e.g. every child keyboard's "⬅️ Back to HQ"
// registers once, in main.go, to onboarding.HandleStart). Anything NOT
// in this set that appears on more than one keyboard is a real bug: two
// different buttons with identical text can only ever call one handler,
// so whichever bot.Handle registration in main.go happens to be
// evaluated silently wins and the other button does the wrong thing.
var allowedSharedText = map[string]bool{
	"⬅️ Back to HQ": true,
}

// allKeyboards must be kept in sync with every *Navigation function in
// this package - deliberately a manual list (not reflection) so adding a
// new keyboard function forces a conscious decision to add it here too.
func allKeyboards() map[string]*telebot.ReplyMarkup {
	return map[string]*telebot.ReplyMarkup{
		"MainNavigation":     MainNavigation(),
		"CampNavigation":     CampNavigation(),
		"CombatNavigation":   CombatNavigation(),
		"EconomyNavigation":  EconomyNavigation(),
		"WorkshopNavigation": WorkshopNavigation(),
		"JobsNavigation":     JobsNavigation(),
		"AdminNavigation":    AdminNavigation(),
	}
}

func TestNoButtonTextCollisionsAcrossKeyboards(t *testing.T) {
	seenOn := map[string][]string{} // button text -> which keyboards it appeared on

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

// TestNoButtonTextDuplicatedWithinASingleKeyboard catches the simpler,
// more obviously broken case: the same button appearing twice on one
// keyboard (a copy-paste mistake), which telebot would also silently
// only ever route to whichever registration wins.
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

// TestJobsNavigationHasBackButton is a small, specific regression guard:
// every child keyboard in this package must offer a way back to
// MainNavigation, and it's easy to forget when adding a new one (there's
// no compiler check for "did I add a Back button").
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

// TestMainNavigationHasOddJobsEntry guards the mother-keyboard side of
// the mother/child pair - Jobs must be reachable from the root menu, not
// just have a child keyboard that nothing links to.
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
