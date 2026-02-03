package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/gateway"
	"github.com/hattiebot/hattiebot/internal/store"
)

// Runner checks for due plans and executes them.
type Runner struct {
	DB           *store.DB
	ToolExecutor core.ToolExecutor
	Router       *gateway.Router // For proactive reminder delivery
	Interval     time.Duration
	stop         chan struct{}
}

func NewRunner(db *store.DB) *Runner {
	return &Runner{
		DB:       db,
		Interval: 1 * time.Minute,
		stop:     make(chan struct{}),
	}
}

// Start begins the background scheduler loop.
func (r *Runner) Start() {
	go func() {
		ticker := time.NewTicker(r.Interval)
		defer ticker.Stop()

		log.Println("[SCHEDULER] Started, checking every", r.Interval)

		for {
			select {
			case <-ticker.C:
				r.checkAndRun()
			case <-r.stop:
				log.Println("[SCHEDULER] Stopped")
				return
			}
		}
	}()
}

// Stop halts the scheduler.
func (r *Runner) Stop() {
	close(r.stop)
}

func (r *Runner) checkAndRun() {
	ctx := context.Background()
	// Lock for 5 minutes (if crash, other nodes pick up after 5m)
	plans, err := r.DB.ClaimDuePlans(ctx, 5*time.Minute)
	if err != nil {
		log.Printf("[SCHEDULER] Error claiming plans: %v", err)
		return
	}

	for _, p := range plans {
		log.Printf("[SCHEDULER] Executing plan %d: %s (%s)", p.ID, p.Description, p.ActionType)
		r.executePlan(ctx, p)

		// Mark as run (updates next_run_at for recurring)
		if err := r.DB.MarkPlanRun(ctx, p.ID, p.ScheduleType); err != nil {
			log.Printf("[SCHEDULER] Error marking plan %d as run: %v", p.ID, err)
		}
	}
}

func (r *Runner) executePlan(ctx context.Context, p store.ScheduledPlan) {
	// Inject user_id from the plan into context so tool policies work
	ctx = context.WithValue(ctx, "user_id", p.UserID)

	switch p.ActionType {
	case "remind":
		// For reminders, we log, store, and proactively deliver to the user
		log.Printf("[SCHEDULER] REMINDER: %s", p.Description)
		msg := "[Scheduled Reminder] " + p.Description
		// Store as a system message so it appears in history
		r.DB.InsertMessage(ctx, "assistant", msg, "", "system", "scheduler", "scheduler", "", "", "")
		// Proactively send to user via their preferred channel (Nextcloud Talk, admin_term, etc.)
		if r.Router != nil && p.UserID != "" {
			if err := r.Router.RouteMessage(ctx, p.UserID, msg, ""); err != nil {
				log.Printf("[SCHEDULER] Failed to route reminder to %s: %v", p.UserID, err)
			}
		}

	case "execute_tool":
		// Parse payload for tool name and args
		if p.ActionPayload == "" {
			log.Printf("[SCHEDULER] execute_tool plan %d has empty payload", p.ID)
			errMsg := "[Scheduled Tool Execution] Error: execute_tool requires action_payload with tool and args"
			r.DB.InsertMessage(ctx, "assistant", errMsg, "", "system", "scheduler", "scheduler", "", "", "")
			return
		}
		var payload struct {
			Tool string          `json:"tool"`
			Args json.RawMessage `json:"args"`
		}
		if err := json.Unmarshal([]byte(p.ActionPayload), &payload); err != nil {
			log.Printf("[SCHEDULER] Invalid tool payload for plan %d: %v", p.ID, err)
			// Store error message so user knows what went wrong
			errMsg := fmt.Sprintf("[Scheduled Tool Execution] Error: Invalid tool payload - %v", err)
			r.DB.InsertMessage(ctx, "assistant", errMsg, "", "system", "scheduler", "scheduler", "", "", "")
			return
		}
		log.Printf("[SCHEDULER] Executing tool: %s", payload.Tool)
		if r.ToolExecutor == nil {
			log.Printf("[SCHEDULER] ToolExecutor not configured, skipping tool execution")
			errMsg := "[Scheduled Tool Execution] Error: ToolExecutor not configured"
			r.DB.InsertMessage(ctx, "assistant", errMsg, "", "system", "scheduler", "scheduler", "", "", "")
			return
		}
		result, err := r.ToolExecutor.Execute(ctx, payload.Tool, string(payload.Args))

		var msg string
		if err != nil {
			log.Printf("[SCHEDULER] Tool %s failed: %v", payload.Tool, err)
			msg = fmt.Sprintf("[Scheduled Tool Execution] Tool **%s** failed:\n```\n%s\n```", payload.Tool, err.Error())
		} else {
			log.Printf("[SCHEDULER] Tool %s completed successfully", payload.Tool)
			msg = fmt.Sprintf("[Scheduled Tool Execution] Tool **%s** completed:\n```json\n%s\n```", payload.Tool, result)
		}
		// Store result so user can see it
		r.DB.InsertMessage(ctx, "assistant", msg, "", "system", "scheduler", "scheduler", "", "", "")

	case "agent_prompt":
		var payload struct {
			Prompt     string `json:"prompt"`
			Autonomous bool   `json:"autonomous"`
		}
		if p.ActionPayload != "" {
			if err := json.Unmarshal([]byte(p.ActionPayload), &payload); err != nil {
				log.Printf("[SCHEDULER] Invalid agent_prompt payload for plan %d: %v", p.ID, err)
				errMsg := fmt.Sprintf("[Scheduled Task] Error: Invalid agent_prompt payload - %v", err)
				r.DB.InsertMessage(ctx, "assistant", errMsg, "", "system", "scheduler", "scheduler", "", "", "")
				return
			}
		}
		if payload.Prompt == "" {
			payload.Prompt = p.Description
		}
		log.Printf("[SCHEDULER] AGENT_PROMPT: %s (autonomous=%v)", payload.Prompt, payload.Autonomous)
		if r.Router == nil {
			log.Printf("[SCHEDULER] Router not configured, cannot push agent prompt")
			r.DB.InsertMessage(ctx, "assistant", "[Scheduled Task] Error: Router not configured", "", "system", "scheduler", "scheduler", "", "", "")
			return
		}
		if !r.Router.PushAgentPrompt(ctx, p.UserID, payload.Prompt, payload.Autonomous, p.ID) {
			log.Printf("[SCHEDULER] Ingress buffer full, agent prompt dropped for plan %d", p.ID)
			r.DB.InsertMessage(ctx, "assistant", "[Scheduled Task] Error: Ingress buffer full, task deferred", "", "system", "scheduler", "scheduler", "", "", "")
		}

	default:
		log.Printf("[SCHEDULER] Unknown action type: %s", p.ActionType)
		msg := fmt.Sprintf("[Scheduled Task] Unknown action type: %s", p.ActionType)
		r.DB.InsertMessage(ctx, "assistant", msg, "", "system", "scheduler", "scheduler", "", "", "")
	}
}
