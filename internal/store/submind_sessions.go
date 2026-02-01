package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/hattiebot/hattiebot/internal/core"
)

// SubmindSession represents a persisted sub-mind run (active or completed).
type SubmindSession struct {
	ID           int64     `json:"id"`
	UserID       string    `json:"user_id"`
	Mode         string    `json:"mode"`
	Task         string    `json:"task"`
	Status       string    `json:"status"` // running, completed, failed, suspended
	MessagesJSON string    `json:"-"`      // stored in DB; use Messages() for parsed slice
	Turns        int       `json:"turns"`
	ResultOutput string    `json:"result_output,omitempty"`
	ResultError  string    `json:"result_error,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Messages returns the session messages parsed from JSON. Returns nil on parse error.
func (s *SubmindSession) Messages() []core.Message {
	if s.MessagesJSON == "" {
		return nil
	}
	var out []core.Message
	if err := json.Unmarshal([]byte(s.MessagesJSON), &out); err != nil {
		return nil
	}
	return out
}

// CreateSubmindSession inserts a new session with status "running", initial messages [system, user], turns 0.
func (db *DB) CreateSubmindSession(ctx context.Context, userID, mode, task, systemPrompt string) (int64, error) {
	initial := []core.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: task},
	}
	raw, err := json.Marshal(initial)
	if err != nil {
		return 0, err
	}
	res, err := db.ExecContext(ctx,
		`INSERT INTO submind_sessions (user_id, mode, task, status, messages, turns) VALUES (?, ?, ?, 'running', ?, 0)`,
		userID, mode, task, string(raw),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// GetSubmindSession returns the session by id and userID. Returns error if not found or wrong user.
func (db *DB) GetSubmindSession(ctx context.Context, id int64, userID string) (*SubmindSession, error) {
	var s SubmindSession
	var resultOut, resultErr sql.NullString
	err := db.QueryRowContext(ctx,
		`SELECT id, user_id, mode, task, status, messages, turns, result_output, result_error, created_at, updated_at
		 FROM submind_sessions WHERE id = ? AND user_id = ?`,
		id, userID,
	).Scan(&s.ID, &s.UserID, &s.Mode, &s.Task, &s.Status, &s.MessagesJSON, &s.Turns, &resultOut, &resultErr, &s.CreatedAt, &s.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	if resultOut.Valid {
		s.ResultOutput = resultOut.String
	}
	if resultErr.Valid {
		s.ResultError = resultErr.String
	}
	return &s, nil
}

// UpdateSubmindSession updates messages, turns, status, and optionally result_output/result_error.
func (db *DB) UpdateSubmindSession(ctx context.Context, id int64, messages []core.Message, turns int, status, resultOutput, resultError string) error {
	raw, err := json.Marshal(messages)
	if err != nil {
		return err
	}
	if status == "" {
		status = "running"
	}
	_, err = db.ExecContext(ctx,
		`UPDATE submind_sessions SET messages = ?, turns = ?, status = ?, result_output = ?, result_error = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		string(raw), turns, status, resultOutput, resultError, id,
	)
	return err
}

// ListSubmindSessions returns sessions for the user, optionally filtered by status ("" = all).
func (db *DB) ListSubmindSessions(ctx context.Context, userID, status string) ([]SubmindSession, error) {
	query := `SELECT id, user_id, mode, task, status, turns, result_output, result_error, created_at, updated_at
	          FROM submind_sessions WHERE user_id = ?`
	args := []interface{}{userID}
	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY updated_at DESC`

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []SubmindSession
	for rows.Next() {
		var s SubmindSession
		var resultOut, resultErr sql.NullString
		if err := rows.Scan(&s.ID, &s.UserID, &s.Mode, &s.Task, &s.Status, &s.Turns, &resultOut, &resultErr, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		if resultOut.Valid {
			s.ResultOutput = resultOut.String
		}
		if resultErr.Valid {
			s.ResultError = resultErr.String
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
