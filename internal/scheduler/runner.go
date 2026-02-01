package scheduler

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/store"
)

// Runner checks for due plans and executes them.
type Runner struct {
	DB           *store.DB
	ToolExecutor core.ToolExecutor
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
		// For reminders, we log and store a message for the user
		log.Printf("[SCHEDULER] REMINDER: %s", p.Description)
		// Store as a system message so user sees it on next chat.
		// Use "system" sender and "scheduler" channel.
		r.DB.InsertMessage(ctx, "assistant", "[Scheduled Reminder] "+p.Description, "", "system", "scheduler", "scheduler", "", "", "")

	case "execute_tool":
		// Parse payload for tool name and args
		var payload struct {
			Tool string          `json:"tool"`
			Args json.RawMessage `json:"args"`
		}
		if err := json.Unmarshal([]byte(p.ActionPayload), &payload); err != nil {
			log.Printf("[SCHEDULER] Invalid tool payload for plan %d: %v", p.ID, err)
			return
		}
		log.Printf("[SCHEDULER] Executing tool: %s", payload.Tool)
		if r.ToolExecutor == nil {
			log.Printf("[SCHEDULER] ToolExecutor not configured, skipping tool execution")
			return
		}
		result, err := r.ToolExecutor.Execute(ctx, payload.Tool, string(payload.Args))
		if err != nil {
			log.Printf("[SCHEDULER] Tool %s failed: %v", payload.Tool, err)
			return
		}
		log.Printf("[SCHEDULER] Tool %s completed: %s", payload.Tool, result)

	default:
		log.Printf("[SCHEDULER] Unknown action type: %s", p.ActionType)
	}
}
