package store

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/hattiebot/hattiebot/internal/health"
)

// LogStore handles structured logging with automatic cleanup.
type LogStore struct {
	db           *sql.DB
	mu           sync.Mutex
	maxEntries   int   // Max log entries to keep
	maxAgeDays   int   // Max age of logs in days
	maxSizeBytes int64 // Max total size (rough estimate)
}

// NewLogStore creates a new log store with default limits.
func NewLogStore(db *sql.DB) *LogStore {
	return &LogStore{
		db:           db,
		maxEntries:   10000,
		maxAgeDays:   7,
		maxSizeBytes: 10 * 1024 * 1024, // 10MB
	}
}

// CreateTable creates the system_logs table if it doesn't exist.
func (s *LogStore) CreateTable() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS system_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			level TEXT NOT NULL,
			component TEXT NOT NULL,
			message TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_logs_timestamp ON system_logs(timestamp);
		CREATE INDEX IF NOT EXISTS idx_logs_level ON system_logs(level);
		CREATE INDEX IF NOT EXISTS idx_logs_component ON system_logs(component);
	`)
	return err
}

// Log writes a log entry.
func (s *LogStore) Log(level, component, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(
		"INSERT INTO system_logs (level, component, message) VALUES (?, ?, ?)",
		level, component, message,
	)
	return err
}

// LogError is a convenience method for error-level logs.
func (s *LogStore) LogError(component, message string) error {
	return s.Log("error", component, message)
}

// LogWarn is a convenience method for warn-level logs.
func (s *LogStore) LogWarn(component, message string) error {
	return s.Log("warn", component, message)
}

// LogInfo is a convenience method for info-level logs.
func (s *LogStore) LogInfo(component, message string) error {
	return s.Log("info", component, message)
}

// GetLogs retrieves recent logs with optional filters.
func (s *LogStore) GetLogs(level, component string, limit int) ([]health.LogEntry, error) {
	query := "SELECT id, timestamp, level, component, message FROM system_logs WHERE 1=1"
	args := []interface{}{}

	if level != "" {
		query += " AND level = ?"
		args = append(args, level)
	}
	if component != "" {
		query += " AND component = ?"
		args = append(args, component)
	}

	query += " ORDER BY timestamp DESC LIMIT ?"
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []health.LogEntry
	for rows.Next() {
		var entry health.LogEntry
		var ts string
		if err := rows.Scan(&entry.ID, &ts, &entry.Level, &entry.Component, &entry.Message); err != nil {
			return nil, err
		}
		entry.Timestamp, _ = time.Parse("2006-01-02 15:04:05", ts)
		logs = append(logs, entry)
	}
	return logs, nil
}

// GetErrors retrieves recent error logs.
func (s *LogStore) GetErrors(limit int) ([]health.LogEntry, error) {
	return s.GetLogs("error", "", limit)
}

// Cleanup removes old logs based on configured limits.
func (s *LogStore) Cleanup() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Delete by age
	cutoff := time.Now().AddDate(0, 0, -s.maxAgeDays)
	_, err := s.db.Exec("DELETE FROM system_logs WHERE timestamp < ?", cutoff.Format("2006-01-02 15:04:05"))
	if err != nil {
		return fmt.Errorf("cleanup by age: %w", err)
	}

	// Delete by count (keep only maxEntries)
	_, err = s.db.Exec(`
		DELETE FROM system_logs WHERE id NOT IN (
			SELECT id FROM system_logs ORDER BY timestamp DESC LIMIT ?
		)
	`, s.maxEntries)
	if err != nil {
		return fmt.Errorf("cleanup by count: %w", err)
	}

	return nil
}

// Count returns the number of log entries.
func (s *LogStore) Count() (int, error) {
	var count int
	err := s.db.QueryRow("SELECT COUNT(*) FROM system_logs").Scan(&count)
	return count, err
}
