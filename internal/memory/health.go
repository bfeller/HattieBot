package memory

import (
	"sync"
	"time"

	"github.com/hattiebot/hattiebot/internal/health"
)

// CompactorHealth tracks compaction state.
type CompactorHealth struct {
	mu              sync.RWMutex
	lastCompaction  time.Time
	messagesCompacted int
	lastError       time.Time
	lastErrorMsg    string
}

// Global health tracker for the compactor
var compactorHealth = &CompactorHealth{}

// RecordCompaction records a successful compaction.
func (h *CompactorHealth) RecordCompaction(messagesCompacted int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastCompaction = time.Now()
	h.messagesCompacted = messagesCompacted
}

// RecordError records a compaction error.
func (h *CompactorHealth) RecordError(err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.lastError = time.Now()
	h.lastErrorMsg = err.Error()
}

// HealthCheck returns the health status of the compactor.
func (c *Compactor) HealthCheck() health.ComponentHealth {
	compactorHealth.mu.RLock()
	defer compactorHealth.mu.RUnlock()

	h := health.ComponentHealth{
		Name:   "compactor",
		Status: "ok",
		LastOK: compactorHealth.lastCompaction,
	}

	if !compactorHealth.lastError.IsZero() {
		if compactorHealth.lastError.After(compactorHealth.lastCompaction) {
			h.Status = "error"
			h.Message = compactorHealth.lastErrorMsg
			h.LastError = compactorHealth.lastError
		}
	}

	if compactorHealth.lastCompaction.IsZero() {
		h.Message = "no compaction yet"
	} else {
		h.Message = "last compacted " + compactorHealth.lastCompaction.Format(time.RFC3339)
	}

	return h
}

// GetHealth returns the global compactor health tracker.
func GetHealth() *CompactorHealth {
	return compactorHealth
}

// LastCompaction returns the time of the last compaction.
func (c *Compactor) LastCompaction() time.Time {
	compactorHealth.mu.RLock()
	defer compactorHealth.mu.RUnlock()
	return compactorHealth.lastCompaction
}

// MessagesCompacted returns the number of messages compacted in the last run.
func (c *Compactor) MessagesCompacted() int {
	compactorHealth.mu.RLock()
	defer compactorHealth.mu.RUnlock()
	return compactorHealth.messagesCompacted
}
