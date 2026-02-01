package store

import (
	"context"
	"database/sql"
	"time"
)

type Fact struct {
	ID        int64     `json:"id"`
	UserID    string    `json:"user_id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Category  string    `json:"category"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SetFact creates or updates a fact for a user.
func (db *DB) SetFact(ctx context.Context, userID, key, value, category string) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO facts (user_id, key, value, category, updated_at) 
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		 ON CONFLICT(user_id, key) DO UPDATE SET value=excluded.value, category=excluded.category, updated_at=CURRENT_TIMESTAMP`,
		userID, key, value, category,
	)
	return err
}

// GetFact retrieves a fact by user and key. Returns nil, nil if not found.
func (db *DB) GetFact(ctx context.Context, userID, key string) (*Fact, error) {
	var f Fact
	var cat sql.NullString
	err := db.QueryRowContext(ctx,
		`SELECT id, user_id, key, value, category, created_at, updated_at FROM facts WHERE user_id = ? AND key = ?`,
		userID, key,
	).Scan(&f.ID, &f.UserID, &f.Key, &f.Value, &cat, &f.CreatedAt, &f.UpdatedAt)
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if cat.Valid {
		f.Category = cat.String
	}
	return &f, nil
}

// SearchFacts finds facts for a user where key or value matches the query (LIKE %query%).
func (db *DB) SearchFacts(ctx context.Context, userID, query string) ([]Fact, error) {
	wildcard := "%" + query + "%"
	rows, err := db.QueryContext(ctx,
		`SELECT id, user_id, key, value, category, created_at, updated_at 
		 FROM facts 
		 WHERE user_id = ? AND (key LIKE ? OR value LIKE ?) 
		 ORDER BY updated_at DESC LIMIT 20`,
		userID, wildcard, wildcard,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Fact
	for rows.Next() {
		var f Fact
		var cat sql.NullString
		if err := rows.Scan(&f.ID, &f.UserID, &f.Key, &f.Value, &cat, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, err
		}
		if cat.Valid {
			f.Category = cat.String
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
