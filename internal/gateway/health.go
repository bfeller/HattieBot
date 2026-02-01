package gateway

import (
	"time"

	"github.com/hattiebot/hattiebot/internal/health"
)

// HealthCheck returns the health status of the gateway.
func (g *Gateway) HealthCheck() health.ComponentHealth {
	g.mu.RLock()
	defer g.mu.RUnlock()

	h := health.ComponentHealth{
		Name:   "gateway",
		Status: "ok",
		LastOK: time.Now(),
	}

	// Check if we have any channels registered
	if len(g.channels) == 0 {
		h.Status = "degraded"
		h.Message = "no channels registered"
		return h
	}

	// Report channel count
	h.Message = ""
	for name := range g.channels {
		if h.Message != "" {
			h.Message += ", "
		}
		h.Message += name
	}
	h.Message = "channels: " + h.Message

	return h
}

// GetChannelNames returns the names of all registered channels.
func (g *Gateway) GetChannelNames() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	names := make([]string, 0, len(g.channels))
	for name := range g.channels {
		names = append(names, name)
	}
	return names
}
