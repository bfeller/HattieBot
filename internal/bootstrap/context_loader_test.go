package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"github.com/hattiebot/hattiebot/internal/store"
	_ "modernc.org/sqlite"
)

func TestLoadContextDocs(t *testing.T) {
	// Setup DB
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	db, err := store.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	// Setup Docs Dir
	docsDir := filepath.Join(tmpDir, "docs/context")
	if err := os.MkdirAll(docsDir, 0755); err != nil {
		t.Fatalf("Failed to create docs dir: %v", err)
	}

	// Create a test file
	testFile := filepath.Join(docsDir, "test_doc.md")
	content := "# Test Doc\nContent here."
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Run Loader
	if err := LoadContextDocs(context.Background(), db, docsDir); err != nil {
		t.Fatalf("LoadContextDocs failed: %v", err)
	}

	// Verify DB
	doc, err := db.GetContextDoc(context.Background(), "test_doc")
	if err != nil {
		t.Fatalf("GetContextDoc failed: %v", err)
	}
	if doc == nil {
		t.Fatal("Doc not found in DB")
	}
	if doc.Content != content {
		t.Errorf("Content mismatch. Want %q, got %q", content, doc.Content)
	}
	if doc.Title != "test_doc" {
		t.Errorf("Title mismatch. Want 'test_doc', got %q", doc.Title)
	}

	// Test Update
	newContent := "# Updated\nNew content."
	if err := os.WriteFile(testFile, []byte(newContent), 0644); err != nil {
		t.Fatalf("Failed to update test file: %v", err)
	}

	// Run Loader Again
	if err := LoadContextDocs(context.Background(), db, docsDir); err != nil {
		t.Fatalf("LoadContextDocs (update) failed: %v", err)
	}

	doc, _ = db.GetContextDoc(context.Background(), "test_doc")
	if doc.Content != newContent {
		t.Errorf("Content update failed. Want %q, got %q", newContent, doc.Content)
	}
}
