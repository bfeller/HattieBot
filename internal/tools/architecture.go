package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
)

// ReadArchitecture reads a fixed set of markdown files (docs/architecture.md, docs/tools.md, docs/creating-tools.md, docs/embedding-service.md).
func ReadArchitecture(ctx context.Context, docsDir string) (string, error) {
	files := []string{"architecture.md", "tools.md", "creating-tools.md", "embedding-service.md"}
	var out string
	for _, f := range files {
		p := filepath.Join(docsDir, f)
		data, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", err
		}
		out += "--- " + f + " ---\n" + string(data) + "\n\n"
	}
	if out == "" {
		return "No architecture docs found in " + docsDir, nil
	}
	return out, nil
}

// ReadArchitectureTool args: {}. Returns {"content": "..."}.
func ReadArchitectureTool(ctx context.Context, docsDir, argsJSON string) (string, error) {
	content, err := ReadArchitecture(ctx, docsDir)
	if err != nil {
		out, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(out), nil
	}
	out, _ := json.Marshal(map[string]string{"content": content})
	return string(out), nil
}
