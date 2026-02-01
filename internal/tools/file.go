package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ReadFile reads the contents of a file (within workspace).
func ReadFile(ctx context.Context, workspaceDir, path string) (string, error) {
	p := filepath.Join(workspaceDir, filepath.Clean(path))
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	base, err := filepath.Abs(workspaceDir)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(base, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", os.ErrPermission
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ListDir lists entries in a directory.
func ListDir(ctx context.Context, workspaceDir, path string) ([]os.DirEntry, error) {
	p := filepath.Join(workspaceDir, filepath.Clean(path))
	abs, err := filepath.Abs(p)
	if err != nil {
		return nil, err
	}
	base, err := filepath.Abs(workspaceDir)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(base, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil, os.ErrPermission
	}
	return os.ReadDir(p)
}

// Stat returns file info (name, size, is_dir, mode).
func Stat(ctx context.Context, workspaceDir, path string) (os.FileInfo, error) {
	p := filepath.Join(workspaceDir, filepath.Clean(path))
	abs, err := filepath.Abs(p)
	if err != nil {
		return nil, err
	}
	base, err := filepath.Abs(workspaceDir)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(base, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil, os.ErrPermission
	}
	return os.Stat(p)
}

// ReadFileTool args: {"path": "..."}. Returns file contents as JSON {"content": "..."}.
func ReadFileTool(ctx context.Context, workspaceDir, argsJSON string) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
	}
	content, err := ReadFile(ctx, workspaceDir, args.Path)
	if err != nil {
		out, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(out), nil
	}
	out, _ := json.Marshal(map[string]string{"content": content})
	return string(out), nil
}

// ListDirTool args: {"path": "..."}. Returns {"entries": [{"name": "...", "is_dir": true/false}, ...]}.
func ListDirTool(ctx context.Context, workspaceDir, argsJSON string) (string, error) {
	var args struct {
		Path string `json:"path"`
	}
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
	}
	if args.Path == "" {
		args.Path = "."
	}
	entries, err := ListDir(ctx, workspaceDir, args.Path)
	if err != nil {
		out, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(out), nil
	}
	type e struct {
		Name  string `json:"name"`
		IsDir bool   `json:"is_dir"`
	}
	var list []e
	for _, ent := range entries {
		list = append(list, e{Name: ent.Name(), IsDir: ent.IsDir()})
	}
	out, _ := json.Marshal(map[string]interface{}{"entries": list})
	return string(out), nil
}

// WriteFile writes content to a file (within workspace). Overwrites if exists. Creates directories.
func WriteFile(ctx context.Context, workspaceDir, path, content string) error {
	p := filepath.Join(workspaceDir, filepath.Clean(path))
	abs, err := filepath.Abs(p)
	if err != nil {
		return err
	}
	base, err := filepath.Abs(workspaceDir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(base, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return os.ErrPermission
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content), 0644)
}

// WriteFileTool args: {"path": "...", "content": "..."}. Returns {"status": "success"}.
func WriteFileTool(ctx context.Context, workspaceDir, argsJSON string) (string, error) {
	var args struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", err
	}
	if err := WriteFile(ctx, workspaceDir, args.Path, args.Content); err != nil {
		out, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(out), nil
	}
	return `{"status": "success"}`, nil
}
