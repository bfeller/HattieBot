package store

import (
	"context"
	"database/sql"
	"time"
)

// RegisteredTool is a row in tools_registry.
type RegisteredTool struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	BinaryPath   string     `json:"binary_path"`
	Description  string     `json:"description"`
	InputSchema  string     `json:"input_schema"` // JSON Schema text
	CreatedAt    time.Time  `json:"created_at"`
	Status       string     `json:"status"`        // active, broken, pending_repair, deprecated
	LastSuccess  *time.Time `json:"last_success,omitempty"`
	FailureCount int       `json:"failure_count"`
	LastError    string     `json:"last_error,omitempty"`
}

// InsertTool inserts a tool and returns its id. New tools get status 'active' and failure_count 0.
func (db *DB) InsertTool(ctx context.Context, name, binaryPath, description, inputSchema string) (int64, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO tools_registry (name, binary_path, description, input_schema, status, failure_count) VALUES (?, ?, ?, ?, 'active', 0)`,
		name, binaryPath, description, inputSchema,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ToolByName returns the tool with the given name, or nil if not found.
func (db *DB) ToolByName(ctx context.Context, name string) (*RegisteredTool, error) {
	var t RegisteredTool
	var inputSchema sql.NullString
	var lastSuccess sql.NullTime
	var status sql.NullString
	var failureCount sql.NullInt64
	var lastError sql.NullString
	err := db.QueryRowContext(ctx,
		`SELECT id, name, binary_path, description, input_schema, created_at, status, last_success, failure_count, last_error FROM tools_registry WHERE name = ?`,
		name,
	).Scan(&t.ID, &t.Name, &t.BinaryPath, &t.Description, &inputSchema, &t.CreatedAt, &status, &lastSuccess, &failureCount, &lastError)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if inputSchema.Valid {
		t.InputSchema = inputSchema.String
	}
	if status.Valid {
		t.Status = status.String
	} else {
		t.Status = "active"
	}
	if lastSuccess.Valid {
		t.LastSuccess = &lastSuccess.Time
	}
	if failureCount.Valid {
		t.FailureCount = int(failureCount.Int64)
	}
	if lastError.Valid {
		t.LastError = lastError.String
	}
	return &t, nil
}

// AllTools returns all registered tools.
func (db *DB) AllTools(ctx context.Context) ([]RegisteredTool, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, name, binary_path, description, input_schema, created_at, status, last_success, failure_count, last_error FROM tools_registry ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RegisteredTool
	for rows.Next() {
		var t RegisteredTool
		var inputSchema sql.NullString
		var lastSuccess sql.NullTime
		var status sql.NullString
		var failureCount sql.NullInt64
		var lastError sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.BinaryPath, &t.Description, &inputSchema, &t.CreatedAt, &status, &lastSuccess, &failureCount, &lastError); err != nil {
			return nil, err
		}
		if inputSchema.Valid {
			t.InputSchema = inputSchema.String
		}
		if status.Valid {
			t.Status = status.String
		} else {
			t.Status = "active"
		}
		if lastSuccess.Valid {
			t.LastSuccess = &lastSuccess.Time
		}
		if failureCount.Valid {
			t.FailureCount = int(failureCount.Int64)
		}
		if lastError.Valid {
			t.LastError = lastError.String
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteTool removes a tool by name.
func (db *DB) DeleteTool(ctx context.Context, name string) error {
	_, err := db.ExecContext(ctx, "DELETE FROM tools_registry WHERE name = ?", name)
	return err
}

// RecordToolSuccess updates last_success and resets failure_count to 0; sets status to active if it was pending_repair.
func (db *DB) RecordToolSuccess(ctx context.Context, name string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE tools_registry SET last_success = ?, failure_count = 0, status = 'active' WHERE name = ?`,
		time.Now().UTC(), name,
	)
	return err
}

// RecordToolFailure increments failure_count, sets last_error. If failure_count >= 3, sets status to 'broken'.
func (db *DB) RecordToolFailure(ctx context.Context, name, errMsg string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE tools_registry SET failure_count = failure_count + 1, last_error = ? WHERE name = ?`,
		errMsg, name,
	)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx,
		`UPDATE tools_registry SET status = 'broken' WHERE name = ? AND failure_count >= 3`,
		name,
	)
	return err
}

// ListBrokenTools returns tools with status = 'broken' for the repair queue.
func (db *DB) ListBrokenTools(ctx context.Context) ([]RegisteredTool, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, name, binary_path, description, input_schema, created_at, status, last_success, failure_count, last_error FROM tools_registry WHERE status = 'broken' ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RegisteredTool
	for rows.Next() {
		var t RegisteredTool
		var inputSchema sql.NullString
		var lastSuccess sql.NullTime
		var status sql.NullString
		var failureCount sql.NullInt64
		var lastError sql.NullString
		if err := rows.Scan(&t.ID, &t.Name, &t.BinaryPath, &t.Description, &inputSchema, &t.CreatedAt, &status, &lastSuccess, &failureCount, &lastError); err != nil {
			return nil, err
		}
		if inputSchema.Valid {
			t.InputSchema = inputSchema.String
		}
		if status.Valid {
			t.Status = status.String
		}
		if lastSuccess.Valid {
			t.LastSuccess = &lastSuccess.Time
		}
		if failureCount.Valid {
			t.FailureCount = int(failureCount.Int64)
		}
		if lastError.Valid {
			t.LastError = lastError.String
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ToolRegistry interface for dependency injection.
type ToolRegistry interface {
	InsertTool(ctx context.Context, name, binaryPath, description, inputSchema string) (int64, error)
	ToolByName(ctx context.Context, name string) (*RegisteredTool, error)
	AllTools(ctx context.Context) ([]RegisteredTool, error)
	DeleteTool(ctx context.Context, name string) error
}

// Ensure *DB implements ToolRegistry.
var _ ToolRegistry = (*DB)(nil)
