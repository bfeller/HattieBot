package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// RunTerminal runs a shell command in the given working directory and returns stdout, stderr, and exit code.
func RunTerminal(ctx context.Context, workDir, command string, envVars map[string]string) (stdout, stderr string, exitCode int, err error) {
	var shell string
	var args []string
	if runtime.GOOS == "windows" {
		shell = "cmd"
		args = []string{"/C", command}
	} else {
		shell = "sh"
		args = []string{"-c", command}
	}
	cmd := exec.CommandContext(ctx, shell, args...)
	cmd.Dir = workDir
	
	// Environment variables
	// Inherit current env
	cmd.Env = os.Environ()
	// Add/Overwrite with provided envVars
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

// RunTerminalWithTimeout runs the command with a timeout (default 5m).
func RunTerminalWithTimeout(ctx context.Context, workDir, command string, envVars map[string]string, timeout time.Duration) (stdout, stderr string, exitCode int, err error) {
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return RunTerminal(ctx, workDir, command, envVars)
}

// RunTerminalTool is the tool entrypoint: args is JSON {"work_dir": "...", "command": "..."}.
func RunTerminalTool(ctx context.Context, workDirDefault string, argsJSON string) (string, error) {
	var args struct {
		WorkDir string            `json:"work_dir"`
		Command string            `json:"command"`
		EnvVars map[string]string `json:"env_vars"`
	}
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
	}
	if args.WorkDir == "" {
		args.WorkDir = workDirDefault
	}
	args.WorkDir = filepath.Clean(args.WorkDir)
	if args.Command == "" {
		out, _ := json.Marshal(map[string]interface{}{"error": "command is required", "stdout": "", "stderr": "", "exit_code": -1})
		return string(out), nil
	}
	stdout, stderr, code, _ := RunTerminalWithTimeout(ctx, args.WorkDir, args.Command, args.EnvVars, 5*time.Minute)
	out := map[string]interface{}{
		"stdout":   stdout,
		"stderr":   stderr,
		"exit_code": code,
	}
	raw, _ := json.Marshal(out)
	return string(raw), nil
}
