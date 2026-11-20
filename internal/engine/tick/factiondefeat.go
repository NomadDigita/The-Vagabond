package tick

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"

	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
	"github.com/NomadDigita/The-Vagabond/internal/game/progression"
	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
)

const factionDefeatResourceThreshold = economicCollapseWarningThreshold

var epitaphTemplates = []string{
	"Here fell %s, broken at last by %s. Its banners are dust; its ruins remain.",
	"%s made its final stand here, and lost - to %s. The wasteland reclaims what it built.",
	"No garrison remains to defend %s. %s saw to that. Only silence answers now.",
	"The fires of %s have gone cold, extinguished by %s. A monument to nothing survives.",
	"%s is no more. %s delivered the final blow. Scavengers already pick at the bones.",
}

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

		if isRealAttacker {
			if err := e.payConquerorBonus(ctx, tx, d.id, d.name, attackerUserID); err != nil {
				return fmt.Errorf("paying conqueror bonus for defeating %s: %w", d.id, err)
			}
			if err := progression.RecordEvent(ctx, tx, attackerUserID, progression.TriggerFactionDefeated, 1); err != nil {
				return fmt.Errorf("recording faction-defeated progress for %d: %w", attackerUserID, err)
			}
		}
	}
	return nil
}

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

const conquerorBonusDollars = 5000.0

func (e *Engine) payConquerorBonus(ctx context.Context, tx *sql.Tx, defeatedID, defeatedName string, attackerUserID int64) error {
	var attackerCampID string
	if err := tx.QueryRowContext(ctx, "SELECT id FROM encampments WHERE user_id = $1", attackerUserID).Scan(&attackerCampID); err != nil {
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
