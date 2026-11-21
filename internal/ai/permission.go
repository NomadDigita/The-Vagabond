package ai

import (
	"context"
	"database/sql"
)

type PermissionManager struct {
	DB *sql.DB
}

func NewPermissionManager(db *sql.DB) *PermissionManager {
	return &PermissionManager{DB: db}
}

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

func (p *PermissionManager) SetUserPreference(ctx context.Context, userID int64, feature Feature, enabled bool) error {
	_, err := p.DB.ExecContext(ctx, `
		INSERT INTO ai_permissions (user_id, feature, enabled, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (user_id, feature) DO UPDATE SET enabled = $3, updated_at = now()`,
		userID, string(feature), enabled)
	return err
}

func (p *PermissionManager) SetGlobalFlag(ctx context.Context, feature Feature, enabled bool) error {
	_, err := p.DB.ExecContext(ctx, `
		INSERT INTO ai_feature_flags (feature, enabled, updated_at)
		VALUES ($1, $2, now())
		ON CONFLICT (feature) DO UPDATE SET enabled = $2, updated_at = now()`,
		string(feature), enabled)
	return err
}

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
