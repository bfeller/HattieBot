package store

import (
	"context"
	"encoding/json"
	"database/sql"
)

// SelfModification represents a self-modification log entry.
type SelfModification struct {
	ID          int64    `json:"id"`
	CreatedAt   string   `json:"created_at"`
	FilePaths   []string `json:"file_paths"`
	ChangeType  string   `json:"change_type"`
	Description string   `json:"description"`
	Context     string   `json:"context,omitempty"`
}

// InsertSelfModification records a self-modification entry.
func (db *DB) InsertSelfModification(ctx context.Context, filePaths []string, changeType, description, context string) error {
	pathsJSON, err := json.Marshal(filePaths)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx,
		"INSERT INTO self_modifications (file_paths, change_type, description, context) VALUES (?, ?, ?, ?)",
		string(pathsJSON), changeType, description, context,
	)
	return err
}

// ListSelfModifications returns recent entries, newest first. limit 0 means default 20.
func (db *DB) ListSelfModifications(ctx context.Context, limit int) ([]SelfModification, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := db.QueryContext(ctx,
		"SELECT id, created_at, file_paths, change_type, description, context FROM self_modifications ORDER BY created_at DESC LIMIT ?",
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SelfModification
	for rows.Next() {
		var sm SelfModification
		var pathsJSON string
		var ctxVal sql.NullString
		if err := rows.Scan(&sm.ID, &sm.CreatedAt, &pathsJSON, &sm.ChangeType, &sm.Description, &ctxVal); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(pathsJSON), &sm.FilePaths); err != nil {
			sm.FilePaths = []string{pathsJSON}
		}
		if ctxVal.Valid {
			sm.Context = ctxVal.String
		}
		result = append(result, sm)
	}
	return result, rows.Err()
}
