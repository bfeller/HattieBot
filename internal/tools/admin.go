package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hattiebot/hattiebot/internal/store"
)

// ApproveUser approves a pending user or updates their trust level.
func ApproveUser(ctx context.Context, db *store.DB, argsJSON string) (string, error) {
	// 1. Authorization Check
	trustLevel, ok := ctx.Value("user_trust").(string)
	if !ok || trustLevel != "admin" {
		return "", fmt.Errorf("unauthorized: only admins can approve users")
	}

	// 2. Parse Args
	var args struct {
		UserID string `json:"user_id"`
		Level  string `json:"level"` // optional, default "trusted"
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	if args.Level == "" {
		args.Level = "trusted"
	}

	// 3. Validation
	validLevels := map[string]bool{"admin": true, "trusted": true, "guest": true, "restricted": true, "blocked": true}
	if !validLevels[args.Level] {
		return "", fmt.Errorf("invalid level: %s", args.Level)
	}

	// 4. Update
	if err := db.UpdateUserTrust(ctx, args.UserID, args.Level); err != nil {
		return "", err
	}

	return fmt.Sprintf("User %s updated to trust level '%s'", args.UserID, args.Level), nil
}

// BlockUser blocks a user.
func BlockUser(ctx context.Context, db *store.DB, argsJSON string) (string, error) {
	// 1. Authorization Check
	trustLevel, ok := ctx.Value("user_trust").(string)
	if !ok || trustLevel != "admin" {
		return "", fmt.Errorf("unauthorized: only admins can block users")
	}

	// 2. Parse Args
	var args struct {
		UserID string `json:"user_id"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// 3. Update
	if err := db.UpdateUserTrust(ctx, args.UserID, "blocked"); err != nil {
		return "", err
	}

	return fmt.Sprintf("User %s blocked", args.UserID), nil
}

// ListUsers lists users (optionally filtered by trust level).
func ListUsers(ctx context.Context, db *store.DB, argsJSON string) (string, error) {
	// 1. Authorization Check
	trustLevel, ok := ctx.Value("user_trust").(string)
	if !ok || trustLevel != "admin" {
		return "", fmt.Errorf("unauthorized: only admins can list users")
	}

	// 2. Parse Args
	var args struct {
		FilterLevel string `json:"filter_level"`
	}
	json.Unmarshal([]byte(argsJSON), &args) // Ignore error, optional

	// 3. Query (Simple raw query safely)
	// TODO: Add ListUsers to DB methods properly?
	// For now, doing it here with DB access since DB is passed.
	// But DB methods are better. Let's do a raw query for speed or add to store.
	
	query := `SELECT id, name, trust_level, platform, last_seen FROM users`
	var params []interface{}
	if args.FilterLevel != "" {
		query += ` WHERE trust_level = ?`
		params = append(params, args.FilterLevel)
	}
	query += ` ORDER BY last_seen DESC LIMIT 50`

	rows, err := db.QueryContext(ctx, query, params...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var id, name, level, platform string
		var lastSeen interface{}
		if err := rows.Scan(&id, &name, &level, &platform, &lastSeen); err != nil {
			continue
		}
		users = append(users, map[string]interface{}{
			"id":          id,
			"name":        name,
			"trust_level": level,
			"platform":    platform,
			"last_seen":   lastSeen,
		})
	}

	bytes, _ := json.MarshalIndent(users, "", "  ")
	return string(bytes), nil
}
