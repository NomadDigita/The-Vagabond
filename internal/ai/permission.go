package ai

import (
	"context"
	"database/sql"
)

// PermissionManager decides whether a given (userID, feature) request
// is allowed to run. Two layers of control exist, checked in order:
//
//  1. Global feature flags (ai_feature_flags table) — an admin kill
//     switch for an entire AI subsystem, independent of any one
//     player. Defaults to enabled if no row exists, so shipping a new
//     Feature never requires a migration to turn it on.
//  2. Per-user opt-in/opt-out (ai_permissions table) — lets a player
//     disable AI assistance for themselves (e.g. "I want to fly the
//     Fleet Commander manually"), or lets staff disable AI for one
//     abusive account without flipping the global switch.
//
// The player's own explicit choice is always respected; automation
// beyond "recommend" (e.g. autopilot mode in the Planet Governor) is a
// separate, stricter opt-in enforced by the Phase B handler itself,
// not by this package.
type PermissionManager struct {
	DB *sql.DB
}

func NewPermissionManager(db *sql.DB) *PermissionManager {
	return &PermissionManager{DB: db}
}

// IsAllowed returns whether the request may proceed, and if not, a
// short human-readable reason suitable for showing the player.
func (p *PermissionManager) IsAllowed(ctx context.Context, userID int64, feature Feature) (bool, string, error) {
	var globallyEnabled sql.NullBool
	err := p.DB.QueryRowContext(ctx,
		`SELECT enabled FROM ai_feature_flags WHERE feature = $1`, string(feature),
	).Scan(&globallyEnabled)
	if err != nil && err != sql.ErrNoRows {
		return false, "", err
	}
	if err == nil && globallyEnabled.Valid && !globallyEnabled.Bool {
		return false, "This AI feature is temporarily disabled by the server administrators.", nil
	}

	if userID == 0 {
		// Background/system request — no per-user row applies.
		return true, "", nil
	}

	var userEnabled sql.NullBool
	err = p.DB.QueryRowContext(ctx,
		`SELECT enabled FROM ai_permissions WHERE user_id = $1 AND feature = $2`,
		userID, string(feature),
	).Scan(&userEnabled)
	if err != nil && err != sql.ErrNoRows {
		return false, "", err
	}
	if err == nil && userEnabled.Valid && !userEnabled.Bool {
		return false, "You've disabled this AI feature for yourself. Re-enable it from /ai_settings.", nil
	}

	return true, "", nil
}

// SetUserPreference lets a player (or an admin, on their behalf) opt a
// feature in or out for that specific player.
func (p *PermissionManager) SetUserPreference(ctx context.Context, userID int64, feature Feature, enabled bool) error {
	_, err := p.DB.ExecContext(ctx, `
		INSERT INTO ai_permissions (user_id, feature, enabled, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (user_id, feature) DO UPDATE SET enabled = $3, updated_at = now()`,
		userID, string(feature), enabled)
	return err
}

// SetGlobalFlag is the admin-only global kill switch for a feature.
func (p *PermissionManager) SetGlobalFlag(ctx context.Context, feature Feature, enabled bool) error {
	_, err := p.DB.ExecContext(ctx, `
		INSERT INTO ai_feature_flags (feature, enabled, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (feature) DO UPDATE SET enabled = $2, updated_at = now()`,
		string(feature), enabled)
	return err
}

// GlobalFlags returns the current enabled/disabled state of every
// known feature, defaulting missing rows to enabled=true.
func (p *PermissionManager) GlobalFlags(ctx context.Context) (map[Feature]bool, error) {
	flags := make(map[Feature]bool)
	for _, f := range AllFeatures() {
		flags[f] = true
	}
	rows, err := p.DB.QueryContext(ctx, `SELECT feature, enabled FROM ai_feature_flags`)
	if err != nil {
		return flags, err
	}
	defer rows.Close()
	for rows.Next() {
		var f string
		var enabled bool
		if err := rows.Scan(&f, &enabled); err == nil {
			flags[Feature(f)] = enabled
		}
	}
	return flags, nil
}
