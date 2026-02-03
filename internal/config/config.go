package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

// Config holds runtime configuration. Secrets (e.g. API key) are read from
// the environment or from the config dir at runtime; never committed.
type Config struct {
	// OpenRouterAPIKey is set from env OPENROUTER_API_KEY or from config file.
	OpenRouterAPIKey string `json:"open_router_api_key"`
	// Model is the OpenRouter model id (e.g. moonshotai/kimi-k2.5).
	Model string `json:"model"`
	// EnvModel stores the value from HATTIEBOT_MODEL env var for fallback purposes.
	EnvModel string `json:"-"`
	
	// ConfigDir is where config file and system_purpose.txt live (e.g. ~/.config/hattiebot or .hattiebot).
	ConfigDir string `json:"-"` // set at runtime
	// DBPath is the path to hattiebot.db.
	DBPath string `json:"-"`
	// WorkspaceDir is the working directory for terminal commands and file tools.
	WorkspaceDir string `json:"-"`
	// SystemPurposePath is the path to system_purpose.txt.
	SystemPurposePath string `json:"-"`
	// ToolsDir is where agent-created Go tool sources live (e.g. tools/ or agent_tools/).
	ToolsDir string `json:"-"`
	// BinDir is where compiled tool binaries are placed (e.g. bin/).
	BinDir string `json:"-"`
	// DocsDir is where architecture docs live (e.g. docs/).
	DocsDir string `json:"-"`
	// RequireApprovalForNewTools when true prompts before first run of a newly registered tool.
	RequireApprovalForNewTools bool `json:"require_approval_for_new_tools"`
	// TokenBudget optional daily token cap; 0 = no limit. Core can count tokens per request and enforce or warn.
	TokenBudget int64 `json:"token_budget"`
	// AgentName is the name of the bot (loaded from config file during onboarding).
	AgentName string `json:"agent_name"`
	// AdminUserID is the ID of the trusted admin user (e.g. Nextcloud uid or "admin").
	AdminUserID string `json:"admin_user_id"`
	// ToolOutputMaxRunes caps tool output length (0 = no truncation). Set via HATTIEBOT_TOOL_OUTPUT_MAX_RUNES.
	ToolOutputMaxRunes int `json:"tool_output_max_runes"`

	// Embedding service (vector memory). When set, memorize/recall use this instead of LLM Embed.
	EmbeddingServiceURL   string `json:"embedding_service_url"`
	EmbeddingServiceAPIKey string `json:"embedding_service_api_key"`
	EmbeddingDimension   int    `json:"embedding_dimension"`

	// Nextcloud (HattieBridge webhook; optional Files/Passwords)
	NextcloudURL              string `json:"nextcloud_url"`
	HattieBridgeWebhookSecret string `json:"hattie_bridge_webhook_secret"`
	NextcloudBotUser          string `json:"nextcloud_bot_user"`
	NextcloudBotAppPassword   string `json:"nextcloud_bot_app_password"`
	// DefaultChannel is used for proactive routing when no user preference (e.g. "admin_term", "nextcloud_talk").
	DefaultChannel string `json:"default_channel"`
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
	defaultCh := os.Getenv("HATTIEBOT_DEFAULT_CHANNEL")
	cfg := &Config{
		OpenRouterAPIKey:        os.Getenv("OPENROUTER_API_KEY"),
		Model:                  os.Getenv("HATTIEBOT_MODEL"), // can be overridden by config file
		EnvModel:               os.Getenv("HATTIEBOT_MODEL"),
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
		NextcloudURL:              os.Getenv("NEXTCLOUD_URL"),
		HattieBridgeWebhookSecret: os.Getenv("HATTIEBOT_WEBHOOK_SECRET"),
		NextcloudBotUser:          os.Getenv("NEXTCLOUD_BOT_USER"),
		NextcloudBotAppPassword: os.Getenv("NEXTCLOUD_BOT_APP_PASSWORD"),
		DefaultChannel:         defaultCh,
		AdminUserID:            os.Getenv("NEXTCLOUD_ADMIN_USER"),
	}

	// Priority: Env < Config File.
	// We load config file (if exists) and OVERWRITE env vars.
	configPath := filepath.Join(configDir, "config.json")
	if data, err := os.ReadFile(configPath); err == nil {
		// Use a temporary map to check presence, or unmarshal into struct directly.
		// Unmarshal into struct works well: keys present in JSON will overwrite fields in struct.
		// Keys missing in JSON will simply leave struct fields untouched (keeping CLI/Env value).
		// Note: This relies on JSON having non-zero values. If JSON has "model": "", it wipes Env model.
		// Usually acceptable for config file.
		_ = json.Unmarshal(data, cfg)
	}

	return cfg
}
