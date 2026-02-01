package store

import (
	"context"
	"database/sql"
	"time"
)

// User represents a user interaction identity.
type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	Platform   string    `json:"platform"`
	TrustLevel string    `json:"trust_level"`
	Metadata   string    `json:"metadata"` // JSON
	FirstSeen  time.Time `json:"first_seen"`
	LastSeen   time.Time `json:"last_seen"`
}

// GetOrCreateUser retrieves a user by ID, or creates one if not exists.
func (db *DB) GetOrCreateUser(ctx context.Context, id, name, platform string) (*User, error) {
	// Try to get
	u, err := db.GetUser(ctx, id)
	if err == nil {
		// Update last_seen
		db.ExecContext(ctx, "UPDATE users SET last_seen=CURRENT_TIMESTAMP WHERE id=?", id)
		return u, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Create
	if name == "" {
		name = "User " + id // Fallback name
	}
	role := "user" // Default role
	
	_, err = db.ExecContext(ctx,
		`INSERT INTO users (id, name, role, platform) VALUES (?, ?, ?, ?)`,
		id, name, role, platform,
	)
	if err != nil {
		return nil, err
	}

	return db.GetUser(ctx, id)
}

// GetUser retrieves a user by ID.
func (db *DB) GetUser(ctx context.Context, id string) (*User, error) {
	var u User
	err := db.QueryRowContext(ctx,
		`SELECT id, name, role, platform, trust_level, COALESCE(metadata, ''), first_seen, last_seen FROM users WHERE id = ?`,
		id,
	).Scan(&u.ID, &u.Name, &u.Role, &u.Platform, &u.TrustLevel, &u.Metadata, &u.FirstSeen, &u.LastSeen)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateUserTrust updates a user's trust level.
func (db *DB) UpdateUserTrust(ctx context.Context, id, level string) error {
	_, err := db.ExecContext(ctx, "UPDATE users SET trust_level = ? WHERE id = ?", level, id)
	return err
}

// UpdateUserMetadata updates the metadata JSON for a user.
func (db *DB) UpdateUserMetadata(ctx context.Context, id, metadata string) error {
	_, err := db.ExecContext(ctx, "UPDATE users SET metadata = ? WHERE id = ?", metadata, id)
	return err
}
