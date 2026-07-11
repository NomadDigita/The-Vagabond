// Package battlereport renders combat results in SpaceHunt's exact visual
// style: an "⚔️ X VS 🛡️ Y" header, per-round unit composition lines,
// losses shown as repeated unit emoji (not just numbers), and a final
// "Battle debris" + winner announcement. This package is pure
// presentation - it has no DB or telebot dependency and does not decide
// any combat outcome; the tick engine computes all math and casualties,
// then hands the results here purely for rendering.
package battlereport

import (
	"fmt"
	"strings"
)

// UnitTally is one line item in a composition or loss listing.
type UnitTally struct {
	Emoji string
	Label string
	Count int
}

// maxLossGlyphs caps how many repeated unit emoji get rendered per side per
// round, so a battle with thousands of casualties doesn't produce an
// unreadable (or Telegram-length-limit-breaking) wall of emoji.
const maxLossGlyphs = 40

// renderComposition renders a side's standing forces, SpaceHunt-style:
// "37🚁 Bomber. 14💥 Destroyer." Units with a zero count are omitted.
func renderComposition(units []UnitTally) string {
	var b strings.Builder
	any := false
	for _, u := range units {
		if u.Count <= 0 {
			continue
		}
		fmt.Fprintf(&b, "%d%s %s. ", u.Count, u.Emoji, u.Label)
		any = true
	}
	if !any {
		return "— wiped out —"
	}
	return strings.TrimSpace(b.String())
}

// renderLossEmojis renders losses as repeated emoji glyphs (matching
// SpaceHunt's visual casualty report) rather than plain numbers, capped at
// maxLossGlyphs with an overflow counter.
func renderLossEmojis(units []UnitTally) string {
	var b strings.Builder
	shown := 0
	total := 0
	for _, u := range units {
		total += u.Count
	}
	if total == 0 {
		return "— none —"
	}

	for _, u := range units {
		for i := 0; i < u.Count; i++ {
			if shown >= maxLossGlyphs {
				fmt.Fprintf(&b, " (+%d more)", total-shown)
				return b.String()
			}
			b.WriteString(u.Emoji)
			shown++
		}
	}
	return b.String()
}

// Outcome describes how a final round resolved.
type Outcome int

const (
	OutcomeOngoing Outcome = iota
	OutcomeAttackerWon
	OutcomeDefenderWon
	OutcomeDraw
)

// Round holds everything needed to render one combat round report.
type Round struct {
	Number       int
	AttackerName string
	DefenderName string

	// Standing forces BEFORE this round's losses are applied - i.e. what
	// marched into the fight.
	AttackerComposition []UnitTally
	DefenderComposition []UnitTally

	// Units lost THIS round only.
	AttackerLosses []UnitTally
	DefenderLosses []UnitTally

	// Final-round-only fields:
	Outcome        Outcome
	LootLines      []string // e.g. []string{"♻️ 396000 Scrap"}
	LootCollector  string   // who collected the battle debris (winner's name)
}

// Render produces the full SpaceHunt-style report text for one round.
func Render(r Round) string {
	roundLabel := fmt.Sprintf("💎 ROUND %d:", r.Number)
	if r.Outcome != OutcomeOngoing {
		roundLabel = "💎 END:"
	}

	msg := fmt.Sprintf(
		"⚔️ %s  VS  🛡️ %s\n%s\n\n"+
			"⚔️ %s: %s\n"+
			"🛡️ %s: %s\n\n"+
			"💥 Losses:\n"+
			"⚔️ %s: %s\n"+
			"🛡️ %s: %s",
		r.AttackerName, r.DefenderName, roundLabel,
		r.AttackerName, renderComposition(r.AttackerComposition),
		r.DefenderName, renderComposition(r.DefenderComposition),
		r.AttackerName, renderLossEmojis(r.AttackerLosses),
		r.DefenderName, renderLossEmojis(r.DefenderLosses),
	)

	switch r.Outcome {
	case OutcomeOngoing:
		msg += "\n\n⏳ Next skirmish round resolves on the next clock tick."
	case OutcomeAttackerWon:
		msg += fmt.Sprintf("\n\n🏆 %s WON!", r.AttackerName)
		if len(r.LootLines) > 0 {
			msg += fmt.Sprintf("\n\n📦 Battle Debris: %s\n%s collected the debris.", strings.Join(r.LootLines, " "), r.LootCollector)
		}
	case OutcomeDefenderWon:
		msg += fmt.Sprintf("\n\n🏆 %s WON!", r.DefenderName)
		if len(r.LootLines) > 0 {
			msg += fmt.Sprintf("\n\n📦 Battle Debris: %s\n%s collected the debris.", strings.Join(r.LootLines, " "), r.LootCollector)
		}
	case OutcomeDraw:
		msg += "\n\n🤝 DRAW! Neither side could break the other. Forces disengage and retreat."
	}

	return msg
}
