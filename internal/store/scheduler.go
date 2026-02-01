package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type ScheduledPlan struct {
	ID            int64      `json:"id"`
	UserID        string     `json:"user_id"`
	Description   string     `json:"description"`
	ActionType    string     `json:"action_type"`    // "remind", "execute_tool"
	ActionPayload string     `json:"action_payload"` // JSON
	ScheduleType  string     `json:"schedule_type"`  // "once", "daily", "weekly"
	ScheduleValue string     `json:"schedule_value"` // time or datetime
	NextRunAt     *time.Time `json:"next_run_at"`
	LastRunAt     *time.Time `json:"last_run_at"`
	LockedUntil   *time.Time `json:"locked_until"`
	Status        string     `json:"status"` // active, paused, completed
	CreatedAt     time.Time  `json:"created_at"`
}

// CreatePlan creates a new scheduled plan.
func (db *DB) CreatePlan(ctx context.Context, userID, description, actionType, actionPayload, scheduleType, scheduleValue string, nextRunAt time.Time) (int64, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO scheduled_plans (user_id, description, action_type, action_payload, schedule_type, schedule_value, next_run_at, status) 
		 VALUES (?, ?, ?, ?, ?, ?, ?, 'active')`,
		userID, description, actionType, actionPayload, scheduleType, scheduleValue, nextRunAt,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ListPlans returns all plans for a user with optional status filter.
func (db *DB) ListPlans(ctx context.Context, userID, status string) ([]ScheduledPlan, error) {
	query := `SELECT id, user_id, description, action_type, action_payload, schedule_type, schedule_value, next_run_at, last_run_at, status, created_at FROM scheduled_plans WHERE user_id = ?`
	args := []interface{}{userID}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY next_run_at ASC"

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ScheduledPlan
	for rows.Next() {
		var p ScheduledPlan
		var nextRun, lastRun sql.NullTime
		var payload sql.NullString
		if err := rows.Scan(&p.ID, &p.UserID, &p.Description, &p.ActionType, &payload, &p.ScheduleType, &p.ScheduleValue, &nextRun, &lastRun, &p.Status, &p.CreatedAt); err != nil {
			return nil, err
		}
		if nextRun.Valid {
			p.NextRunAt = &nextRun.Time
		}
		if lastRun.Valid {
			p.LastRunAt = &lastRun.Time
		}
		if payload.Valid {
			p.ActionPayload = payload.String
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// GetDuePlans returns plans that should run now or in the past (global, for scheduler).
func (db *DB) GetDuePlans(ctx context.Context) ([]ScheduledPlan, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, user_id, description, action_type, action_payload, schedule_type, schedule_value, next_run_at, last_run_at, status, created_at 
		 FROM scheduled_plans 
		 WHERE status = 'active' AND next_run_at <= ?`,
		time.Now(),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ScheduledPlan
	for rows.Next() {
		var p ScheduledPlan
		var nextRun, lastRun sql.NullTime
		var payload sql.NullString
		if err := rows.Scan(&p.ID, &p.UserID, &p.Description, &p.ActionType, &payload, &p.ScheduleType, &p.ScheduleValue, &nextRun, &lastRun, &p.Status, &p.CreatedAt); err != nil {
			return nil, err
		}
		if nextRun.Valid {
			p.NextRunAt = &nextRun.Time
		}
		if lastRun.Valid {
			p.LastRunAt = &lastRun.Time
		}
		if payload.Valid {
			p.ActionPayload = payload.String
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ClaimDuePlans atomically locks and returns plans that are due (global, for scheduler).
func (db *DB) ClaimDuePlans(ctx context.Context, lockTimeout time.Duration) ([]ScheduledPlan, error) {
	now := time.Now()
	lockUntil := now.Add(lockTimeout)

	// Attempt using UPDATE ... RETURNING (SQLite 3.35+)
	query := `
		UPDATE scheduled_plans 
		SET locked_until = ? 
		WHERE status = 'active' 
		  AND next_run_at <= ? 
		  AND (locked_until IS NULL OR locked_until < ?)
		RETURNING id, user_id, description, action_type, action_payload, schedule_type, schedule_value, next_run_at, last_run_at, locked_until, status, created_at
	`
	
	rows, err := db.QueryContext(ctx, query, lockUntil, now, now)
	if err != nil {
		return nil, fmt.Errorf("claiming plans: %w", err)
	}
	defer rows.Close()

	var out []ScheduledPlan
	for rows.Next() {
		var p ScheduledPlan
		var nextRun, lastRun, lockedUntil sql.NullTime
		var payload sql.NullString
		if err := rows.Scan(&p.ID, &p.UserID, &p.Description, &p.ActionType, &payload, &p.ScheduleType, &p.ScheduleValue, &nextRun, &lastRun, &lockedUntil, &p.Status, &p.CreatedAt); err != nil {
			return nil, err
		}
		if nextRun.Valid { p.NextRunAt = &nextRun.Time }
		if lastRun.Valid { p.LastRunAt = &lastRun.Time }
		if lockedUntil.Valid { p.LockedUntil = &lockedUntil.Time }
		if payload.Valid { p.ActionPayload = payload.String }
		out = append(out, p)
	}
	return out, rows.Err()
}

// MarkPlanRun updates last_run_at and calculates next_run_at for recurring plans.
func (db *DB) MarkPlanRun(ctx context.Context, id int64, scheduleType string) error {
	now := time.Now()
	var nextRun *time.Time

	switch scheduleType {
	case "once":
		// One-time plans are marked complete
		_, err := db.ExecContext(ctx,
			`UPDATE scheduled_plans SET last_run_at = ?, status = 'completed' WHERE id = ?`,
			now, id,
		)
		return err
	case "daily":
		next := now.Add(24 * time.Hour)
		nextRun = &next
	case "weekly":
		next := now.Add(7 * 24 * time.Hour)
		nextRun = &next
	case "hourly":
		next := now.Add(1 * time.Hour)
		nextRun = &next
	}

	_, err := db.ExecContext(ctx,
		`UPDATE scheduled_plans SET last_run_at = ?, next_run_at = ?, locked_until = NULL WHERE id = ?`,
		now, nextRun, id,
	)
	return err
}

// UpdatePlanStatus changes the status of a plan.
func (db *DB) UpdatePlanStatus(ctx context.Context, id int64, status string) error {
	_, err := db.ExecContext(ctx, `UPDATE scheduled_plans SET status = ? WHERE id = ?`, status, id)
	return err
}

// DeletePlan removes a plan.
func (db *DB) DeletePlan(ctx context.Context, id int64) error {
	_, err := db.ExecContext(ctx, `DELETE FROM scheduled_plans WHERE id = ?`, id)
	return err
}
