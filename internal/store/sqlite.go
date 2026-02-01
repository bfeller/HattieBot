package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB wraps *sql.DB for HattieBot storage. Schema is owned by the app; no agent SQL.
type DB struct {
	*sql.DB
}

// Open opens the SQLite database at path and applies the schema. Creates file if missing.
// When embedding is enabled (e.g. via config), load sqlite-vec extension and create vec0
// virtual table for message or tool-doc embeddings; the agent can then use RAG for context.
func Open(ctx context.Context, path string) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	// TODO: if config has embedding_model set, load sqlite-vec and create vec table
	if _, err := db.ExecContext(ctx, schema); err != nil {
		db.Close()
		return nil, err
	}

	// Schema Migration: Ensure locked_until exists for scheduled_plans
	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('scheduled_plans') WHERE name='locked_until'").Scan(&count); err == nil && count == 0 {
		if _, err := db.ExecContext(ctx, "ALTER TABLE scheduled_plans ADD COLUMN locked_until DATETIME"); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrating schema (scheduled_plans.locked_until): %w", err)
		}
	}

	// Gap 3 Migrations: Strict Schema (No defaults, assumes empty tables if NOT NULL required)

	// 1. users table: handled by schema exec (CREATE IF NOT EXISTS)

	// 2. messages: sender_id
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('messages') WHERE name='sender_id'").Scan(&count); err == nil && count == 0 {
		// Strict migration: fails if table has rows
		if _, err := db.ExecContext(ctx, "ALTER TABLE messages ADD COLUMN sender_id TEXT NOT NULL"); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrating schema (messages.sender_id): %w (table must be empty or column allows null)", err)
		}
	}

	// 3. messages: channel
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('messages') WHERE name='channel'").Scan(&count); err == nil && count == 0 {
		if _, err := db.ExecContext(ctx, "ALTER TABLE messages ADD COLUMN channel TEXT NOT NULL"); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrating schema (messages.channel): %w", err)
		}
	}

	// 4. facts: user_id
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('facts') WHERE name='user_id'").Scan(&count); err == nil && count == 0 {
		// facts UNIQUE constraint issue: existing is UNIQUE(key). New schema wants UNIQUE(user_id, key).
		// SQLite ALTER TABLE cannot drop constraints. We must recreate if we want to enforce new constraint.
		// For now, adding column is enough to support code. constraint remains UNIQUE(key) for old table.
		// If we really want to fix constraint, we need recreation.
		// Given strict "greenfield", we could try to Rename-Recreate if table is empty.
		// Simplified: Just add column.
		if _, err := db.ExecContext(ctx, "ALTER TABLE facts ADD COLUMN user_id TEXT NOT NULL"); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrating schema (facts.user_id): %w", err)
		}
	}

	// 5. messages: thread_id
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('messages') WHERE name='thread_id'").Scan(&count); err == nil && count == 0 {
		if _, err := db.ExecContext(ctx, "ALTER TABLE messages ADD COLUMN thread_id TEXT NOT NULL DEFAULT ''"); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrating schema (messages.thread_id): %w", err)
		}
	}

	// 6. users: trust_level
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('users') WHERE name='trust_level'").Scan(&count); err == nil && count == 0 {
		if _, err := db.ExecContext(ctx, "ALTER TABLE users ADD COLUMN trust_level TEXT DEFAULT 'restricted'"); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrating schema (users.trust_level): %w", err)
		}
	}

	// 7. users: metadata
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('users') WHERE name='metadata'").Scan(&count); err == nil && count == 0 {
		if _, err := db.ExecContext(ctx, "ALTER TABLE users ADD COLUMN metadata TEXT"); err != nil {
			db.Close()
			return nil, fmt.Errorf("migrating schema (users.metadata): %w", err)
		}
	}

	// tools_registry: tool health (status, last_success, failure_count, last_error)
	for _, col := range []struct{ name, def string }{
		{"status", "TEXT DEFAULT 'active'"},
		{"last_success", "DATETIME"},
		{"failure_count", "INTEGER DEFAULT 0"},
		{"last_error", "TEXT"},
	} {
		if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('tools_registry') WHERE name=?", col.name).Scan(&count); err == nil && count == 0 {
			if _, err := db.ExecContext(ctx, "ALTER TABLE tools_registry ADD COLUMN "+col.name+" "+col.def); err != nil {
				db.Close()
				return nil, fmt.Errorf("migrating schema (tools_registry.%s): %w", col.name, err)
			}
		}
	}

	return &DB{db}, nil
}

// Close closes the database.
func (db *DB) Close() error {
	return db.DB.Close()
}
