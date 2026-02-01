package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings" // Added strings
	"testing"

	"github.com/hattiebot/hattiebot/internal/config"
	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/gateway"
	"github.com/hattiebot/hattiebot/internal/openrouter"
	"github.com/hattiebot/hattiebot/internal/store"
	// "github.com/hattiebot/hattiebot/internal/tools" // Removed unused
)

// MockExecutor for testing tool calls
type MockExecutor struct {
	LastToolCalled string
	LastArgs       string
}

func (m *MockExecutor) Execute(ctx context.Context, name, argsJSON string) (string, error) {
	m.LastToolCalled = name
	m.LastArgs = argsJSON
	return "mock_result", nil
}

// SetupTestDB creates an in-memory SQLite DB for testing
func SetupTestDB(t *testing.T) *store.DB {
	ctx := context.Background()
	// Use a temporary file
	tmp := t.TempDir() + "/test.db"
	db, err := store.Open(ctx, tmp)
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return db
}

func TestContextManager_Stress(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	cm := &ContextManager{DB: db}

	// 1. Inject 100 messages
	for i := 0; i < 100; i++ {
		// Use distinct content
		content := fmt.Sprintf("msg %d", i)
		_, err := db.InsertMessage(ctx, "user", content, "", "user_id", "test_channel", "test_thread", "", "", "")
		if err != nil {
			t.Fatalf("insert failed: %v", err)
		}
	}

	// 2. Select History with limit 30
	hist, err := cm.SelectHistory(ctx, "test_thread")
	if err != nil {
		t.Fatalf("select failed: %v", err)
	}

	// 3. Assert limit respected (30)
	if len(hist) > 30 {
		t.Errorf("expected <=30 messages, got %d", len(hist))
	}

	// 4. Verify Content: Should have the LATEST messages.
	// We inserted 0..99.
	// If limit is 30, we expect roughly 70..99.
	// Let's check the LAST message is "msg 99"
	if len(hist) > 0 {
		lastMsg := hist[len(hist)-1]
		if lastMsg.Content != "msg 99" {
			t.Errorf("Expected last message 'msg 99', got '%s'", lastMsg.Content)
		}
		// Check the FIRST message returned (oldest in the window)
		// Should be around 70.
		firstMsg := hist[0]
		var id int
		fmt.Sscanf(firstMsg.Content, "msg %d", &id)
		if id < 50 {
			t.Errorf("Expected message id > 50 (recent), got %d from '%s'", id, firstMsg.Content)
		}
	} else {
		t.Error("History is empty")
	}
}

func TestPersistence_ToolCalls(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// 1. Simulate loop execution: Save Assistant+ToolCalls, then Tool Result
	toolCalls := []openrouter.ToolCall{
		{
			ID: "call_123",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: "test_tool", Arguments: "{}"},
		},
	}
	tcJSON, _ := json.Marshal(toolCalls)
	
	// Assistant thought
	_, err := db.InsertMessage(ctx, "assistant", "I will run a tool", "model", "bot", "channel", "thread_1", string(tcJSON), "", "")
	if err != nil {
		t.Fatal(err)
	}

	// Tool result
	_, err = db.InsertMessage(ctx, "tool", "result_data", "", "system", "channel", "thread_1", "", "", "call_123")
	if err != nil {
		t.Fatal(err)
	}

	// 2. Retrieve via ContextManager
	cm := &ContextManager{DB: db}
	hist, err := cm.SelectHistory(ctx, "thread_1")
	if err != nil {
		t.Fatal(err)
	}

	// 3. Verify retrieval
	foundToolCall := false
	foundResult := false
	for _, m := range hist {
		if m.Role == "assistant" && len(m.ToolCalls) > 0 {
			if m.ToolCalls[0].Function.Name == "test_tool" {
				foundToolCall = true
			}
		}
		if m.Role == "tool" && m.Content == "result_data" {
			foundResult = true
		}
	}

	if !foundToolCall {
		t.Error("Did not find persisted tool call in history")
	}
	if !foundResult {
		t.Error("Did not find persisted tool result in history")
	}
}

func TestThreadIsolation(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()
	cm := &ContextManager{DB: db}

	// Thread A
	db.InsertMessage(ctx, "user", "Message in Thread A", "", "user1", "chan1", "thread_a", "", "", "")
	
	// Thread B
	db.InsertMessage(ctx, "user", "Message in Thread B", "", "user2", "chan1", "thread_b", "", "", "")

	// Verify Thread A history
	histA, _ := cm.SelectHistory(ctx, "thread_a")
	if len(histA) != 1 || histA[0].Content != "Message in Thread A" {
		t.Errorf("Thread A isolation failed: got %v", histA)
	}

	// Verify Thread B history
	histB, _ := cm.SelectHistory(ctx, "thread_b")
	if len(histB) != 1 || histB[0].Content != "Message in Thread B" {
		t.Errorf("Thread B isolation failed: got %v", histB)
	}
}



func TestEpicMemory_JobBlocking(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	// 1. Create a job
	id, err := db.CreateJob(ctx, "test-user", "Test Job", "Do stuff")
	if err != nil {
		t.Fatal(err)
	}

	// 2. Block it
	err = db.UpdateJobStatus(ctx, id, "blocked", "Need API Key")
	if err != nil {
		t.Fatal(err)
	}

	// 3. Verify Store logic for Active Job
	job, err := db.GetActiveJob(ctx, "test-user")
	if err != nil {
		t.Fatal(err)
	}
	if job == nil {
		t.Fatal("Expected active job, got nil")
	}
	if job.Status != "blocked" {
		t.Errorf("Expected blocked status, got %s", job.Status)
	}
	if job.BlockedReason != "Need API Key" {
		t.Errorf("Expected reason 'Need API Key', got %s", job.BlockedReason)
	}
}

func TestUserTrustLevels(t *testing.T) {
	db := SetupTestDB(t)
	defer db.Close()
	ctx := context.Background()

	cfg := &config.Config{
		AdminUserID: "admin",
		Model: "mock-model",
	}

	// Mock Client
	client := &MockClient{}

	loop := &Loop{
		Config:   cfg,
		DB:       db,
		Client:   client,
		Context:  &ContextManager{DB: db},
		Executor: &MockExecutor{},
	}

	// 1. New User (Admin) -> Should be auto-promoted
	msg1 := gateway.Message{SenderID: "admin", Content: "Hello", Channel: "test", ThreadID: "t1"}
	_, err := loop.RunOneTurn(ctx, msg1)
	if err != nil {
		t.Errorf("RunOneTurn failed: %v", err)
	}
	user, _ := db.GetUser(ctx, "admin")
	if user.TrustLevel != "admin" {
		t.Errorf("Expected admin trust level, got %s", user.TrustLevel)
	}

	// 2. New User (Stranger) -> Should be restricted (assuming schema default is 'restricted')
	msg2 := gateway.Message{SenderID: "stranger", Content: "Hello", Channel: "test", ThreadID: "t2"}
	reply, err := loop.RunOneTurn(ctx, msg2)
	if err != nil {
		t.Errorf("RunOneTurn failed: %v", err)
	}
	if !strings.Contains(reply, "Restricted") {
		t.Errorf("Expected restriction message, got: %s", reply)
	}
	user2, _ := db.GetUser(ctx, "stranger")
	if user2.TrustLevel != "restricted" {
		t.Errorf("Expected restricted trust level, got %s", user2.TrustLevel)
	}

	// 3. Admin approves stranger
	db.UpdateUserTrust(ctx, "stranger", "trusted")
	
	// 4. Stranger (now Trusted) -> Should proceed
	msg3 := gateway.Message{SenderID: "stranger", Content: "Hello again", Channel: "test", ThreadID: "t2"}
	reply3, err := loop.RunOneTurn(ctx, msg3)
	if err != nil {
		t.Errorf("RunOneTurn failed: %v", err)
	}
	if strings.Contains(reply3, "Restricted") {
		t.Errorf("User still restricted after approval")
	}
}

// MockClient needs full interface imp
type MockClient struct {}
func (m *MockClient) ChatCompletion(ctx context.Context, msgs []openrouter.Message) (string, error) {
	return "mock_response", nil
}
func (m *MockClient) ChatCompletionWithTools(ctx context.Context, msgs []openrouter.Message, tools []openrouter.ToolDefinition) (string, []openrouter.ToolCall, error) {
	return "mock_response", nil, nil
}
func (m *MockClient) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, nil
}

// MockSubmindLLMSimple returns no tool calls so sub-mind completes in one turn.
type MockSubmindLLMSimple struct{}
func (m *MockSubmindLLMSimple) ChatCompletion(ctx context.Context, msgs []openrouter.Message) (string, error) {
	return "done", nil
}
func (m *MockSubmindLLMSimple) ChatCompletionWithTools(ctx context.Context, msgs []openrouter.Message, tools []openrouter.ToolDefinition) (string, []openrouter.ToolCall, error) {
	return "done", nil, nil
}
func (m *MockSubmindLLMSimple) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, nil
}

func TestLoop_SpawnSubmind_createsAndPersistsSession(t *testing.T) {
	ctx := context.Background()
	db := SetupTestDB(t)
	defer db.Close()
	_, _ = db.GetOrCreateUser(ctx, "u1", "", "test")

	reg, err := LoadSubmindRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	loop := &Loop{
		Config:          &config.Config{},
		DB:              db,
		Client:          &MockSubmindLLMSimple{},
		Context:         &ContextManager{DB: db},
		Executor:        &MockExecutor{},
		SubmindRegistry: reg,
	}
	result, err := loop.SpawnSubmind(ctx, "u1", "reflection", "task", 0)
	if err != nil {
		t.Fatalf("SpawnSubmind failed: %v", err)
	}
	if result.SessionID == 0 {
		t.Error("expected SessionID in result")
	}
	if !result.Success || result.Turns < 1 {
		t.Errorf("expected success and at least 1 turn, got success=%v turns=%d", result.Success, result.Turns)
	}
	ses, err := db.GetSubmindSession(ctx, result.SessionID, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if ses.Status != "completed" {
		t.Errorf("session status: got %s", ses.Status)
	}
}

func TestLoop_SpawnSubmind_resume(t *testing.T) {
	ctx := context.Background()
	db := SetupTestDB(t)
	defer db.Close()
	_, _ = db.GetOrCreateUser(ctx, "u1", "", "test")

	reg, _ := LoadSubmindRegistry(t.TempDir())
	loop := &Loop{
		Config:          &config.Config{},
		DB:              db,
		Client:          &MockSubmindLLMSimple{},
		Context:         &ContextManager{DB: db},
		Executor:        &MockExecutor{},
		SubmindRegistry: reg,
	}
	id, _ := db.CreateSubmindSession(ctx, "u1", "reflection", "task", "You are helpful.")
	_ = db.UpdateSubmindSession(ctx, id, []core.Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "task"},
	}, 0, "running", "", "")

	result, err := loop.SpawnSubmind(ctx, "u1", "reflection", "task", id)
	if err != nil {
		t.Fatalf("SpawnSubmind resume failed: %v", err)
	}
	if !result.Success {
		t.Errorf("resume failed: %v", result.Error)
	}
	ses, _ := db.GetSubmindSession(ctx, id, "u1")
	if ses.Status != "completed" {
		t.Errorf("session after resume: status=%s", ses.Status)
	}
}
