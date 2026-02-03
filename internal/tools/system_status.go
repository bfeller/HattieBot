package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hattiebot/hattiebot/internal/gateway"
	"github.com/hattiebot/hattiebot/internal/health"
	"github.com/hattiebot/hattiebot/internal/memory"
	"github.com/hattiebot/hattiebot/internal/openrouter"
	"github.com/hattiebot/hattiebot/internal/store"
)

// SystemStatus contains comprehensive system state information.
type SystemStatus struct {
	Timestamp         time.Time                         `json:"timestamp"`
	MessageCount      int                               `json:"message_count"`
	MemoryChunkCount  int                               `json:"memory_chunk_count,omitempty"`
	LogEntryCount     int                               `json:"log_entry_count"`
	TokenBudget       string                            `json:"token_budget"`
	RegisteredTools   []string                          `json:"registered_tools"`
	ActiveChannels    []string                          `json:"active_channels"`
	Components        map[string]health.ComponentHealth `json:"components"`
	RecentErrors      []health.LogEntry                 `json:"recent_errors,omitempty"`
	LastReflection    time.Time                         `json:"last_reflection,omitempty"`
}

// SystemStatusGatherer collects system status from various components.
type SystemStatusGatherer struct {
	DB           *store.DB
	LogStore     *store.LogStore
	Gateway      *gateway.Gateway
	Compactor    *memory.Compactor
	Client       *openrouter.Client
	HealthReg    *health.Registry
	TokenBudget  int
}

// Gather collects comprehensive system status.
func (g *SystemStatusGatherer) Gather(ctx context.Context) (SystemStatus, error) {
	tokenBudgetStr := "Unlimited"
	if g.TokenBudget > 0 {
		tokenBudgetStr = fmt.Sprintf("%d", g.TokenBudget)
	}

	status := SystemStatus{
		Timestamp:    time.Now(),
		TokenBudget:  tokenBudgetStr,
		Components:   make(map[string]health.ComponentHealth),
	}

	// Message count
	if g.DB != nil {
		if count, err := g.DB.GetMessageCount(); err == nil {
			status.MessageCount = count
		}
	}

	// Log entry count
	if g.LogStore != nil {
		if count, err := g.LogStore.Count(); err == nil {
			status.LogEntryCount = count
		}

		// Recent errors
		if errors, err := g.LogStore.GetErrors(10); err == nil {
			status.RecentErrors = errors
		}
	}

	// Registered tools
	if g.DB != nil {
		if tools, err := g.DB.AllTools(ctx); err == nil {
			for _, t := range tools {
				status.RegisteredTools = append(status.RegisteredTools, t.Name)
			}
		}
	}

	// Active channels
	if g.Gateway != nil {
		status.ActiveChannels = g.Gateway.GetChannelNames()
	}

	// Component health
	if g.HealthReg != nil {
		report := g.HealthReg.Check()
		status.Components = report.Components
	} else {
		// Manual health checks if no registry
		if g.DB != nil {
			status.Components["database"] = g.DB.HealthCheck()
		}
		if g.Client != nil {
			status.Components["llm_client"] = g.Client.HealthCheck()
		}
		if g.Gateway != nil {
			status.Components["gateway"] = g.Gateway.HealthCheck()
		}
		if g.Compactor != nil {
			status.Components["compactor"] = g.Compactor.HealthCheck()
		}
	}

	return status, nil
}

// SystemStatusTool executes the system_status tool.
func SystemStatusTool(ctx context.Context, gatherer *SystemStatusGatherer) (string, error) {
	status, err := gatherer.Gather(ctx)
	if err != nil {
		result := map[string]string{"error": err.Error()}
		out, _ := json.Marshal(result)
		return string(out), nil
	}

	out, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		result := map[string]string{"error": err.Error()}
		out, _ := json.Marshal(result)
		return string(out), nil
	}
	return string(out), nil
}
