package store

import (
	"time"

	"github.com/hattiebot/hattiebot/internal/health"
)

// lastHealthOK tracks when the DB was last healthy
var lastHealthOK time.Time

// HealthCheck returns the health status of the database.
func (db *DB) HealthCheck() health.ComponentHealth {
	h := health.ComponentHealth{
		Name:   "database",
		Status: "ok",
	}

	// Ping the database
	if err := db.Ping(); err != nil {
		h.Status = "error"
		h.Message = err.Error()
		h.LastError = time.Now()
		return h
	}

	// Check we can query
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count); err != nil {
		h.Status = "degraded"
		h.Message = "cannot query messages: " + err.Error()
		h.LastError = time.Now()
		return h
	}

	lastHealthOK = time.Now()
	h.LastOK = lastHealthOK
	return h
}

// GetMessageCount returns the number of messages in the database.
func (db *DB) GetMessageCount() (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM messages").Scan(&count)
	return count, err
}
