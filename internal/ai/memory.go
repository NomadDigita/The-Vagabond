package ai

import (
	"context"
	"database/sql"
)

type MemoryStore interface {
	Append(ctx context.Context, userID int64, scope string, msg Message) error
	Recent(ctx context.Context, userID int64, scope string, limit int) ([]Message, error)
	Clear(ctx context.Context, userID int64, scope string) error
}

type PostgresMemoryStore struct {
	DB *sql.DB
}

func NewPostgresMemoryStore(db *sql.DB) *PostgresMemoryStore {
	return &PostgresMemoryStore{DB: db}
}

func (m *PostgresMemoryStore) Append(ctx context.Context, userID int64, scope string, msg Message) error {
	_, err := m.DB.ExecContext(ctx, `
		INSERT INTO ai_memory (user_id, scope, role, content, tool_call_id, created_at)
		VALUES ($1, $2, $3, $4, $5, now())`,
		userID, scope, string(msg.Role), msg.Content, nullableString(msg.ToolCallID))
	return err
}

func (m *PostgresMemoryStore) Recent(ctx context.Context, userID int64, scope string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := m.DB.QueryContext(ctx, `
		SELECT role, content, COALESCE(tool_call_id, '')
		FROM (
			SELECT role, content, tool_call_id, created_at
			FROM ai_memory
			WHERE user_id = $1 AND scope = $2
			ORDER BY created_at DESC
			LIMIT $3
		) recent
		ORDER BY created_at ASC`,
		userID, scope, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var role, content, toolCallID string
		if err := rows.Scan(&role, &content, &toolCallID); err != nil {
			return nil, err
		}
		out = append(out, Message{Role: Role(role), Content: content, ToolCallID: toolCallID})
	}
	return out, rows.Err()
}

func (m *PostgresMemoryStore) Clear(ctx context.Context, userID int64, scope string) error {
	_, err := m.DB.ExecContext(ctx, `DELETE FROM ai_memory WHERE user_id = $1 AND scope = $2`, userID, scope)
	return err
}

func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
