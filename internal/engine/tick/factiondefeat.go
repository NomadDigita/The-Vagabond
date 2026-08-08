package tick

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"

	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
)

// RARE_WORLD_FEATURES_PLAN.md Phase 3: Legacy Epitaphs.
//
// The game's first permanent-defeat state, deliberately scoped to AI
// factions only (see the plan doc's design principle 3 - real players
// are never wiped by anything in this feature set). This file
// deliberately does NOT touch resolveRaidCombats (engine.go's large,
// well-tested combat resolution function) at all - checkForFactionDefeat
// is a brand new, independent, read-then-conditionally-write tick phase
// that runs AFTER combat resolution each tick and asks a simple
// question per AI faction: is this faction now at (near-)zero
// resources AND (near-)zero garrison? If yes, and it wasn't already
// marked defeated, mark it now. See the plan doc's section 3.1 for the
// full "why this is safe to add" rationale.

// factionDefeatResourceThreshold reuses engine.go's own
// economicCollapseWarningThreshold (50.0) rather than defining a
// second, potentially-drifting constant with the same intended
// meaning - "critically depleted" and "totally defeated" should stay
// in lockstep if that number is ever tuned.
const factionDefeatResourceThreshold = economicCollapseWarningThreshold

// epitaphTemplates are the flavor templates a newly-defeated faction's
// epitaph is drawn from. %s is the faction's own name; %s (second) is
// the finishing attacker's name, or "the wasteland itself" when no
// raid history exists (see epitaphAttackerFlavor below) - e.g. a
// faction that starved out from a weather/tax pass instead of combat.
var epitaphTemplates = []string{
	"Here fell %s, broken at last by %s. Its banners are dust; its ruins remain.",
	"%s made its final stand here, and lost - to %s. The wasteland reclaims what it built.",
	"No garrison remains to defend %s. %s saw to that. Only silence answers now.",
	"The fires of %s have gone cold, extinguished by %s. A monument to nothing survives.",
	"%s is no more. %s delivered the final blow. Scavengers already pick at the bones.",
}

// checkForFactionDefeat is the "faction_defeat_check" tick phase,
// running after raid_combat each tick (see engine.go's phase list).
// For every AI faction not already marked defeated: total resources
// (scrap+metal+crystal+dollars) at or below
// factionDefeatResourceThreshold AND garrison (soldiers+mechs) at or
// below zero, BOTH conditions - not just resources, so a faction that's
// merely poor but still garrisoned (or vice versa) isn't prematurely
// retired.
func (e *Engine) checkForFactionDefeat(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT e.id, e.name, c.region
		FROM encampments e
		JOIN coordinates c ON c.id = e.coordinate_id
		LEFT JOIN resources r ON r.encampment_id = e.id
		LEFT JOIN workshop_inventory w ON w.encampment_id = e.id
		WHERE e.is_ai_faction = TRUE AND e.is_defeated = FALSE
		  AND COALESCE(r.scrap, 0) + COALESCE(r.metal, 0) + COALESCE(r.crystal, 0) + COALESCE(r.dollars, 0) <= $1
		  AND COALESCE(w.soldiers, 0) + COALESCE(w.mechs, 0) <= 0`,
		factionDefeatResourceThreshold)
	if err != nil {
		return fmt.Errorf("querying for defeated factions: %w", err)
	}

	type defeated struct{ id, name, region string }
	var toDefeat []defeated
	for rows.Next() {
		var d defeated
		if scanErr := rows.Scan(&d.id, &d.name, &d.region); scanErr == nil {
			toDefeat = append(toDefeat, d)
		}
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		rows.Close()
		return fmt.Errorf("reading defeated faction rows: %w", rowsErr)
	}
	rows.Close()

	for _, d := range toDefeat {
		attackerName, isRealAttacker, attackerUserID := e.mostRecentAttacker(ctx, tx, d.id)

		attackerFlavor := attackerName
		if attackerFlavor == "" {
			attackerFlavor = "the wasteland itself"
		}
		template := epitaphTemplates[rand.Intn(len(epitaphTemplates))]
		epitaph := fmt.Sprintf(template, d.name, attackerFlavor)

		if _, err := tx.ExecContext(ctx, `
			UPDATE encampments SET is_defeated = TRUE, defeated_at = CURRENT_TIMESTAMP, epitaph = $1
			WHERE id = $2`, epitaph, d.id); err != nil {
			return fmt.Errorf("marking faction %s defeated: %w", d.id, err)
		}

		broadcast := fmt.Sprintf("🪦 A FACTION HAS FALLEN: %s is no more. %s", d.name, epitaph)
		if err := notifications.QueueToRegion(ctx, tx, d.region, broadcast, "general"); err != nil {
			return fmt.Errorf("broadcasting faction defeat for %s: %w", d.id, err)
		}

		// Conqueror bonus: a small, one-time "you get to remember doing
		// this" cash payoff to whichever real player delivered the
		// final blow - notified directly, not just via the regional
		// broadcast everyone else gets.
		if isRealAttacker {
			if err := e.payConquerorBonus(ctx, tx, d.id, d.name, attackerUserID); err != nil {
				return fmt.Errorf("paying conqueror bonus for defeating %s: %w", d.id, err)
			}
		}
	}
	return nil
}

// mostRecentAttacker looks up the most recent completed raid against
// defenderID to credit as the "final blow" in that faction's epitaph.
// Returns the attacker's display name, whether that attacker is a real
// player (vs. another AI faction - both can finish off a faction, but
// only a real player gets the conqueror bonus below), and the
// attacker's Telegram user_id (0 if not a real player / not found).
// Falls back to ("", false, 0) - which checkForFactionDefeat renders as
// "the wasteland itself" - when there's no raid history at all, e.g. a
// faction that starved out from a weather/tax pass instead of combat.
func (e *Engine) mostRecentAttacker(ctx context.Context, tx *sql.Tx, defenderID string) (name string, isRealPlayer bool, userID int64) {
	var attackerID string
	var isAI bool
	var attackerUserID int64
	err := tx.QueryRowContext(ctx, `
		SELECT a.id, a.name, a.is_ai_faction, a.user_id
		FROM raids r
		JOIN encampments a ON a.id = r.attacker_id
		WHERE r.defender_id = $1 AND r.state = 'completed'
		ORDER BY r.resolve_time DESC LIMIT 1`, defenderID).Scan(&attackerID, &name, &isAI, &attackerUserID)
	if err != nil {
		return "", false, 0
	}
	return name, !isAI, attackerUserID
}

// conquerorBonusDollars is the flat one-time cash payoff a real player
// receives for delivering the finishing blow to an AI faction - scaled
// like Phase 1's relic reward (a genuine, felt windfall) rather than a
// token amount, since finishing off a faction permanently is meant to
// feel like a real, memorable accomplishment.
const conquerorBonusDollars = 5000.0

// payConquerorBonus credits attackerUserID's own encampment with
// conquerorBonusDollars (storage-cap clamped, same as every other
// resource-gain path) and sends them a direct notification distinct
// from the regional "a faction has fallen" broadcast everyone else
// gets.
func (e *Engine) payConquerorBonus(ctx context.Context, tx *sql.Tx, defeatedID, defeatedName string, attackerUserID int64) error {
	var attackerCampID string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", attackerUserID).Scan(&attackerCampID); err != nil {
		// Attacker's encampment no longer exists (account deleted since
		// the raid) - nothing to pay, not an error worth failing the
		// whole tick phase over.
		return nil
	}

	var currentDollars float64
	_ = tx.QueryRowContext(ctx, "SELECT dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", attackerCampID).Scan(&currentDollars)
	storageCap := storagecap.CapFor(ctx, tx, attackerCampID)
	newDollars, _ := storagecap.Clamp(currentDollars, conquerorBonusDollars, storageCap)
	if _, err := tx.ExecContext(ctx, "UPDATE resources SET dollars = $1 WHERE encampment_id = $2", newDollars, attackerCampID); err != nil {
		return err
	}

	msg := fmt.Sprintf("🪦🏆 CONQUEROR: You delivered the finishing blow to %s! +%s awarded for permanently ending their threat.",
		defeatedName, fmt.Sprintf("$%.0f", conquerorBonusDollars))
	return notifications.Queue(ctx, tx, attackerUserID, msg, "general")
}
