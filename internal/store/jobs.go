package store

import (
	"context"
	"database/sql"
	"time"
)

// Job represents a long-running task or "Epic".
type Job struct {
	ID            int64      `json:"id"`
	UserID        string     `json:"user_id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	Status        string     `json:"status"` // "open", "blocked", "closed"
	BlockedReason string     `json:"blocked_reason,omitempty"`
	SnoozedUntil  *time.Time `json:"snoozed_until,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// CreateJob creates a new job.
func (db *DB) CreateJob(ctx context.Context, userID, title, description string) (int64, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO jobs (user_id, title, description, status) VALUES (?, ?, ?, 'open')`,
		userID, title, description,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateJobStatus updates the status and optionally the blocked reason.
func (db *DB) UpdateJobStatus(ctx context.Context, id int64, status, blockedReason string) error {
	_, err := db.ExecContext(ctx,
		`UPDATE jobs SET status = ?, blocked_reason = ?, snoozed_until = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, blockedReason, id,
	)
	return err
}

// SnoozeJob hides a job from the prompt until the specified time.
func (db *DB) SnoozeJob(ctx context.Context, id int64, until time.Time) error {
	_, err := db.ExecContext(ctx,
		`UPDATE jobs SET snoozed_until = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		until, id,
	)
	return err
}

// ListJobs returns jobs filtered by user and status (excludes snoozed jobs).
func (db *DB) ListJobs(ctx context.Context, userID, status string) ([]Job, error) {
	query := `SELECT id, user_id, title, description, status, blocked_reason, snoozed_until, created_at, updated_at 
	          FROM jobs WHERE user_id = ? AND (snoozed_until IS NULL OR snoozed_until <= ?)`
	args := []interface{}{userID, time.Now()}
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

	var jobs []Job
	for rows.Next() {
		var j Job
		var reason sql.NullString
		var snoozed sql.NullTime
		if err := rows.Scan(&j.ID, &j.UserID, &j.Title, &j.Description, &j.Status, &reason, &snoozed, &j.CreatedAt, &j.UpdatedAt); err != nil {
			return nil, err
		}
		if reason.Valid {
			j.BlockedReason = reason.String
		}
		if snoozed.Valid {
			j.SnoozedUntil = &snoozed.Time
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

// GetActiveJob returns the most recent 'open' or 'blocked' job for a user (excludes snoozed).
// This is used to maintain "Epic Context".
func (db *DB) GetActiveJob(ctx context.Context, userID string) (*Job, error) {
	query := `SELECT id, user_id, title, description, status, blocked_reason, snoozed_until, created_at, updated_at FROM jobs 
	          WHERE user_id = ? AND status IN ('open', 'blocked') 
	          AND (snoozed_until IS NULL OR snoozed_until <= ?)
	          ORDER BY updated_at DESC LIMIT 1`
	
	var j Job
	var reason sql.NullString
	var snoozed sql.NullTime
	err := db.QueryRowContext(ctx, query, userID, time.Now()).Scan(&j.ID, &j.UserID, &j.Title, &j.Description, &j.Status, &reason, &snoozed, &j.CreatedAt, &j.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if reason.Valid {
		j.BlockedReason = reason.String
	}
	if snoozed.Valid {
		j.SnoozedUntil = &snoozed.Time
	}
	return &j, nil
}
