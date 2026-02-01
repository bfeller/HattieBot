package agent

import (
	"context"
	"testing"

	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/openrouter"
	"github.com/hattiebot/hattiebot/internal/store"
)

type MockSubmindLLM struct {
	TurnCount int
}

func (m *MockSubmindLLM) ChatCompletion(ctx context.Context, msgs []openrouter.Message) (string, error) {
	return "response", nil
}

func (m *MockSubmindLLM) ChatCompletionWithTools(ctx context.Context, msgs []openrouter.Message, tools []openrouter.ToolDefinition) (string, []openrouter.ToolCall, error) {
	m.TurnCount++
	// First turn: call a tool
	if m.TurnCount == 1 {
		return "Thought: I need to use a tool.", []openrouter.ToolCall{
			{
				ID: "call_1",
				Function: struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				}{
					Name:      "allowed_tool",
					Arguments: "{}",
				},
			},
		}, nil
	}
	// Second turn: finish
	return "I am done.", nil, nil
}

func (m *MockSubmindLLM) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, nil
}

type MockSubmindExecutor struct{}

func (m *MockSubmindExecutor) Execute(ctx context.Context, name, args string) (string, error) {
	return "tool_output", nil
}

func TestSubMindRun(t *testing.T) {
	mockLLM := &MockSubmindLLM{}
	mockExec := &MockSubmindExecutor{}

	cfg := core.SubMindConfig{
		Name:         "test_mode",
		SystemPrompt: "sys",
		AllowedTools: []string{"allowed_tool"},
		MaxTurns:     5,
	}

	sm := &SubMind{
		Config:   cfg,
		Client:   mockLLM,
		Executor: mockExec,
		LogStore: nil,
	}

	task := "do something"
	result, err := sm.Run(context.Background(), task)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if !result.Success {
		t.Errorf("Expected success, got failure")
	}
	if result.Turns != 2 {
		t.Errorf("Expected 2 turns, got %d", result.Turns)
	}
	if result.Output != "I am done." {
		t.Errorf("Unexpected output: %s", result.Output)
	}
}

func TestSubMindMaxTurns(t *testing.T) {
	// Mock that always returns tool calls -> infinite loop
	mockLLM := &MockSubmindLLMInternal{
		ResponseFunc: func(turn int) (string, []openrouter.ToolCall) {
			return "looping", []openrouter.ToolCall{
				{
					ID: "call",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{Name: "allowed_tool", Arguments: "{}"},
				},
			}
		},
	}
	mockExec := &MockSubmindExecutor{}

	cfg := core.SubMindConfig{
		Name:         "loop_mode",
		SystemPrompt: "sys",
		AllowedTools: []string{"allowed_tool"},
		MaxTurns:     3,
	}

	sm := &SubMind{
		Config:   cfg,
		Client:   mockLLM,
		Executor: mockExec,
	}

	result, _ := sm.Run(context.Background(), "task")

	if result.Turns != 3 {
		t.Errorf("Expected 3 turns (max), got %d", result.Turns)
	}
	if !result.Truncated {
		t.Errorf("Expected Truncated=true")
	}
	// Partial success logic: if truncated, output is the last context content?
	// Actually submind.go implementation returns output from last assistant message if success/truncated.
	// But if looping tool calls, last message is tool result... so output might be empty or partial?
	// Let's check submind.go logic logic for loop break.
	// If max turns reached, it returns Result{Success: false, Truncated: true}.
	// Wait, did I implement success=true on truncation?
	// Let's check submind.go.
}

func TestSubMindRunWithSession_persists(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, _ = db.GetOrCreateUser(ctx, "u1", "", "test")

	mockLLM := &MockSubmindLLM{}
	mockExec := &MockSubmindExecutor{}
	cfg := core.SubMindConfig{
		Name:         "test_mode",
		SystemPrompt: "sys",
		AllowedTools: []string{"allowed_tool"},
		MaxTurns:     5,
	}
	sm := &SubMind{Config: cfg, Client: mockLLM, Executor: mockExec}

	id, err := db.CreateSubmindSession(ctx, "u1", "test_mode", "do something", "sys")
	if err != nil {
		t.Fatal(err)
	}

	result, err := sm.RunWithSession(ctx, "do something", id, "u1", db)
	if err != nil {
		t.Fatalf("RunWithSession failed: %v", err)
	}
	if !result.Success || result.Turns != 2 || result.SessionID != id {
		t.Errorf("result: success=%v turns=%d session_id=%d", result.Success, result.Turns, result.SessionID)
	}

	ses, err := db.GetSubmindSession(ctx, id, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if ses.Status != "completed" || ses.Turns != 2 || ses.ResultOutput != "I am done." {
		t.Errorf("session after run: status=%s turns=%d output=%s", ses.Status, ses.Turns, ses.ResultOutput)
	}
}

func TestSubMindRunWithSession_resume(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, _ = db.GetOrCreateUser(ctx, "u1", "", "test")

	// Mock that on first call (resume turn 2) returns no tool calls -> done
	mockLLM := &MockSubmindLLMInternal{
		ResponseFunc: func(turn int) (string, []openrouter.ToolCall) {
			return "Resumed and done.", nil
		},
	}
	mockExec := &MockSubmindExecutor{}
	cfg := core.SubMindConfig{
		Name:         "resume_mode",
		SystemPrompt: "sys",
		AllowedTools: []string{"allowed_tool"},
		MaxTurns:     5,
	}
	sm := &SubMind{Config: cfg, Client: mockLLM, Executor: mockExec}

	id, _ := db.CreateSubmindSession(ctx, "u1", "resume_mode", "task", "sys")
	// Seed with one turn done: system, user, assistant (tool call), tool
	tc := core.ToolCall{ID: "c1"}
	tc.Function.Name = "allowed_tool"
	tc.Function.Arguments = "{}"
	msgs := []core.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "task"},
		{Role: "assistant", Content: "thinking", ToolCalls: []core.ToolCall{tc}},
		{Role: "tool", Content: "ok", ToolCallID: "c1"},
	}
	_ = db.UpdateSubmindSession(ctx, id, msgs, 1, "running", "", "")

	result, err := sm.RunWithSession(ctx, "task", id, "u1", db)
	if err != nil {
		t.Fatalf("RunWithSession resume failed: %v", err)
	}
	if !result.Success || result.Turns != 2 {
		t.Errorf("resume result: success=%v turns=%d", result.Success, result.Turns)
	}
	ses, _ := db.GetSubmindSession(ctx, id, "u1")
	if ses.Status != "completed" || ses.ResultOutput != "Resumed and done." {
		t.Errorf("session after resume: status=%s output=%s", ses.Status, ses.ResultOutput)
	}
}

// Helper mock with closure
type MockSubmindLLMInternal struct {
	ResponseFunc func(turn int) (string, []openrouter.ToolCall)
	TurnCount    int
}

func (m *MockSubmindLLMInternal) ChatCompletion(ctx context.Context, msgs []openrouter.Message) (string, error) {
	return "", nil
}
func (m *MockSubmindLLMInternal) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, nil
}
func (m *MockSubmindLLMInternal) ChatCompletionWithTools(ctx context.Context, msgs []openrouter.Message, tools []openrouter.ToolDefinition) (string, []openrouter.ToolCall, error) {
	m.TurnCount++
	content, calls := m.ResponseFunc(m.TurnCount)
	return content, calls, nil
}
