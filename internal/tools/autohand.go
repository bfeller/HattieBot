package tools

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"
)

// AutohandCLI invokes the Autohand CLI with the given instruction (e.g. autohand -p "instruction").
func AutohandCLI(ctx context.Context, instruction string) (stdout, stderr string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "autohand", "-p", instruction)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", string(out), err
	}
	return string(out), "", nil
}

// AutohandCLITool args: {"instruction": "..."}. Returns {"stdout": "...", "stderr": "...", "error": "..."}.
func AutohandCLITool(ctx context.Context, argsJSON string) (string, error) {
	var args struct {
		Instruction string `json:"instruction"`
	}
	if argsJSON != "" {
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return "", err
		}
	}
	stdout, stderr, err := AutohandCLI(ctx, args.Instruction)
	m := map[string]string{"stdout": stdout, "stderr": stderr}
	if err != nil {
		m["error"] = err.Error()
	}
	out, _ := json.Marshal(m)
	return string(out), nil
}
