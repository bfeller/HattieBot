package openrouter

import (
	"sync"
	"time"

	"github.com/hattiebot/hattiebot/internal/health"
)

// ClientHealth tracks LLM client health state.
type ClientHealth struct {
	mu            sync.RWMutex
	lastSuccess   time.Time
	lastError     time.Time
	lastErrorMsg  string
	successCount  int64
	errorCount    int64
}

// Global health tracker for the client
var clientHealth = &ClientHealth{}

// RecordSuccess records a successful API call.
func (c *ClientHealth) RecordSuccess() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastSuccess = time.Now()
	c.successCount++
}

// RecordError records a failed API call.
func (c *ClientHealth) RecordError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastError = time.Now()
	c.lastErrorMsg = err.Error()
	c.errorCount++
}

// HealthCheck returns the health status of the LLM client.
func (c *Client) HealthCheck() health.ComponentHealth {
	clientHealth.mu.RLock()
	defer clientHealth.mu.RUnlock()

	h := health.ComponentHealth{
		Name:   "llm_client",
		Status: "ok",
		LastOK: clientHealth.lastSuccess,
	}

	// Check if we've had recent errors
	if !clientHealth.lastError.IsZero() {
		// If last error is more recent than last success, we're in trouble
		if clientHealth.lastError.After(clientHealth.lastSuccess) {
			h.Status = "error"
			h.Message = clientHealth.lastErrorMsg
			h.LastError = clientHealth.lastError
		} else if time.Since(clientHealth.lastError) < 5*time.Minute {
			// Recent error but recovered
			h.Status = "degraded"
			h.Message = "recent error: " + clientHealth.lastErrorMsg
			h.LastError = clientHealth.lastError
		}
	}

	// If we've never had a successful call, status is unknown
	if clientHealth.lastSuccess.IsZero() {
		h.Status = "unknown"
		h.Message = "no API calls yet"
	}

	return h
}

// GetHealth returns the global client health tracker.
func GetHealth() *ClientHealth {
	return clientHealth
}
