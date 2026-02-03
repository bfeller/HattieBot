package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/hattiebot/hattiebot/internal/core"
)

// SubmindRegistry manages sub-mind configurations.
// Implements core.SubmindRegistry interface.
type SubmindRegistry struct {
	configs   map[string]core.SubMindConfig
	configDir string
	mu        sync.RWMutex
}

// SubmindRegistryFile is the JSON structure for subminds.json.
type SubmindRegistryFile struct {
	Subminds []core.SubMindConfig `json:"subminds"`
}

// NewSubmindRegistry creates a new empty registry.
func NewSubmindRegistry(configDir string) *SubmindRegistry {
	return &SubmindRegistry{
		configs:   make(map[string]core.SubMindConfig),
		configDir: configDir,
	}
}

// LoadSubmindRegistry loads sub-mind configurations from the config directory.
func LoadSubmindRegistry(configDir string) (*SubmindRegistry, error) {
	registry := NewSubmindRegistry(configDir)

	// Load from subminds.json
	path := filepath.Join(configDir, "subminds.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// No config file, use defaults
			registry.loadDefaults()
			return registry, nil
		}
		return nil, fmt.Errorf("read subminds.json: %w", err)
	}

	var file SubmindRegistryFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse subminds.json: %w", err)
	}

	for _, cfg := range file.Subminds {
		if cfg.MaxTurns <= 0 {
			cfg.MaxTurns = 10 // Default
		}
		registry.configs[cfg.Name] = cfg
	}

	return registry, nil
}

// loadDefaults adds built-in sub-mind configurations.
func (r *SubmindRegistry) loadDefaults() {
	defaults := []core.SubMindConfig{
		{
			Name:         "reflection",
			SystemPrompt: "You are analyzing your own system state. Be conservative — only flag real problems.\n\nIf healthy: \"No issues detected.\"\nIf problems: Describe ONE issue and suggest ONE action.",
			AllowedTools: []string{"system_status", "read_logs"},
			MaxTurns:     3,
			Protected:    true,
		},
		{
			Name:         "tool_creation",
			SystemPrompt: "You are building a Go CLI tool.\n\n1. Define JSON schema\n2. Write Go code (CGO_ENABLED=0)\n3. Compile with go build\n4. Register with register_tool\n\nAll tools MUST be Go. Use standard library. Return JSON.",
			AllowedTools: []string{"read_file", "write_file", "run_terminal_cmd", "register_tool", "list_dir"},
			MaxTurns:     20,
			Protected:    true,
		},
		{
			Name:         "code_analysis",
			SystemPrompt: "Analyze the provided code. Focus on structure, purpose, and potential issues. Do NOT modify files.",
			AllowedTools: []string{"read_file", "list_dir"},
			MaxTurns:     5,
			Protected:    true,
		},
		{
			Name:         "planning",
			SystemPrompt: "Decompose this task into numbered steps. Be specific about files and tools needed.",
			AllowedTools: []string{},
			MaxTurns:     3,
			Protected:    true,
		},
		{
			Name:         "nextcloud_explorer",
			SystemPrompt: "You are the Nextcloud Explorer. Your job is to navigate the Nextcloud instance to find files, users, or information.\n\n**SECURITY RULE**: NEVER output passwords or API keys in chat. Always use `store_secret` to save them and tell the user to check the Password Manager.\n\nUse `list_nextcloud_files` to browse directory trees.\nUse `read_nextcloud_file` to read content.\nUse `request_nextcloud_ocs` for admin queries.\nUse `get_secret` to retrieve credentials required for tasks.",
			AllowedTools: []string{"request_nextcloud_ocs", "list_nextcloud_files", "read_nextcloud_file", "get_secret", "store_secret", "read_file", "list_dir"},
			MaxTurns:     15,
			Protected:    true,
		},
	}

	for _, cfg := range defaults {
		r.configs[cfg.Name] = cfg
	}
}

// Get returns a sub-mind config by name.
func (r *SubmindRegistry) Get(name string) (core.SubMindConfig, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	cfg, ok := r.configs[name]
	return cfg, ok
}

// Add adds or updates a sub-mind configuration.
func (r *SubmindRegistry) Add(config core.SubMindConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if trying to overwrite protected config
	if existing, ok := r.configs[config.Name]; ok && existing.Protected {
		return fmt.Errorf("cannot overwrite protected sub-mind: %s", config.Name)
	}

	if config.MaxTurns <= 0 {
		config.MaxTurns = 10
	}

	r.configs[config.Name] = config
	return r.save()
}

// Delete removes a sub-mind configuration.
func (r *SubmindRegistry) Delete(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	cfg, ok := r.configs[name]
	if !ok {
		return fmt.Errorf("sub-mind not found: %s", name)
	}
	if cfg.Protected {
		return fmt.Errorf("cannot delete protected sub-mind: %s", name)
	}

	delete(r.configs, name)
	return r.save()
}

// List returns all sub-mind configurations.
func (r *SubmindRegistry) List() []core.SubMindConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out []core.SubMindConfig
	for _, cfg := range r.configs {
		out = append(out, cfg)
	}
	return out
}

// save persists the registry to disk.
func (r *SubmindRegistry) save() error {
	if r.configDir == "" {
		return nil // No persistence
	}

	file := SubmindRegistryFile{
		Subminds: make([]core.SubMindConfig, 0, len(r.configs)),
	}
	for _, cfg := range r.configs {
		file.Subminds = append(file.Subminds, cfg)
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(r.configDir, "subminds.json")
	return os.WriteFile(path, data, 0644)
}

// AsMap returns the registry as a map.
func (r *SubmindRegistry) AsMap() map[string]core.SubMindConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make(map[string]core.SubMindConfig)
	for k, v := range r.configs {
		out[k] = v
	}
	return out
}
