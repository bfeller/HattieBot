package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// Config holds runtime configuration. Secrets (e.g. API key) are read from
// the environment or from the config dir at runtime; never committed.
type Config struct {
	// OpenRouterAPIKey is set from env OPENROUTER_API_KEY or from config file.
	OpenRouterAPIKey string
	// Model is the OpenRouter model id (e.g. moonshotai/kimi-k2.5).
	Model string
	
	// Zulip Configuration
	ZulipURL   string
	ZulipEmail string
	ZulipKey   string
	// ConfigDir is where config file and system_purpose.txt live (e.g. ~/.config/hattiebot or .hattiebot).
	ConfigDir string
	// DBPath is the path to hattiebot.db.
	DBPath string
	// WorkspaceDir is the working directory for terminal commands and file tools.
	WorkspaceDir string
	// SystemPurposePath is the path to system_purpose.txt.
	SystemPurposePath string
	// ToolsDir is where agent-created Go tool sources live (e.g. tools/ or agent_tools/).
	ToolsDir string
	// BinDir is where compiled tool binaries are placed (e.g. bin/).
	BinDir string
	// DocsDir is where architecture docs live (e.g. docs/).
	DocsDir string
	// RequireApprovalForNewTools when true prompts before first run of a newly registered tool.
	RequireApprovalForNewTools bool
	// TokenBudget optional daily token cap; 0 = no limit. Core can count tokens per request and enforce or warn.
	TokenBudget int64
	// AgentName is the name of the bot (loaded from config file during onboarding).
	AgentName string
	// AdminUserID is the ID of the trusted admin user (e.g. Zulip email or "admin").
	AdminUserID string
	// ToolOutputMaxRunes caps tool output length (0 = no truncation). Set via HATTIEBOT_TOOL_OUTPUT_MAX_RUNES.
	ToolOutputMaxRunes int

	// Embedding service (vector memory). When set, memorize/recall use this instead of LLM Embed.
	EmbeddingServiceURL   string // e.g. http://embeddinggood:8000 or https://embedding.bfs5.com
	EmbeddingServiceAPIKey string
	EmbeddingDimension   int    // 128, 256, 512, or 768; default 768
}

// DefaultConfigDir returns the default config directory (project-local .hattiebot if present, else ~/.config/hattiebot).
func DefaultConfigDir() string {
	cwd, _ := os.Getwd()
	local := filepath.Join(cwd, ".hattiebot")
	if info, err := os.Stat(local); err == nil && info.IsDir() {
		return local
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "hattiebot")
}

// New builds config from env and optional config dir. ConfigDir can be empty to use default.
// In Docker, set HATTIEBOT_CONFIG_DIR=/data (or your mount) so DB and system_purpose.txt persist.
func New(configDir string) *Config {
	if configDir == "" {
		if d := os.Getenv("HATTIEBOT_CONFIG_DIR"); d != "" {
			configDir = d
		} else {
			configDir = DefaultConfigDir()
		}
	}
	dbPath := filepath.Join(configDir, "hattiebot.db")
	systemPurposePath := filepath.Join(configDir, "system_purpose.txt")
	cwd, _ := os.Getwd()
	toolOutputMaxRunes := 0
	if v := os.Getenv("HATTIEBOT_TOOL_OUTPUT_MAX_RUNES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			toolOutputMaxRunes = n
		}
	}
	embedDim := 768
	if v := os.Getenv("HATTIEBOT_EMBEDDING_DIMENSION"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && (n == 128 || n == 256 || n == 512 || n == 768) {
			embedDim = n
		}
	}
	return &Config{
		OpenRouterAPIKey:        os.Getenv("OPENROUTER_API_KEY"),
		Model:                  os.Getenv("HATTIEBOT_MODEL"), // can be overridden by config file
		ConfigDir:              configDir,
		DBPath:                 dbPath,
		WorkspaceDir:           cwd,
		SystemPurposePath:      systemPurposePath,
		ToolsDir:               filepath.Join(configDir, "tools"),
		BinDir:                 filepath.Join(configDir, "bin"),
		DocsDir:                filepath.Join(cwd, "docs"),
		ToolOutputMaxRunes:     toolOutputMaxRunes,
		EmbeddingServiceURL:    os.Getenv("EMBEDDING_SERVICE_URL"),
		EmbeddingServiceAPIKey: os.Getenv("EMBEDDING_SERVICE_API_KEY"),
		EmbeddingDimension:    embedDim,
	}
}
