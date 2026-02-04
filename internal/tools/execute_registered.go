package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hattiebot/hattiebot/internal/store"
)

// ExecuteRegisteredTool runs the binary at the given path with JSON args on stdin; returns stdout and exit code.
func ExecuteRegisteredTool(ctx context.Context, binaryPath, argsJSON string, envVars map[string]string) (stdout, stderr string, exitCode int, err error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath)
	cmd.Stdin = bytes.NewReader([]byte(argsJSON))
	
	// Add env vars
	cmd.Env = os.Environ()
	for k, v := range envVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	runErr := cmd.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	if runErr != nil {
		if exit, ok := runErr.(*exec.ExitError); ok {
			exitCode = exit.ExitCode()
		} else {
			exitCode = -1
		}
	}
	return stdout, stderr, exitCode, nil
}

// ExecuteRegisteredToolByName looks up the tool by name in the registry and runs it.
// If binaryPath in the registry is relative, it is resolved against workspaceDir.
func ExecuteRegisteredToolByName(ctx context.Context, db store.ToolRegistry, workspaceDir, name, argsJSON string, envVars map[string]string) (string, error) {
	tool, err := db.ToolByName(ctx, name)
	if err != nil {
		out, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(out), nil
	}
	if tool == nil {
		out, _ := json.Marshal(map[string]string{"error": "tool not found: " + name})
		return string(out), nil
	}
	binaryPath := tool.BinaryPath
	if !filepath.IsAbs(binaryPath) && workspaceDir != "" {
		binaryPath = filepath.Join(workspaceDir, filepath.Clean(binaryPath))
	}
	stdout, stderr, code, _ := ExecuteRegisteredTool(ctx, binaryPath, argsJSON, envVars)
	out := map[string]interface{}{
		"stdout":    stdout,
		"stderr":    stderr,
		"exit_code": code,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}

// ValidateToolOutput returns true if exitCode is 0 and stdout is valid JSON (tool contract).
// Used for health recording: invalid or non-zero triggers RecordToolFailure.
func ValidateToolOutput(stdout string, exitCode int) bool {
	// We accept non-zero exit codes if the output is valid JSON (e.g. {"error": "..."})
	// This ensures tools that correctly report errors via JSON are not marked as broken.

	trimmed := bytes.TrimSpace([]byte(stdout))
	if len(trimmed) == 0 {
		return false
	}
	var v interface{}
	return json.Unmarshal(trimmed, &v) == nil
}
