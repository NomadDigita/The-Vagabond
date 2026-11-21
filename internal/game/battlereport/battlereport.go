package battlereport

import (
	"fmt"
	"strings"
)

func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

type UnitTally struct {
	Emoji string
	Label string
	Count int
}

const maxLossGlyphs = 40

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

type Outcome int

const (
	OutcomeOngoing Outcome = iota
	OutcomeAttackerWon
	OutcomeDefenderWon
	OutcomeDraw
)

type Round struct {
	Number       int
	AttackerName string
	DefenderName string

	AttackerComposition []UnitTally
	DefenderComposition []UnitTally

	AttackerLosses []UnitTally
	DefenderLosses []UnitTally

	Outcome       Outcome
	LootLines     []string
	LootCollector string

	AttackerNotes []string
	DefenderNotes []string
}

func Render(r Round) string {
	attacker := htmlEscape(r.AttackerName)
	defender := htmlEscape(r.DefenderName)

	roundLabel := fmt.Sprintf("🔮 <b>ROUND %d</b>", r.Number)
	if r.Outcome != OutcomeOngoing {
		roundLabel = "🔮 <b>FINAL CLASH</b>"
	}

	msg := fmt.Sprintf(
		"⚔️ <b>%s</b>  🆚  🛡️ <b>%s</b>\n%s\n\n"+
			"⚔️ %s: <code>%s</code>\n"+
			"🛡️ %s: <code>%s</code>\n\n"+
			"💥 <b>Losses:</b>\n"+
			"⚔️ %s: <code>%s</code>\n"+
			"🛡️ %s: <code>%s</code>",
		attacker, defender, roundLabel,
		attacker, renderComposition(r.AttackerComposition),
		defender, renderComposition(r.DefenderComposition),
		attacker, renderLossEmojis(r.AttackerLosses),
		defender, renderLossEmojis(r.DefenderLosses),
	)

	if len(r.AttackerNotes) > 0 {
		msg += "\n" + htmlEscape(strings.Join(r.AttackerNotes, "\n"))
	}
	if len(r.DefenderNotes) > 0 {
		msg += "\n" + htmlEscape(strings.Join(r.DefenderNotes, "\n"))
	}

	switch r.Outcome {
	case OutcomeOngoing:
		msg += "\n\n⏳ <i>Next skirmish round resolves on the next clock tick.</i>"
	case OutcomeAttackerWon:
		msg += fmt.Sprintf("\n\n🏆 <b>%s WON!</b> 🎉", attacker)
		if len(r.LootLines) > 0 {
			msg += fmt.Sprintf("\n\n📦 <b>Battle Debris:</b>\n<blockquote expandable>%s</blockquote>\n🏅 %s collected the debris.",
				htmlEscape(strings.Join(r.LootLines, "\n")), htmlEscape(r.LootCollector))
		}
	case OutcomeDefenderWon:
		msg += fmt.Sprintf("\n\n🏆 <b>%s WON!</b> 🎉", defender)
		if len(r.LootLines) > 0 {
			msg += fmt.Sprintf("\n\n📦 <b>Battle Debris:</b>\n<blockquote expandable>%s</blockquote>\n🏅 %s collected the debris.",
				htmlEscape(strings.Join(r.LootLines, "\n")), htmlEscape(r.LootCollector))
		}
	case OutcomeDraw:
		msg += "\n\n🤝 <b>DRAW!</b> Neither side could break the other. Forces disengage and retreat."
	}

	return msg
}
