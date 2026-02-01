package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hattiebot/hattiebot/internal/gateway"
	"github.com/hattiebot/hattiebot/internal/store"
)

// EscalationMonitor checks for overdue plans and blocked jobs, escalating them if needed.
type EscalationMonitor struct {
	DB      *store.DB
	Router  *gateway.Router
}

// Start begins a periodic check.
func (e *EscalationMonitor) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if err := e.CheckAndEscalate(ctx); err != nil {
					log.Printf("[ESCALATION] Check failed: %v", err)
				}
			}
		}
	}()
}

// CheckAndEscalate finds items needing attention.
func (e *EscalationMonitor) CheckAndEscalate(ctx context.Context) error {
	// 1. Check Overdue Scheduled Plans
	// Use GetDuePlans which returns active plans with next_run_at <= Now
	// We might want plans overdue by a threshold.
	// But GetDuePlans returns anything ready. "Overdue" implies ignored.
	// Let's filter in memory for now: if NextRunAt is < (Now - 1h).
	
	duePlans, err := e.DB.GetDuePlans(ctx)
	if err != nil {
		return err
	}

	now := time.Now()
	threshold := now.Add(-1 * time.Hour)

	for _, p := range duePlans {
		if p.NextRunAt == nil {
			continue
		}
		
		if p.NextRunAt.Before(threshold) {
			// Overdue logic
			msg := fmt.Sprintf("Plan #%d '%s' is overdue (was set for %s).", p.ID, p.Description, p.NextRunAt.Format(time.RFC3339))
			
			log.Printf("[ESCALATION] Escalating overdue plan %d", p.ID)
			if e.Router != nil {
				if err := e.Router.RouteMessage(ctx, "admin", msg, "urgent"); err != nil {
					log.Printf("[ESCALATION] Failed to route message: %v", err)
				}
			}
		}
	}

	return nil
}
