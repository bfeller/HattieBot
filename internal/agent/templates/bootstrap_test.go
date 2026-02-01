package templates

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureTemplates(t *testing.T) {
	dir := t.TempDir()
	if err := EnsureTemplates(dir); err != nil {
		t.Fatal(err)
	}
	toolMain := filepath.Join(dir, "templates", "tool_main.go")
	submindExample := filepath.Join(dir, "templates", "submind_example.json")
	for _, path := range []string{toolMain, submindExample} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected template file %s to exist: %v", path, err)
		}
	}
	// Ensure we don't overwrite: run again and content should be unchanged
	data1, _ := os.ReadFile(toolMain)
	if err := EnsureTemplates(dir); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(toolMain)
	if string(data1) != string(data2) {
		t.Error("EnsureTemplates should not overwrite existing files")
	}
}

func TestEnsureTemplates_empty_configDir_no_error(t *testing.T) {
	// Empty string should be no-op
	if err := EnsureTemplates(""); err != nil {
		t.Fatal(err)
	}
}
