package store

import (
	"context"
	"testing"
	"path/filepath"

	_ "modernc.org/sqlite" // Ensure sqlite driver is loaded
)

func TestContextDocs(t *testing.T) {
	// Setup temporary DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Failed to create DB: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Test Create
	id, err := db.CreateContextDoc(ctx, "Test Doc", "This is content", "description")
	if err != nil {
		t.Fatalf("CreateContextDoc failed: %v", err)
	}
	if id == 0 {
		t.Fatal("ID should not be 0")
	}

	// Test Get
	doc, err := db.GetContextDoc(ctx, "Test Doc")
	if err != nil {
		t.Fatalf("GetContextDoc failed: %v", err)
	}
	if doc == nil {
		t.Fatal("Doc not found")
	}
	if doc.Content != "This is content" {
		t.Errorf("Expected content 'This is content', got '%s'", doc.Content)
	}
	if doc.IsActive {
		t.Error("New doc should be inactive")
	}

	// Test Update
	err = db.UpdateContextDoc(ctx, "Test Doc", "New Content", "New Desc")
	if err != nil {
		t.Fatalf("UpdateContextDoc failed: %v", err)
	}
	doc, _ = db.GetContextDoc(ctx, "Test Doc")
	if doc.Content != "New Content" {
		t.Errorf("Content not updated")
	}

	// Test Toggle Active
	err = db.SetContextDocActive(ctx, "Test Doc", true)
	if err != nil {
		t.Fatalf("SetContextDocActive failed: %v", err)
	}
	
	activeDocs, err := db.ListActiveContextDocs(ctx)
	if err != nil {
		t.Fatalf("ListActiveContextDocs failed: %v", err)
	}
	if len(activeDocs) != 1 {
		t.Errorf("Expected 1 active doc, got %d", len(activeDocs))
	}
	if activeDocs[0].Title != "Test Doc" {
		t.Errorf("Expected active doc title 'Test Doc', got '%s'", activeDocs[0].Title)
	}

	// Test List All
	_, err = db.CreateContextDoc(ctx, "Doc 2", "Content 2", "Desc 2")
	if err != nil {
		t.Fatalf("Create second doc failed: %v", err)
	}
	allDocs, err := db.ListContextDocs(ctx)
	if err != nil {
		t.Fatalf("ListContextDocs failed: %v", err)
	}
	if len(allDocs) != 2 {
		t.Errorf("Expected 2 docs, got %d", len(allDocs))
	}

	// Test Delete
	err = db.DeleteContextDoc(ctx, "Test Doc")
	if err != nil {
		t.Fatalf("DeleteContextDoc failed: %v", err)
	}
	doc, err = db.GetContextDoc(ctx, "Test Doc")
	if err != nil {
		t.Fatalf("GetContextDoc failed after delete: %v", err)
	}
	if doc != nil {
		t.Error("Doc should be deleted")
	}
}
