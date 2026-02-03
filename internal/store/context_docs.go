package store

import (
	"context"
	"database/sql"
	"time"
)

// ContextDoc represents a document that can be loaded into the LLM context.
type ContextDoc struct {
	ID          int64     `json:"id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Description string    `json:"description"`
	IsActive    bool      `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateContextDoc inserts a new context document.
func (db *DB) CreateContextDoc(ctx context.Context, title, content, description string) (int64, error) {
	res, err := db.ExecContext(ctx,
		`INSERT INTO context_documents (title, content, description, is_active) VALUES (?, ?, ?, 0)`,
		title, content, description,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateContextDoc updates an existing context document.
func (db *DB) UpdateContextDoc(ctx context.Context, title, content, description string) error {
	// Build dynamic query based on what's provided? Or just require content?
	// For simplicity, let's assume if content/description is empty, we keep existing?
	// Actually, standard pattern is to update what's passed. But if empty string is valid update?
	// Let's assume the tool layer handles logic of fetching existing if needed.
	// But to be safe, let's just update provided fields.
	
	// However, simple SQL is better. Let's assume the caller provides all fields or we retrieve first.
	// Let's make it simple: Update everything.
	
	_, err := db.ExecContext(ctx,
		`UPDATE context_documents SET content = ?, description = ?, updated_at = CURRENT_TIMESTAMP WHERE title = ?`,
		content, description, title,
	)
	return err
}

// GetContextDoc retrieves a document by title.
func (db *DB) GetContextDoc(ctx context.Context, title string) (*ContextDoc, error) {
	var doc ContextDoc
	var isActive int // SQLite bool is int
	
	err := db.QueryRowContext(ctx,
		`SELECT id, title, content, description, is_active, created_at, updated_at FROM context_documents WHERE title = ?`,
		title,
	).Scan(&doc.ID, &doc.Title, &doc.Content, &doc.Description, &isActive, &doc.CreatedAt, &doc.UpdatedAt)
	doc.IsActive = isActive != 0
	
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &doc, nil
}

// ListContextDocs returns metadata for all documents.
func (db *DB) ListContextDocs(ctx context.Context) ([]ContextDoc, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, title, content, description, is_active, created_at, updated_at FROM context_documents ORDER BY title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []ContextDoc
	for rows.Next() {
		var doc ContextDoc
		if err := rows.Scan(&doc.ID, &doc.Title, &doc.Content, &doc.Description, &doc.IsActive, &doc.CreatedAt, &doc.UpdatedAt); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

// ListActiveContextDocs returns specific active documents.
func (db *DB) ListActiveContextDocs(ctx context.Context) ([]ContextDoc, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, title, content, description, is_active, created_at, updated_at FROM context_documents WHERE is_active = 1 ORDER BY title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []ContextDoc
	for rows.Next() {
		var doc ContextDoc
		if err := rows.Scan(&doc.ID, &doc.Title, &doc.Content, &doc.Description, &doc.IsActive, &doc.CreatedAt, &doc.UpdatedAt); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}
	return docs, rows.Err()
}

// SetContextDocActive toggles the active state.
func (db *DB) SetContextDocActive(ctx context.Context, title string, active bool) error {
	val := 0
	if active {
		val = 1
	}
	_, err := db.ExecContext(ctx,
		`UPDATE context_documents SET is_active = ?, updated_at = CURRENT_TIMESTAMP WHERE title = ?`,
		val, title,
	)
	return err
}

// DeleteContextDoc removes a document.
func (db *DB) DeleteContextDoc(ctx context.Context, title string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM context_documents WHERE title = ?`, title)
	return err
}
