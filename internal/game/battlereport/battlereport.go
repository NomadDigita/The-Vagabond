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

// htmlEscape makes a string safe to place inside a Telegram HTML-mode
// message. Duplicated locally (rather than imported from bot/handlers)
// so this package stays dependency-free, per the repo's package-per-
// feature convention. AttackerName/DefenderName ultimately come from
// player-set encampment names, so this MUST run on them before they're
// interpolated into the report.
func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")
	return r.Replace(s)
}

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
	Outcome       Outcome
	LootLines     []string // e.g. []string{"♻️ 396000 Scrap"}
	LootCollector string   // who collected the battle debris (winner's name)

	// Context notes shown right after each side's composition line -
	// Defense Grid turret levels, Guardians/Observers, Nuclear Shields,
	// and a Hero/Warlord's active superpower. These already factor into
	// the actual combat math the tick engine computes (see engine.go's
	// turretDefenseBonus/guardianBonus/observerBonus/defenderShields),
	// but before this field existed, none of it ever reached the
	// player-facing report - recon would show a detailed Defense Grid
	// and a Warlord, then the actual battle outcome read like a bare
	// soldiers-vs-soldiers skirmish with no trace of either. Optional:
	// an empty slice renders nothing extra.
	AttackerNotes []string
	DefenderNotes []string
}

// Render produces the full SpaceHunt-style report text for one round,
// using Telegram HTML formatting: bold for names/headers, monospace for
// unit tallies so columns line up, and an expandable blockquote for the
// final loot line so a long battle report doesn't crowd out the verdict.
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
