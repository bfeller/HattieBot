package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// TrustedIdentity represents a verified external identity (email, phone, etc.).
type TrustedIdentity struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`  // e.g. "email", "phone", "api_key"
	Value     string    `json:"value"` // e.g. "bob@example.com"
	Notes     string    `json:"notes"`
	AddedAt   time.Time `json:"added_at"`
}

// AddTrustedIdentity adds a new identity to the trust circle.
func (db *DB) AddTrustedIdentity(ctx context.Context, idType, value, notes string) error {
	idType = strings.ToLower(strings.TrimSpace(idType))
	value = strings.TrimSpace(value)
	if idType == "" || value == "" {
		return fmt.Errorf("type and value are required")
	}

	query := `INSERT INTO trusted_identities (type, value, notes) VALUES (?, ?, ?)`
	_, err := db.ExecContext(ctx, query, idType, value, notes)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return fmt.Errorf("identity already trusted")
		}
		return err
	}
	return nil
}

// RemoveTrustedIdentity removes an identity from the trust circle.
func (db *DB) RemoveTrustedIdentity(ctx context.Context, idType, value string) error {
	query := `DELETE FROM trusted_identities WHERE type = ? AND value = ?`
	res, err := db.ExecContext(ctx, query, idType, value)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("identity not found")
	}
	return nil
}

// CheckTrustedIdentity returns true if the identity is trusted.
func (db *DB) CheckTrustedIdentity(ctx context.Context, idType, value string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM trusted_identities WHERE type = ? AND value = ?`
	err := db.QueryRowContext(ctx, query, idType, value).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ListTrustedIdentities returns all trusted identities, optionally filtered by type.
func (db *DB) ListTrustedIdentities(ctx context.Context, filterType string) ([]TrustedIdentity, error) {
	var query string
	var args []interface{}
	
	if filterType != "" {
		query = `SELECT id, type, value, notes, added_at FROM trusted_identities WHERE type = ? ORDER BY type, value`
		args = []interface{}{filterType}
	} else {
		query = `SELECT id, type, value, notes, added_at FROM trusted_identities ORDER BY type, value`
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var identities []TrustedIdentity
	for rows.Next() {
		var i TrustedIdentity
		var addedAt sql.NullTime
		if err := rows.Scan(&i.ID, &i.Type, &i.Value, &i.Notes, &addedAt); err != nil {
			return nil, err
		}
		if addedAt.Valid {
			i.AddedAt = addedAt.Time
		}
		identities = append(identities, i)
	}
	return identities, nil
}
