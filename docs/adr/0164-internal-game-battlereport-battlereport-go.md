# ADR 0164: Decisions from `internal/game/battlereport/battlereport.go`

Status: Accepted

These decisions were written as comments in `internal/game/battlereport/battlereport.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Package battlereport renders combat results in SpaceHunt's exact visual

Removed from `internal/game/battlereport/battlereport.go:1`.

~~~~text
Package battlereport renders combat results in SpaceHunt's exact visual
style: an "⚔️ X VS 🛡️ Y" header, per-round unit composition lines,
losses shown as repeated unit emoji (not just numbers), and a final
"Battle debris" + winner announcement. This package is pure
presentation - it has no DB or telebot dependency and does not decide
any combat outcome; the tick engine computes all math and casualties,
then hands the results here purely for rendering.
~~~~


## htmlEscape makes a string safe to place inside a Telegram HTML-mode

Removed from `internal/game/battlereport/battlereport.go:15`.

~~~~text
htmlEscape makes a string safe to place inside a Telegram HTML-mode
message. Duplicated locally (rather than imported from bot/handlers)
so this package stays dependency-free, per the repo's package-per-
feature convention. AttackerName/DefenderName ultimately come from
player-set encampment names, so this MUST run on them before they're
interpolated into the report.
~~~~


## maxLossGlyphs caps how many repeated unit emoji get rendered per side per

Removed from `internal/game/battlereport/battlereport.go:33`.

~~~~text
maxLossGlyphs caps how many repeated unit emoji get rendered per side per
round, so a battle with thousands of casualties doesn't produce an
unreadable (or Telegram-length-limit-breaking) wall of emoji.
~~~~


## Standing forces BEFORE this round's losses are applied - i.e. what

Removed from `internal/game/battlereport/battlereport.go:99`.

~~~~text
Standing forces BEFORE this round's losses are applied - i.e. what
marched into the fight.
~~~~


## Context notes shown right after each side's composition line -

Removed from `internal/game/battlereport/battlereport.go:113`.

~~~~text
Context notes shown right after each side's composition line -
Defense Grid turret levels, Guardians/Observers, Nuclear Shields,
and a Hero/Warlord's active superpower. These already factor into
the actual combat math the tick engine computes (see engine.go's
turretDefenseBonus/guardianBonus/observerBonus/defenderShields),
but before this field existed, none of it ever reached the
player-facing report - recon would show a detailed Defense Grid
and a Warlord, then the actual battle outcome read like a bare
soldiers-vs-soldiers skirmish with no trace of either. Optional:
an empty slice renders nothing extra.
~~~~


## Render produces the full SpaceHunt-style report text for one round,

Removed from `internal/game/battlereport/battlereport.go:127`.

~~~~text
Render produces the full SpaceHunt-style report text for one round,
using Telegram HTML formatting: bold for names/headers, monospace for
unit tallies so columns line up, and an expandable blockquote for the
final loot line so a long battle report doesn't crowd out the verdict.
~~~~
