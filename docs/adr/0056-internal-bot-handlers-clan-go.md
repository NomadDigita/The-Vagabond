# ADR 0056: Decisions from `internal/bot/handlers/clan.go`

Status: Accepted

These decisions were written as comments in `internal/bot/handlers/clan.go` and moved here so the source no longer carries them. Each entry is the original comment.

## canManageClan reports whether a role grants alliance-management

Removed from `internal/bot/handlers/clan.go:31`.

~~~~text
canManageClan reports whether a role grants alliance-management
permissions (reviewing applications, kicking Soldiers, setting the
icon/description). Introduced to fix BUGS_AND_INCONSISTENCIES.md
bug #4: HandlePromoteMemberCallback set role = "Co-Leader" but
nothing ever checked for it, making Promote a purely cosmetic
no-op. Co-Leader now grants real, but deliberately narrower than
Leader, authority - see the callers of this helper for exactly
which actions it unlocks. Declaring war, renaming the clan, and
promoting/demoting/kicking a Leader or another Co-Leader remain
Leader-only: those are either costly/structural (rename costs
Crystal, war commits the whole clan for 48h) or would let a
Co-Leader remove the person who promoted them.
~~~~


## Co-Leaders can manage rank-and-file Soldiers but not

Removed from `internal/bot/handlers/clan.go:419`.

~~~~text
Co-Leaders can manage rank-and-file Soldiers but not
the Leader or fellow Co-Leaders, and can't promote -
only the Leader hands out the Co-Leader rank.
~~~~


## BUGFIX: this used to DELETE by targetID alone, with no check that

Removed from `internal/bot/handlers/clan.go:461`.

~~~~text
BUGFIX: this used to DELETE by targetID alone, with no check that
the target actually belongs to the acting leader's own clan. A
stale "Kick" button (e.g. the member left and joined a different
clan between opening Manage Members and tapping Kick) could
remove someone from an unrelated clan's roster entirely. The
clan_id match makes this a no-op instead of a cross-clan kick.
~~~~


## BUGFIX: same cross-clan gap as HandleKickMemberCallback above -

Removed from `internal/bot/handlers/clan.go:500`.

~~~~text
BUGFIX: same cross-clan gap as HandleKickMemberCallback above -
verify the target is actually in the acting leader's own clan
before promoting them, rather than trusting targetID alone.
~~~~


## randomAnimalIcons is the pool /guild_icon draws from, matching

Removed from `internal/bot/handlers/clan.go:552`.

~~~~text
randomAnimalIcons is the pool /guild_icon draws from, matching
SpaceHunt's "change your guild icon to another random animal" feature.
~~~~


## HandleGuildDescription (/guild_description [text]) sets the clan's

Removed from `internal/bot/handlers/clan.go:671`.

~~~~text
HandleGuildDescription (/guild_description [text]) sets the clan's
description, shown on the recruitment /board.
~~~~


## HandleBoard (/board) is the recruitment post board - clans currently

Removed from `internal/bot/handlers/clan.go:705`.

~~~~text
HandleBoard (/board) is the recruitment post board - clans currently
open to recruiting, with their icon and description, distinct from
/clans (which is the pure ranking list).
~~~~


## HandleCreateClanCommand establishes a clan with a REAL custom name

Removed from `internal/bot/handlers/clan.go:752`.

~~~~text
HandleCreateClanCommand establishes a clan with a REAL custom name
(payload), matching SpaceHunt's naming freedom instead of an
auto-generated placeholder.
~~~~


## HandleDeclareClanWarCallback creates a REAL tracked war (48h duration,

Removed from `internal/bot/handlers/clan.go:883`.

~~~~text
HandleDeclareClanWarCallback creates a REAL tracked war (48h duration,
live score accumulated from actual raid outcomes between the two clans'
members, resolved by the tick engine) - replacing the old stub that just
sent a one-time notification with zero mechanical effect.
~~~~


## clanDonateMaxLen bounds /clan_donate's @username argument length

Removed from `internal/bot/handlers/clan.go:944`.

~~~~text
clanDonateMaxLen bounds /clan_donate's @username argument length
purely as a sanity check before it ever reaches a query - Telegram
usernames are capped at 32 characters, so anything longer is
certainly a typo/garbage, not a real target.
~~~~


## HandleClanDonateCommand (/clan_donate <resource> <amount> <@username>)

Removed from `internal/bot/handlers/clan.go:950`.

~~~~text
HandleClanDonateCommand (/clan_donate <resource> <amount> <@username>)
shows a Confirm/Cancel preview before transferring resources from the
caller's own stockpile to a fellow clan member's - 2026-08-05
addition, closing a real gap flagged by the project owner ("there's
no way to transport resources to clan"). Deliberately a direct
peer-to-peer transfer between two members' existing `resources` rows
rather than a new clan-wide treasury/vault concept - the latter would
need a new table, a withdrawal-authority model (who can spend from
it, and how that interacts with Leader/Co-Leader/Soldier roles), and
its own audit trail, which is a materially bigger design than "let me
help a clanmate who's short on Crystal right now." Nothing here
touches the database - the actual transfer only happens if the
player taps Confirm, via HandleClanDonateConfirmCallback calling
doClanDonate.
~~~~


## HandleClanDonateConfirmCallback fires when a player taps "✅ Confirm

Removed from `internal/bot/handlers/clan.go:1026`.

~~~~text
HandleClanDonateConfirmCallback fires when a player taps "✅ Confirm
Donation" on the card HandleClanDonateCommand rendered. Re-derives
the caller's own clan/balance server-side rather than trusting
anything about the sender from callback_data (only the resource,
amount, and target ID are read from it - all revalidated fresh
inside doClanDonate).
~~~~


## doClanDonate is the testable core of HandleClanDonateConfirmCallback:

Removed from `internal/bot/handlers/clan.go:1058`.

~~~~text
doClanDonate is the testable core of HandleClanDonateConfirmCallback:
transfers amount of resource from senderUserID's stockpile to
targetUserID's, revalidating from scratch that both are still real
clanmates in the SAME clan (membership could have changed between the
card being shown and Confirm being tapped) and that senderUserID
still has enough of resource. Storage-cap clamps the recipient's gain
the same way every other resource-gain path in this codebase does
(see storagecap.Clamp's callers in exchange.go).
~~~~
