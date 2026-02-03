package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ConfigFile holds persisted config (API key, model) for first boot and beyond.
// Stored in ConfigDir as config.json. Do not commit this file if it contains secrets.
type ConfigFile struct {
	OpenRouterAPIKey string `json:"openrouter_api_key,omitempty"`
	Model            string `json:"model,omitempty"`
	// New fields for Agentic Upgrade
	AgentName    string `json:"agent_name,omitempty"`
	WorkspaceDir string `json:"workspace_dir,omitempty"`
	RiskAccepted bool   `json:"risk_accepted,omitempty"`
	AdminUserID  string `json:"admin_user_id,omitempty"`

	// Embedding service (vector memory)
	EmbeddingServiceURL   string `json:"embedding_service_url,omitempty"`
	EmbeddingServiceAPIKey string `json:"embedding_service_api_key,omitempty"`
	EmbeddingDimension   int    `json:"embedding_dimension,omitempty"`

	// Nextcloud (HattieBridge webhook, optional Files/Passwords)
	NextcloudURL               string `json:"nextcloud_url,omitempty"`
	HattieBridgeWebhookSecret  string `json:"hattiebridge_webhook_secret,omitempty"`
	NextcloudBotUser           string `json:"nextcloud_bot_user,omitempty"`
	NextcloudBotAppPassword    string `json:"nextcloud_bot_app_password,omitempty"`
	NextcloudIntroSent         bool   `json:"nextcloud_intro_sent,omitempty"`
	DefaultChannel             string `json:"default_channel,omitempty"`
}

// LoadConfigFile reads config from dir/config.json. Missing file returns nil, nil.
func LoadConfigFile(dir string) (*ConfigFile, error) {
	p := filepath.Join(dir, "config.json")
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var c ConfigFile
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// SaveConfigFile writes config to dir/config.json.
func SaveConfigFile(dir string, c *ConfigFile) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	p := filepath.Join(dir, "config.json")
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}

// WriteSystemPurpose writes the cleaned system purpose text to dir/system_purpose.txt.
func WriteSystemPurpose(dir, content string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	p := filepath.Join(dir, "system_purpose.txt")
	return os.WriteFile(p, []byte(content), 0600)
}

// ReadSystemPurpose reads system_purpose.txt from dir.
func ReadSystemPurpose(dir string) (string, error) {
	p := filepath.Join(dir, "system_purpose.txt")
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
// SystemConfig defines which component implementations to use.
// Stored in ConfigDir as system.json.
type SystemConfig struct {
	ContextSelector string `json:"context_selector"`
	LLMClient       string `json:"llm_client"`
	ToolExecutor    string `json:"tool_executor"`
}

var DefaultSystemConfig = &SystemConfig{
	ContextSelector: "default",
	LLMClient:       "default",
	ToolExecutor:    "default",
}

// LoadSystemConfig reads system.json. Returns defaults if missing.
func LoadSystemConfig(dir string) (*SystemConfig, error) {
	p := filepath.Join(dir, "system.json")
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultSystemConfig, nil
		}
		return nil, err
	}
	var c SystemConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return DefaultSystemConfig, nil // Fallback on corrupted config
	}
	// Fill empty fields with defaults
	if c.ContextSelector == "" {
		c.ContextSelector = "default"
	}
	if c.LLMClient == "" {
		c.LLMClient = "default"
	}
	if c.ToolExecutor == "" {
		c.ToolExecutor = "default"
	}
	return &c, nil
}

// SaveSystemConfig writes system.json.
func SaveSystemConfig(dir string, c *SystemConfig) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	p := filepath.Join(dir, "system.json")
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}
