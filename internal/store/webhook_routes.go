package store

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const webhookRoutesFile = "webhook_routes.json"

// WebhookRoute defines a configurable webhook endpoint.
type WebhookRoute struct {
	Path        string `json:"path"`
	ID          string `json:"id"`
	SecretHeader string `json:"secret_header"`
	SecretEnv   string `json:"secret_env"`
	AuthType    string `json:"auth_type"` // "header" or "hmac_sha256"
}

// LoadWebhookRoutes reads routes from $CONFIG_DIR/webhook_routes.json.
// Returns nil, nil if file does not exist.
func LoadWebhookRoutes(configDir string) ([]WebhookRoute, error) {
	p := filepath.Join(configDir, webhookRoutesFile)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var routes []WebhookRoute
	if err := json.Unmarshal(data, &routes); err != nil {
		return nil, err
	}
	return routes, nil
}

// SaveWebhookRoutes writes routes to $CONFIG_DIR/webhook_routes.json.
func SaveWebhookRoutes(configDir string, routes []WebhookRoute) error {
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	p := filepath.Join(configDir, webhookRoutesFile)
	data, err := json.MarshalIndent(routes, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0600)
}
