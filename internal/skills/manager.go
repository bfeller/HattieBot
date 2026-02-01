package skills

import (
	"context"
	"fmt"
	"os/exec"
)

// Skill represents a tool/skill the agent can install and use.
type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	InstallCmd  []string `json:"install_cmd"` // e.g. ["go", "install", "github.com/foo/bar@latest"]
	CheckCmd    []string `json:"check_cmd"`   // e.g. ["bar", "--version"]
}

type Manager struct {
	ConfigDir string
}

func NewManager(configDir string) *Manager {
	return &Manager{ConfigDir: configDir}
}

// ListInstalled returns a list of skills that pass their CheckCmd.
func (m *Manager) ListInstalled(ctx context.Context) ([]string, error) {
	// For now, we don't have a persistent registry of installed skills other than the binaries themselves.
	// This is a placeholder. Realistically, we'd check against a known registry or scan a bin dir.
	return []string{}, nil
}

// Install runs the installation command for a known skill.
// For the MVP, we allow installing arbitrary go packages or brew packages if the agent specifies them (autonomous).
// But to be safe, let's restrict it to a "Registry" concept later.
// For this phase, we'll implement a generic "InstallPackage" that takes a command.
func (m *Manager) InstallPackage(ctx context.Context, manager string, pkg string) (string, error) {
	var cmd *exec.Cmd

	switch manager {
	case "go":
		// go install <pkg>
		cmd = exec.CommandContext(ctx, "go", "install", pkg)
		// Assume GOPATH/bin is in PATH or we handle it.
	case "brew":
		// brew install <pkg>
		cmd = exec.CommandContext(ctx, "brew", "install", pkg)
	case "npm":
		// npm install -g <pkg>
		cmd = exec.CommandContext(ctx, "npm", "install", "-g", pkg)
	default:
		return "", fmt.Errorf("unsupported package manager: %s", manager)
	}

	out, err := cmd.CombinedOutput()
	output := string(out)
	if err != nil {
		return output, fmt.Errorf("install failed: %w\nOutput:\n%s", err, output)
	}

	return output, nil
}
