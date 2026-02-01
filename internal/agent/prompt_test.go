package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/hattiebot/hattiebot/internal/config"
	"github.com/hattiebot/hattiebot/internal/store"
)

func TestBuildSystemPrompt_contains_SelfImprovement(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := &config.Config{ConfigDir: t.TempDir(), WorkspaceDir: t.TempDir(), AgentName: "Test"}
	prompt, err := BuildSystemPrompt(ctx, db, cfg, "user1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "Self-Improvement") {
		t.Error("expected prompt to contain Self-Improvement block")
	}
	if !strings.Contains(prompt, "manage_submind") {
		t.Error("expected prompt to mention manage_submind")
	}
}

func TestBuildSystemPrompt_injects_broken_tools(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.InsertTool(ctx, "broken_one", "/bin/broken", "desc", "{}")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		_ = db.RecordToolFailure(ctx, "broken_one", "invalid json output")
	}
	cfg := &config.Config{ConfigDir: t.TempDir(), WorkspaceDir: t.TempDir(), AgentName: "Test"}
	prompt, err := BuildSystemPrompt(ctx, db, cfg, "user1")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "== BROKEN TOOLS ==") {
		t.Error("expected prompt to contain BROKEN TOOLS block")
	}
	if !strings.Contains(prompt, "broken_one") {
		t.Error("expected prompt to contain broken tool name")
	}
	if !strings.Contains(prompt, "invalid json output") {
		t.Error("expected prompt to contain last_error")
	}
}

func TestBuildSystemPrompt_no_broken_tools_block_when_empty(t *testing.T) {
	ctx := context.Background()
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cfg := &config.Config{ConfigDir: t.TempDir(), WorkspaceDir: t.TempDir(), AgentName: "Test"}
	prompt, err := BuildSystemPrompt(ctx, db, cfg, "user1")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(prompt, "== BROKEN TOOLS ==") {
		t.Error("expected no BROKEN TOOLS block when no broken tools")
	}
}
