package store

import (
	"context"
	"testing"
)

func TestInsertTool_and_ToolByName_and_AllTools(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id, err := db.InsertTool(ctx, "my_tool", "/bin/my_tool", "Does something", `{"type":"object"}`)
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Errorf("expected positive id, got %d", id)
	}

	tool, err := db.ToolByName(ctx, "my_tool")
	if err != nil {
		t.Fatal(err)
	}
	if tool == nil {
		t.Fatal("expected tool, got nil")
	}
	if tool.Name != "my_tool" || tool.BinaryPath != "/bin/my_tool" || tool.Description != "Does something" {
		t.Errorf("tool: %+v", tool)
	}

	all, err := db.AllTools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || all[0].Name != "my_tool" {
		t.Errorf("AllTools: %+v", all)
	}

	none, err := db.ToolByName(ctx, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if none != nil {
		t.Errorf("expected nil for nonexistent, got %+v", none)
	}
}

func TestRecordToolSuccess_and_RecordToolFailure_and_ListBrokenTools(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id, err := db.InsertTool(ctx, "health_tool", "/bin/health_tool", "desc", "{}")
	if err != nil {
		t.Fatal(err)
	}
	if id <= 0 {
		t.Fatal("expected positive id")
	}

	// RecordToolSuccess: updates last_success, resets failure_count
	if err := db.RecordToolSuccess(ctx, "health_tool"); err != nil {
		t.Fatal(err)
	}
	tool, _ := db.ToolByName(ctx, "health_tool")
	if tool == nil || tool.FailureCount != 0 || tool.Status != "active" || tool.LastSuccess == nil {
		t.Errorf("after success: %+v", tool)
	}

	// RecordToolFailure: increments failure_count, sets last_error; after 3 sets status broken
	if err := db.RecordToolFailure(ctx, "health_tool", "err1"); err != nil {
		t.Fatal(err)
	}
	tool, _ = db.ToolByName(ctx, "health_tool")
	if tool == nil || tool.FailureCount != 1 || tool.LastError != "err1" {
		t.Errorf("after 1 failure: %+v", tool)
	}
	db.RecordToolFailure(ctx, "health_tool", "err2")
	db.RecordToolFailure(ctx, "health_tool", "err3")
	tool, _ = db.ToolByName(ctx, "health_tool")
	if tool == nil || tool.FailureCount != 3 || tool.Status != "broken" {
		t.Errorf("after 3 failures: %+v", tool)
	}

	// ListBrokenTools returns only broken
	broken, err := db.ListBrokenTools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(broken) != 1 || broken[0].Name != "health_tool" {
		t.Errorf("ListBrokenTools: %+v", broken)
	}

	// RecordToolSuccess resets failure_count and status
	if err := db.RecordToolSuccess(ctx, "health_tool"); err != nil {
		t.Fatal(err)
	}
	tool, _ = db.ToolByName(ctx, "health_tool")
	if tool == nil || tool.FailureCount != 0 || tool.Status != "active" {
		t.Errorf("after success again: %+v", tool)
	}
	broken, _ = db.ListBrokenTools(ctx)
	if len(broken) != 0 {
		t.Errorf("expected no broken tools, got %+v", broken)
	}
}

func TestListBrokenTools_empty(t *testing.T) {
	ctx := context.Background()
	db, err := Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	broken, err := db.ListBrokenTools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(broken) != 0 {
		t.Errorf("expected empty, got %+v", broken)
	}
}
