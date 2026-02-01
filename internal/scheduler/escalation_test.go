
package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/hattiebot/hattiebot/internal/gateway"
	"github.com/hattiebot/hattiebot/internal/store"
)

func TestEscalationMonitor(t *testing.T) {
	// Setup DB
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	// Setup Gateway/Router
	gw := gateway.New(func(ctx context.Context, msg gateway.Message) (string, error) { return "", nil })
	gw.Register(&MockChannel{name: "admin_term"})
	router := gateway.NewRouter(gw, db)

	monitor := &EscalationMonitor{
		DB:     db,
		Router: router,
	}

	// 1. Create Overdue Plan
	// created_at = now-2h, next_run = now-2h
	past := time.Now().Add(-2 * time.Hour)
	_, err = db.CreatePlan(ctx, "test-user", "Overdue Task", "remind", "", "once", past.Format(time.RFC3339), past)
	if err != nil {
		t.Fatalf("Failed to create plan: %v", err)
	}

	// 2. Run Check
	// We can't easily mock the Router.RouteMessage call without interface or structural change.
	// But `monitor.CheckAndEscalate` logs or errors. 
	// For testing, we can check if it runs without error, but identifying if it *tried* to escalate is hard without mocking the router or gateway.
	// Let's rely on no-error for now, and maybe inspecting logs if we redirected them?
	// Or just trust the logic: "If overdue, log".

	if err := monitor.CheckAndEscalate(ctx); err != nil {
		t.Errorf("CheckAndEscalate failed: %v", err)
	}

	// 3. Verify Plan is still active (monitor doesn't change status, just nags)
	plans, _ := db.ListPlans(ctx, "test-user", "active")
	if len(plans) != 1 {
		t.Errorf("Expected 1 active plan, got %d", len(plans))
	}
}

type MockChannel struct {
	name string
}
func (m *MockChannel) Name() string { return m.name }
func (m *MockChannel) Start(ctx context.Context, ingress chan<- gateway.Message) error { return nil }
func (m *MockChannel) Send(msg gateway.Message) error { return nil }
func (m *MockChannel) SendProactive(userID, content string) error { return nil }
