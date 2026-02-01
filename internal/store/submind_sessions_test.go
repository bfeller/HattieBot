package store

import (
	"context"
	"testing"

	"github.com/hattiebot/hattiebot/internal/core"
)

func TestSubmindSessions_CreateGetUpdateList(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, _ = db.GetOrCreateUser(ctx, "u1", "", "test")
	_, _ = db.GetOrCreateUser(ctx, "u2", "", "test")

	id, err := db.CreateSubmindSession(ctx, "u1", "reflection", "do something", "You are helpful.")
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}

	s, err := db.GetSubmindSession(ctx, id, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if s.UserID != "u1" || s.Mode != "reflection" || s.Task != "do something" || s.Status != "running" || s.Turns != 0 {
		t.Errorf("session: %+v", s)
	}
	msgs := s.Messages()
	if len(msgs) != 2 || msgs[0].Role != "system" || msgs[1].Role != "user" || msgs[1].Content != "do something" {
		t.Errorf("messages: %+v", msgs)
	}

	updated := []core.Message{
		{Role: "system", Content: "You are helpful."},
		{Role: "user", Content: "do something"},
		{Role: "assistant", Content: "Thinking..."},
		{Role: "tool", Content: "ok", ToolCallID: "call_1"},
	}
	err = db.UpdateSubmindSession(ctx, id, updated, 1, "running", "", "")
	if err != nil {
		t.Fatal(err)
	}

	s2, err := db.GetSubmindSession(ctx, id, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if s2.Turns != 1 || s2.Status != "running" {
		t.Errorf("after update: turns=%d status=%s", s2.Turns, s2.Status)
	}
	if len(s2.Messages()) != 4 {
		t.Errorf("expected 4 messages, got %d", len(s2.Messages()))
	}

	err = db.UpdateSubmindSession(ctx, id, updated, 1, "completed", "Done.", "")
	if err != nil {
		t.Fatal(err)
	}
	s3, _ := db.GetSubmindSession(ctx, id, "u1")
	if s3.Status != "completed" || s3.ResultOutput != "Done." {
		t.Errorf("final: status=%s result_output=%s", s3.Status, s3.ResultOutput)
	}
}

func TestSubmindSessions_ListByUserAndStatus(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, _ = db.GetOrCreateUser(ctx, "u1", "", "test")

	id1, _ := db.CreateSubmindSession(ctx, "u1", "reflection", "task1", "sys")
	id2, _ := db.CreateSubmindSession(ctx, "u1", "reflection", "task2", "sys")
	_ = db.UpdateSubmindSession(ctx, id2, []core.Message{}, 0, "completed", "out", "")

	all, err := db.ListSubmindSessions(ctx, "u1", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(all))
	}

	running, err := db.ListSubmindSessions(ctx, "u1", "running")
	if err != nil {
		t.Fatal(err)
	}
	if len(running) != 1 || running[0].ID != id1 {
		t.Errorf("expected 1 running session id=%d, got %d sessions %+v", id1, len(running), running)
	}

	completed, err := db.ListSubmindSessions(ctx, "u1", "completed")
	if err != nil {
		t.Fatal(err)
	}
	if len(completed) != 1 || completed[0].ID != id2 {
		t.Errorf("expected 1 completed session id=%d, got %+v", id2, completed)
	}
}

func TestSubmindSessions_GetWrongUser(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, _ = db.GetOrCreateUser(ctx, "u1", "", "test")
	_, _ = db.GetOrCreateUser(ctx, "u2", "", "test")

	id, _ := db.CreateSubmindSession(ctx, "u1", "reflection", "task", "sys")

	_, err = db.GetSubmindSession(ctx, id, "u2")
	if err == nil {
		t.Error("expected error when getting session with wrong user")
	}
}
