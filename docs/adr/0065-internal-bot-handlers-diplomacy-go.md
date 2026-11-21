# ADR 0065: Decisions from `internal/bot/handlers/diplomacy.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/diplomacy.go` and moved here so the source no longer carries them. Each entry is the original comment.

## getMyClanForDiplomacy mirrors federation.go's private getMyClan helper

Removed from `internal/bot/handlers/diplomacy.go:21`.

~~~~text
getMyClanForDiplomacy mirrors federation.go's private getMyClan helper
(can't call that one directly - it's a method on *FederationHandler,
a different receiver type).
~~~~


## Same pattern as combat.go's Road Contact prompts: a pact response

Removed from `internal/bot/handlers/diplomacy.go:76`.

~~~~text
Same pattern as combat.go's Road Contact prompts: a pact response
is a real accept-vs-reject decision, so it gets pulled out of the
shared panel keyboard and sent as its own colored message below
(see keyboards/styled.go) instead of an unstyled button mixed into
the panel.
~~~~


## Pending: only the RECEIVING Clan Leader (not the proposer) gets accept/reject buttons.

Removed from `internal/bot/handlers/diplomacy.go:101`.

~~~~text
Pending: only the RECEIVING Clan Leader (not the proposer) gets accept/reject buttons.
~~~~


## proposePact handles both /ally and /nap - they differ only in

Removed from `internal/bot/handlers/diplomacy.go:143`.

~~~~text
proposePact handles both /ally and /nap - they differ only in
pactType, so the actual proposal logic is shared.
~~~~


## HandleBreakPact (/break_pact [clan_name]) lets either Clan Leader end an

Removed from `internal/bot/handlers/diplomacy.go:253`.

~~~~text
HandleBreakPact (/break_pact [clan_name]) lets either Clan Leader end an
active pact unilaterally - diplomacy shouldn't be a permanent trap.
~~~~


## pactQueryer is satisfied by both *sql.DB and *sql.Tx, so combat.go's

Removed from `internal/bot/handlers/diplomacy.go:295`.

~~~~text
pactQueryer is satisfied by both *sql.DB and *sql.Tx, so combat.go's
raid-launch check (already inside a transaction) and any future
caller outside one can both use HasActivePact.
~~~~


## HasActivePact reports whether two Clans currently have an active

Removed from `internal/bot/handlers/diplomacy.go:302`.

~~~~text
HasActivePact reports whether two Clans currently have an active
alliance or non-aggression pact - used by combat.go's raid launch to
block attacks between diplomatically protected Clans.
~~~~
