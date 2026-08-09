package tick

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/NomadDigita/The-Vagabond/internal/engine/notifications"
	"github.com/NomadDigita/The-Vagabond/internal/game/storagecap"
)

// RARE_WORLD_FEATURES_PLAN.md Phase 5: Constellation Alliances.
//
// The handler side (formation, accept/reject) lives in
// internal/bot/handlers/constellation.go. This file is the tick.Engine
// side: the joint dividend payout and automatic expiry for active
// Constellations.

// constellationDividendDollars is the flat "Alignment Dividend" every
// member Clan's encampments each receive once per day while their
// Constellation is active - deliberately a flat administrative bonus
// rather than a percentage-of-resource-generation change, so this
// phase never has to touch the resource-tick formulas Phase 2's Bloom
// bonus already extended (see the plan doc's section 5.4 - keeps the
// two features' code paths fully independent).
const constellationDividendDollars = 500.0

// constellationDividend is the "constellation_dividend" tick phase.
// For every active Constellation whose last_dividend_at is more than
// 24 hours ago (or never set), every member Clan's encampments each
// receive constellationDividendDollars, storage-cap clamped like every
// other resource-gain path, with a direct notification per recipient.
// Tracked via last_dividend_at rather than assuming the tick cadence
// lines up with a calendar day - see plan doc 5.4.
func (e *Engine) constellationDividend(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, name, banner_emoji FROM constellations
		WHERE status = 'active'
		  AND (last_dividend_at IS NULL OR last_dividend_at <= CURRENT_TIMESTAMP - INTERVAL '24 hours')`)
	if err != nil {
		return fmt.Errorf("querying constellations due a dividend: %w", err)
	}
	type due struct{ id, name, bannerEmoji string }
	var dueList []due
	for rows.Next() {
		var d due
		if scanErr := rows.Scan(&d.id, &d.name, &d.bannerEmoji); scanErr == nil {
			dueList = append(dueList, d)
		}
	}
	rows.Close()

	for _, d := range dueList {
		if err := e.payConstellationDividend(ctx, tx, d.id, d.name, d.bannerEmoji); err != nil {
			return fmt.Errorf("paying dividend for constellation %s: %w", d.id, err)
		}
		if _, err := tx.ExecContext(ctx, "UPDATE constellations SET last_dividend_at = CURRENT_TIMESTAMP WHERE id = $1", d.id); err != nil {
			return fmt.Errorf("updating last_dividend_at for constellation %s: %w", d.id, err)
		}
	}
	return nil
}

// payConstellationDividend credits constellationDividendDollars to
// every member Clan's encampments and notifies each recipient
// directly.
func (e *Engine) payConstellationDividend(ctx context.Context, tx *sql.Tx, constellationID, name, bannerEmoji string) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT enc.id, enc.user_id
		FROM constellation_members cm
		JOIN clans cl ON cl.id = cm.clan_id
		JOIN user_clans uc ON uc.clan_id = cl.id
		JOIN encampments enc ON enc.user_id = uc.user_id
		WHERE cm.constellation_id = $1 AND cm.status = 'accepted'`, constellationID)
	if err != nil {
		return fmt.Errorf("finding constellation member encampments: %w", err)
	}
	type recipient struct {
		campID string
		userID int64
	}
	var recipients []recipient
	for rows.Next() {
		var r recipient
		if scanErr := rows.Scan(&r.campID, &r.userID); scanErr == nil {
			recipients = append(recipients, r)
		}
	}
	rows.Close()

	for _, r := range recipients {
		var currentDollars float64
		_ = tx.QueryRowContext(ctx, "SELECT dollars FROM resources WHERE encampment_id = $1 FOR UPDATE", r.campID).Scan(&currentDollars)
		storageCap := storagecap.CapFor(ctx, tx, r.campID)
		newDollars, _ := storagecap.Clamp(currentDollars, constellationDividendDollars, storageCap)
		if _, err := tx.ExecContext(ctx, "UPDATE resources SET dollars = $1 WHERE encampment_id = $2", newDollars, r.campID); err != nil {
			return err
		}
		msg := fmt.Sprintf("%s ALIGNMENT DIVIDEND: The %s Constellation has paid your outpost +%s in joint prosperity.",
			bannerEmoji, name, fmt.Sprintf("$%.0f", constellationDividendDollars))
		if err := notifications.Queue(ctx, tx, r.userID, msg, "general"); err != nil {
			return err
		}
	}
	return nil
}

// constellationExpire is the "constellation_expire" tick phase -
// active Constellations past their expires_at become 'dissolved' with
// a "the Constellation has ended" broadcast to every member Clan,
// matching every other expiry phase in this feature set (Phase
// 1.4/2/4.5's precedent).
func (e *Engine) constellationExpire(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, name, banner_emoji FROM constellations
		WHERE status = 'active' AND expires_at <= CURRENT_TIMESTAMP`)
	if err != nil {
		return fmt.Errorf("querying expired constellations: %w", err)
	}
	type expired struct{ id, name, bannerEmoji string }
	var toExpire []expired
	for rows.Next() {
		var x expired
		if scanErr := rows.Scan(&x.id, &x.name, &x.bannerEmoji); scanErr == nil {
			toExpire = append(toExpire, x)
		}
	}
	rows.Close()

	for _, x := range toExpire {
		if _, err := tx.ExecContext(ctx, "UPDATE constellations SET status = 'dissolved' WHERE id = $1", x.id); err != nil {
			return fmt.Errorf("dissolving constellation %s: %w", x.id, err)
		}

		memberRows, err := tx.QueryContext(ctx, `
			SELECT uc.user_id
			FROM constellation_members cm
			JOIN user_clans uc ON uc.clan_id = cm.clan_id
			WHERE cm.constellation_id = $1 AND cm.status = 'accepted'`, x.id)
		if err != nil {
			return fmt.Errorf("finding constellation members to notify of expiry: %w", err)
		}
		var userIDs []int64
		for memberRows.Next() {
			var uid int64
			if scanErr := memberRows.Scan(&uid); scanErr == nil {
				userIDs = append(userIDs, uid)
			}
		}
		memberRows.Close()

		msg := fmt.Sprintf("%s The %s Constellation has ended - its duration ran out. The Alignment Dividend has stopped, but nothing else about your Clan changes.", x.bannerEmoji, x.name)
		for _, uid := range userIDs {
			if err := notifications.Queue(ctx, tx, uid, msg, "general"); err != nil {
				return fmt.Errorf("notifying constellation member %d of expiry: %w", uid, err)
			}
		}
	}
	return nil
}
