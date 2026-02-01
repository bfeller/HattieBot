package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hattiebot/hattiebot/internal/store"
)

func TestReadFileTool(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.txt")
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	// ReadFileTool expects workspaceDir and path relative to it; we used dir as workspace and file at foo.txt
	out, err := ReadFileTool(ctx, dir, `{"path":"foo.txt"}`)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	if m["content"] != "hello" {
		t.Errorf("content: got %q", m["content"])
	}
}

func TestListDirTool(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.txt"), nil, 0644)
	_ = os.Mkdir(filepath.Join(dir, "sub"), 0755)
	out, err := ListDirTool(ctx, dir, `{"path":"."}`)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	entries, _ := m["entries"].([]interface{})
	if len(entries) < 2 {
		t.Errorf("expected at least 2 entries, got %d", len(entries))
	}
}

func TestRunTerminalTool(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	out, err := RunTerminalTool(ctx, dir, `{"command":"echo ok"}`)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	if m["stdout"] != "ok\n" && m["stdout"] != "ok\r\n" {
		t.Errorf("stdout: got %q", m["stdout"])
	}
	if code, _ := m["exit_code"].(float64); code != 0 {
		t.Errorf("exit_code: got %v", m["exit_code"])
	}
}

func TestReadArchitectureTool(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	out, err := ReadArchitectureTool(ctx, dir, `{}`)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	// Empty docs dir should return "No architecture docs found" or similar
	if m["content"] == "" && m["error"] == "" {
		t.Errorf("expected content or error, got %v", m)
	}
}

func TestExecuteRegisteredTool_and_Registry(t *testing.T) {
	ctx := context.Background()
	// Build the echo tool to a temp binary (go test runs from package dir internal/tools)
	workspace := t.TempDir()
	echoBin := filepath.Join(workspace, "echo_bin")
	moduleRoot := filepath.Join("..", "..")
	echoPkg := filepath.Join(moduleRoot, "tools", "echo")
	if _, err := os.Stat(echoPkg); err != nil {
		t.Skipf("echo tool source not found at %s: %v", echoPkg, err)
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-o", echoBin, "./tools/echo")
	cmd.Dir = moduleRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("go build echo failed: %v\n%s", err, out)
	}

	// In-memory DB, apply schema, register echo tool
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.InsertTool(ctx, "echo", echoBin, "Echoes back the message", `{"type":"object","properties":{"message":{"type":"string"}},"required":["message"]}`)
	if err != nil {
		t.Fatal(err)
	}

	// Execute via ExecuteRegisteredToolByName (workspaceDir "" since we used absolute path)
	out, err := ExecuteRegisteredToolByName(ctx, db, "", "echo", `{"message":"hello"}`)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	stdout, _ := m["stdout"].(string)
	var reply struct {
		Reply string `json:"reply"`
	}
	if err := json.Unmarshal([]byte(stdout), &reply); err != nil {
		t.Fatalf("stdout %q: %v", stdout, err)
	}
	if reply.Reply != "hello" {
		t.Errorf("reply: got %q", reply.Reply)
	}

	// ExecuteRegisteredTool with unknown name
	out2, _ := ExecuteRegisteredToolByName(ctx, db, "", "nonexistent", `{}`)
	var m2 map[string]string
	_ = json.Unmarshal([]byte(out2), &m2)
	if m2["error"] == "" {
		t.Errorf("expected error for unknown tool")
	}
}

func TestRegisterTool_rejects_invalid_contract(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	// Build a tiny binary that prints plain text (not JSON)
	badSrc := filepath.Join(dir, "bad_main.go")
	const badSrcContent = `package main
import ("fmt"; "os")
func main() { fmt.Fprint(os.Stdout, "not json") }
`
	if err := os.WriteFile(badSrc, []byte(badSrcContent), 0644); err != nil {
		t.Fatal(err)
	}
	badBin := filepath.Join(dir, "bad_tool")
	if out, err := exec.CommandContext(ctx, "go", "build", "-o", badBin, badSrc).CombinedOutput(); err != nil {
		t.Skipf("go build bad tool: %v\n%s", err, out)
	}
	db, err := store.Open(ctx, ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	executor := &Executor{DB: db, WorkspaceDir: dir}
	out, err := executor.Execute(ctx, "register_tool", `{"name":"bad_tool","binary_path":"`+badBin+`","description":"test"}`)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	if m["error"] == nil {
		t.Errorf("expected register_tool to reject invalid contract, got %s", out)
	}
	errStr, _ := m["error"].(string)
	if errStr != "" && !strings.Contains(errStr, "contract test") {
		t.Errorf("expected error to mention contract test, got %q", errStr)
	}
	// Tool should not be registered
	tool, _ := db.ToolByName(ctx, "bad_tool")
	if tool != nil {
		t.Error("tool should not be registered after contract failure")
	}
}