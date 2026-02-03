package tools

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// ensureGitRepo initializes git in configDir if needed and sets user.name/email so autohand's git ops don't prompt.
func ensureGitRepo(configDir string) {
	gitDir := filepath.Join(configDir, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return // already a repo
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = configDir
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[AUTOHAND] git init failed: %v\n%s", err, out)
		return
	}
	for _, arg := range []struct{ k, v string }{
		{"user.name", "HattieBot"},
		{"user.email", "hattiebot@local"},
	} {
		c := exec.Command("git", "config", arg.k, arg.v)
		c.Dir = configDir
		if out, err := c.CombinedOutput(); err != nil {
			log.Printf("[AUTOHAND] git config %s failed: %v\n%s", arg.k, err, out)
		}
	}
}

// ensureAutohandConfig writes ~/.autohand/config.json so autohand does not prompt for login on first run.
func ensureAutohandConfig() (configPath string, err error) {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/root" // Docker default when running as root
	}
	autohandDir := filepath.Join(home, ".autohand")
	if err := os.MkdirAll(autohandDir, 0755); err != nil {
		return "", err
	}
	configPath = filepath.Join(autohandDir, "config.json")
	apiKey := os.Getenv("OPENROUTER_API_KEY")
	model := os.Getenv("HATTIEBOT_MODEL")
	if model == "" {
		model = "anthropic/claude-sonnet-4"
	}
	cfg := map[string]interface{}{
		"provider": "openrouter",
		"openrouter": map[string]interface{}{
			"apiKey":     apiKey,
			"model":      model,
			"maxTokens":  8192,
			"temperature": 0.3,
		},
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(configPath, b, 0600); err != nil {
		return "", err
	}
	return configPath, nil
}

// AutohandCLI invokes the Autohand CLI with the given instruction (e.g. autohand -p "instruction").
// Uses --yes and --unrestricted for non-interactive use (no TTY); --path for workspace.
// Ensures autohand config exists to avoid first-run login prompt.
func AutohandCLI(ctx context.Context, instruction string) (stdout, stderr string, err error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	// --yes: auto-confirm risky actions
	// --unrestricted: skip all approval prompts (required when no TTY; otherwise autohand blocks)
	// --path: workspace to operate in (config dir where tools live)
	configDir := os.Getenv("CONFIG_DIR")
	if configDir == "" {
		configDir = os.Getenv("HATTIEBOT_CONFIG_DIR")
	}
	if configDir == "" {
		configDir = "/data" // Docker default
	}
	// Ensure workspace is a git repo with user config so autohand's git ops don't prompt
	ensureGitRepo(configDir)
	// Ensure config exists so autohand does not prompt for login
	autohandConfig, cfgErr := ensureAutohandConfig()
	args := []string{"--yes", "--unrestricted", "--path", configDir, "-p", instruction}
	if cfgErr == nil && autohandConfig != "" {
		args = append([]string{"--config", autohandConfig}, args...)
	}
	cmd := exec.CommandContext(ctx, "autohand", args...)
	cmd.Env = os.Environ() // Inherit OPENROUTER_API_KEY and other env
	// Pipe newlines so if autohand prompts "Press Enter to continue" it gets input and proceeds
	cmd.Stdin = strings.NewReader("\n\n\n\n\n")
	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		log.Printf("[AUTOHAND] failed: %v\noutput:\n%s", err, output)
		return "", output, err
	}
	return output, "", nil
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
	// Prepend strict instruction to overwrite files
	fullInstruction := "IMPORTANT: If the target file exists, you MUST overwrite it with the new code. Do not skip writing. " + args.Instruction
	stdout, stderr, err := AutohandCLI(ctx, fullInstruction)
	m := map[string]string{"stdout": stdout, "stderr": stderr}
	if err != nil {
		m["error"] = err.Error()
	}
	out, _ := json.Marshal(m)
	return string(out), nil
}
