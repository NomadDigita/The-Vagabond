# ADR 0228: Decisions from `migrations/023_spacehunt_phase7_garrison.sql`

Status: Accepted

These decisions were written as comments in `migrations/023_spacehunt_phase7_garrison.sql` and moved here so the source no longer carries them. Each entry is the original comment.

## Lets a player manually lock a portion of their Soldiers/Mechs into a

Removed from `migrations/023_spacehunt_phase7_garrison.sql:10`.

~~~~text
Lets a player manually lock a portion of their Soldiers/Mechs into a
permanent home garrison. Garrisoned units:
- Still count fully toward defense (workshop_inventory.soldiers/mechs
is unchanged - garrison is a *reservation*, not a separate pool).
- Are excluded from the *draftable* total shown/enforced when staging
a campaign (see combat.go availSoldiers/availMechs), so a player
can't accidentally send their entire home defense out on a raid.
- Can be withdrawn (un-reserved) at any time via the Garrison panel.
~~~~
