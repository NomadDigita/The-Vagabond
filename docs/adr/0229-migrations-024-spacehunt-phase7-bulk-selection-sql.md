# ADR 0229: Decisions from `migrations/024_spacehunt_phase7_bulk_selection.sql`

Status: Accepted

These decisions were written as comments in `migrations/024_spacehunt_phase7_bulk_selection.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## One "step size" per draft, cycled via a Step: x1 -> x10 -> x100 -> MAX

Removed from `migrations/024_spacehunt_phase7_bulk_selection.sql:10`.

~~~~text
One "step size" per draft, cycled via a Step: x1 -> x10 -> x100 -> MAX
button row on the draft customizer HUD. Every existing per-unit +/-
button then moves this many units per tap (MAX snaps straight to the
draftable cap for that unit type), instead of requiring one tap per
unit for a large army.
~~~~
