package tools

import (
	"context"
	"testing"
)

type MockExecutor struct {
	CalledWith string
}

func (m *MockExecutor) Execute(ctx context.Context, name, args string) (string, error) {
	m.CalledWith = name
	return "ok", nil
}

func TestFilteredExecutor(t *testing.T) {
	mock := &MockExecutor{}
	allowed := []string{"allowed_tool"}
	f := NewFilteredExecutor(mock, allowed)
	ctx := context.Background()

	// Test allowed tool
	resp, err := f.Execute(ctx, "allowed_tool", "{}")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if resp != "ok" {
		t.Errorf("Expected 'ok', got %s", resp)
	}
	if mock.CalledWith != "allowed_tool" {
		t.Errorf("Mock not called with correct tool")
	}

	// Test disallowed tool
	mock.CalledWith = ""
	resp, err = f.Execute(ctx, "forbidden_tool", "{}")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if resp != `{"error": "tool not available in this sub-mind mode"}` {
		t.Errorf("Expected error JSON, got %s", resp)
	}
	if mock.CalledWith != "" {
		t.Errorf("Mock should not have been called")
	}

	// Test blocked tool (spawn_submind) even if explicitly allowed (should be blocked by FilterToolDefs but executor also blocks)
	f2 := NewFilteredExecutor(mock, []string{"spawn_submind"})
	resp, err = f2.Execute(ctx, "spawn_submind", "{}")
	if resp != `{"error": "tool not available in sub-mind context"}` {
		t.Errorf("Expected blocked error, got %s", resp)
	}
}
