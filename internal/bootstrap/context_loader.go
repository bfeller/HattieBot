package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hattiebot/hattiebot/internal/store"
)

// LoadContextDocs scans the given directory for .md files and upserts them into the database.
func LoadContextDocs(ctx context.Context, db *store.DB, dir string) error {
	// Ensure directory exists
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		// Just create it? Or warn?
		// Let's create it to make it easy for user.
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create context dir: %w", err)
		}
		return nil // Empty dir, nothing to load.
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("context path %s is not a directory", dir)
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if !strings.HasSuffix(f.Name(), ".md") {
			continue
		}

		// Title is filename without extension. 
		// Convention: snake_case filename -> Title Case title? 
		// Or strictly filename. Let's use clean filename for now.
		title := strings.TrimSuffix(f.Name(), ".md")
		title = strings.ReplaceAll(title, "_", " ")
		title = titlecaser(title) // Simple helper or just leave as is.
		// Actually, let's keep it simple: Filename is the key (Title).
		title = strings.TrimSuffix(f.Name(), ".md")

		// Read content
		path := filepath.Join(dir, f.Name())
		contentBytes, err := os.ReadFile(path)
		if err != nil {
			fmt.Printf("warning: failed to read context doc %s: %v\n", path, err)
			continue
		}
		content := string(contentBytes)
		description := "Synced from " + f.Name()

		// Upsert logic
		existing, err := db.GetContextDoc(ctx, title)
		if err != nil {
			fmt.Printf("warning: failed to check context doc %s: %v\n", title, err)
			continue
		}

		if existing != nil {
			// Update if content changed? Always update for now to simple.
			if existing.Content != content {
				if err := db.UpdateContextDoc(ctx, title, content, description); err != nil {
					fmt.Printf("warning: failed to update context doc %s: %v\n", title, err)
				} else {
					fmt.Printf("[Context] Updated '%s' from file.\n", title)
				}
			}
		} else {
			if _, err := db.CreateContextDoc(ctx, title, content, description); err != nil {
				fmt.Printf("warning: failed to create context doc %s: %v\n", title, err)
			} else {
				fmt.Printf("[Context] Created '%s' from file.\n", title)
			}
			// Note: Created is inactive by default.
		}
	}
	return nil
}

// Simple title caser (e.g. "tool_guide" -> "Tool Guide" if we wanted).
// But for now we stick to strict filename mapping (tool_guide -> tool_guide) to avoid ambiguity.
func titlecaser(s string) string {
	// strings.Title is deprecated.
	return s 
}
