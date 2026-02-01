package health

import (
	"sync"
	"time"
)

// ComponentHealth represents the health status of a single component.
type ComponentHealth struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`  // "ok", "degraded", "error"
	Message   string    `json:"message,omitempty"`
	LastOK    time.Time `json:"last_ok"`
	LastError time.Time `json:"last_error,omitempty"`
}

// HealthReport aggregates health from all components.
type HealthReport struct {
	Timestamp  time.Time                  `json:"timestamp"`
	Components map[string]ComponentHealth `json:"components"`
	Errors     []LogEntry                 `json:"recent_errors"`
}

// LogEntry represents a structured log entry.
type LogEntry struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`     // error, warn, info
	Component string    `json:"component"` // db, llm, gateway, compactor
	Message   string    `json:"message"`
}

// HealthChecker interface for components to implement.
type HealthChecker interface {
	HealthCheck() ComponentHealth
}

// Registry holds health checkers for all components.
type Registry struct {
	mu       sync.RWMutex
	checkers map[string]HealthChecker
}

// NewRegistry creates a new health registry.
func NewRegistry() *Registry {
	return &Registry{
		checkers: make(map[string]HealthChecker),
	}
}

// Register adds a component health checker.
func (r *Registry) Register(name string, checker HealthChecker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checkers[name] = checker
}

// Check runs all health checks and returns a report.
func (r *Registry) Check() HealthReport {
	r.mu.RLock()
	defer r.mu.RUnlock()

	report := HealthReport{
		Timestamp:  time.Now(),
		Components: make(map[string]ComponentHealth),
	}

	for name, checker := range r.checkers {
		report.Components[name] = checker.HealthCheck()
	}

	return report
}

// GetStatus returns the overall system status.
func (r *Registry) GetStatus() string {
	report := r.Check()
	for _, c := range report.Components {
		if c.Status == "error" {
			return "error"
		}
	}
	for _, c := range report.Components {
		if c.Status == "degraded" {
			return "degraded"
		}
	}
	return "ok"
}
