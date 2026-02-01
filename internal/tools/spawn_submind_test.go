package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/store"
)

// mockSpawner implements core.SubmindSpawner for tests.
type mockSpawner struct {
	lastUserID    string
	lastSessionID int64
	result        core.SubMindResult
	err           error
}

func (m *mockSpawner) SpawnSubmind(ctx context.Context, userID, mode, task string, sessionID int64) (core.SubMindResult, error) {
	m.lastUserID = userID
	m.lastSessionID = sessionID
	if m.err != nil {
		return core.SubMindResult{}, m.err
	}
	return m.result, nil
}

func TestSpawnSubmind_includesUserIDAndSessionID(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "u1")
	spawner := &mockSpawner{result: core.SubMindResult{Success: true, SessionID: 42, Turns: 1, Output: "ok"}}
	e := &Executor{Spawner: spawner}

	out, err := e.Execute(ctx, "spawn_submind", `{"mode":"reflection","task":"do it"}`)
	if err != nil {
		t.Fatal(err)
	}
	var result core.SubMindResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if result.SessionID != 42 {
		t.Errorf("expected session_id 42, got %d", result.SessionID)
	}
	if spawner.lastUserID != "u1" {
		t.Errorf("spawner lastUserID: got %q", spawner.lastUserID)
	}
	if spawner.lastSessionID != 0 {
		t.Errorf("expected sessionID 0 for new spawn, got %d", spawner.lastSessionID)
	}
}

func TestSpawnSubmind_resumePassesSessionID(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "u1")
	spawner := &mockSpawner{result: core.SubMindResult{Success: true, SessionID: 99, Turns: 2, Output: "resumed"}}
	e := &Executor{Spawner: spawner}

	out, err := e.Execute(ctx, "spawn_submind", `{"mode":"reflection","task":"continue","session_id":99}`)
	if err != nil {
		t.Fatal(err)
	}
	var result core.SubMindResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("parse result: %v", err)
	}
	if spawner.lastSessionID != 99 {
		t.Errorf("expected sessionID 99 for resume, got %d", spawner.lastSessionID)
	}
	if result.SessionID != 99 {
		t.Errorf("result session_id: got %d", result.SessionID)
	}
}

func TestManageSubmind_listSessions(t *testing.T) {
	ctx := context.WithValue(context.Background(), "user_id", "u1")
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, _ = db.GetOrCreateUser(ctx, "u1", "", "test")
	id, _ := db.CreateSubmindSession(ctx, "u1", "reflection", "task", "sys")

	e := &Executor{DB: db, SubmindRegistry: &mockSubmindRegistry{}}
	out, err := e.Execute(ctx, "manage_submind", `{"action":"list_sessions"}`)
	if err != nil {
		t.Fatal(err)
	}
	var sessions []store.SubmindSession
	if err := json.Unmarshal([]byte(out), &sessions); err != nil {
		t.Fatalf("parse list_sessions result: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != id {
		t.Errorf("expected one session id=%d, got %+v", id, sessions)
	}
}

// mockSubmindRegistry implements core.SubmindRegistry for list_sessions test (not used by list_sessions but required by switch).
type mockSubmindRegistry struct{}

func (m *mockSubmindRegistry) Get(name string) (core.SubMindConfig, bool) { return core.SubMindConfig{}, false }
func (m *mockSubmindRegistry) Add(cfg core.SubMindConfig) error          { return nil }
func (m *mockSubmindRegistry) Delete(name string) error                  { return nil }
func (m *mockSubmindRegistry) List() []core.SubMindConfig                { return nil }
