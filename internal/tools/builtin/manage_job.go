package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hattiebot/hattiebot/internal/openrouter"
	"github.com/hattiebot/hattiebot/internal/store"
)

type ManageJobTool struct {
	DB *store.DB
}

func NewManageJobTool(db *store.DB) *ManageJobTool {
	return &ManageJobTool{DB: db}
}

func (t *ManageJobTool) Name() string {
	return "manage_job"
}

func (t *ManageJobTool) Definition() openrouter.ToolDefinition {
	return openrouter.ToolDefinition{
		Type: "function",
		Function: openrouter.FunctionSpec{
			Name:        "manage_job",
			Description: "Manage long-running tasks (Epic Memory). Use this to track what you are working on across sessions.",
			Parameters: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"action":         map[string]interface{}{"type": "string", "enum": []string{"create", "update", "list"}, "description": "Action to perform"},
					"title":          map[string]interface{}{"type": "string", "description": "Job title (for create)"},
					"description":    map[string]interface{}{"type": "string", "description": "Job description (for create)"},
					"id":             map[string]interface{}{"type": "integer", "description": "Job ID (for update)"},
					"status":         map[string]interface{}{"type": "string", "enum": []string{"open", "blocked", "closed"}, "description": "New status (for update/list)"},
					"blocked_reason": map[string]interface{}{"type": "string", "description": "Reason if blocked (for update)"},
					"duration":       map[string]interface{}{"type": "string", "description": "Duration for snooze (e.g. 1h, 2d)"},
				},
				"required": []string{"action"},
			},
		},
	}
}

func (t *ManageJobTool) Execute(ctx context.Context, argsJSON string) (string, error) {
	userID, err := getUserID(ctx)
	if err != nil {
		return ErrJSON(err), nil
	}
	var args struct {
		Action        string `json:"action"`
		Title         string `json:"title"`
		Description   string `json:"description"`
		ID            int64  `json:"id"`
		Status        string `json:"status"`
		BlockedReason string `json:"blocked_reason"`
		Duration      string `json:"duration"` // For snooze: "1h", "2d", etc.
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return ErrJSON(err), nil
	}
	switch args.Action {
	case "create":
		id, err := t.DB.CreateJob(ctx, userID, args.Title, args.Description)
		if err != nil {
			return ErrJSON(err), nil
		}
		return fmt.Sprintf(`{"id": %d, "status": "created"}`, id), nil
	case "update":
		err := t.DB.UpdateJobStatus(ctx, args.ID, args.Status, args.BlockedReason)
		if err != nil {
			return ErrJSON(err), nil
		}
		return `{"status": "updated"}`, nil
	case "snooze":
		// Parse duration string (e.g., "1h", "2d", "30m")
		duration, err := parseDuration(args.Duration)
		if err != nil {
			return ErrJSON(fmt.Errorf("invalid duration '%s': %w", args.Duration, err)), nil
		}
		until := time.Now().Add(duration)
		if err := t.DB.SnoozeJob(ctx, args.ID, until); err != nil {
			return ErrJSON(err), nil
		}
		return fmt.Sprintf(`{"status": "snoozed", "until": "%s"}`, until.Format(time.RFC3339)), nil
	case "list":
		jobs, err := t.DB.ListJobs(ctx, userID, args.Status)
		if err != nil {
			return ErrJSON(err), nil
		}
		b, _ := json.Marshal(jobs)
		return string(b), nil
	default:
		return ErrJSON(fmt.Errorf("unknown action: %s", args.Action)), nil
	}
}
