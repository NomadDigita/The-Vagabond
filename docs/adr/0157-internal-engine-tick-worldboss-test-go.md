# ADR 0157: Decisions from `internal/engine/tick/worldboss_test.go`

Status: Accepted

These decisions were written as comments in `internal/engine/tick/worldboss_test.go` and moved here so the source no longer carries them. Each entry is the original comment.

## Just confirm it's a recognized key from the catalog - the

Removed from `internal/engine/tick/worldboss_test.go:61`.

~~~~text
Just confirm it's a recognized key from the catalog - the
exact rarity (Epic vs Legendary) is randomized, both are
acceptable outcomes for a guaranteed drop.
~~~~


## TestPayoutWorldBossLoot_NonGauntletDoesNotGuaranteeDrop is a

Removed from `internal/engine/tick/worldboss_test.go:86`.

~~~~text
TestPayoutWorldBossLoot_NonGauntletDoesNotGuaranteeDrop is a
statistical sanity check: a non-Gauntlet kill should NOT guarantee a
drop the way a Gauntlet kill does - run enough independent kills
that at least one produces no drop at all (the 90% no-drop case at
the existing 2%/8% roll rates).
~~~~
