package tools

import (
	"context"
	"encoding/json"

	"github.com/hattiebot/hattiebot/internal/health"
	"github.com/hattiebot/hattiebot/internal/store"
)

// ReadLogsArgs represents the arguments for the read_logs tool.
type ReadLogsArgs struct {
	Level     string `json:"level,omitempty"`     // error, warn, info
	Component string `json:"component,omitempty"` // db, llm, gateway, compactor
	Limit     int    `json:"limit,omitempty"`     // max entries to return
}

// ReadLogsResult represents the result of the read_logs tool.
type ReadLogsResult struct {
	Logs  []health.LogEntry `json:"logs"`
	Count int               `json:"count"`
}

// ReadLogsTool retrieves recent logs with optional filtering.
func ReadLogsTool(ctx context.Context, logStore *store.LogStore, argsJSON string) (string, error) {
	var args ReadLogsArgs
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		result := map[string]string{"error": "invalid arguments: " + err.Error()}
		out, _ := json.Marshal(result)
		return string(out), nil
	}

	// Default limit
	if args.Limit <= 0 {
		args.Limit = 50
	}
	if args.Limit > 200 {
		args.Limit = 200
	}

	logs, err := logStore.GetLogs(args.Level, args.Component, args.Limit)
	if err != nil {
		result := map[string]string{"error": err.Error()}
		out, _ := json.Marshal(result)
		return string(out), nil
	}

	result := ReadLogsResult{
		Logs:  logs,
		Count: len(logs),
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	return string(out), nil
}
