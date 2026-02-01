package templates

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed default/*
var defaultTemplates embed.FS

// EnsureTemplates copies embedded template files into configDir/templates/ if they do not already exist.
// Does not overwrite existing files so user edits are preserved.
func EnsureTemplates(configDir string) error {
	if configDir == "" {
		return nil
	}
	dir := filepath.Join(configDir, "templates")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create templates dir: %w", err)
	}
	entries, err := fs.ReadDir(defaultTemplates, "default")
	if err != nil {
		return fmt.Errorf("read embedded templates: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		dst := filepath.Join(dir, e.Name())
		if _, err := os.Stat(dst); err == nil {
			continue // file exists, skip
		}
		data, err := fs.ReadFile(defaultTemplates, filepath.Join("default", e.Name()))
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(dst, data, 0644); err != nil {
			return fmt.Errorf("write %s: %w", dst, err)
		}
	}
	return nil
}
