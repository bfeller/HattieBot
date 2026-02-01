package llmrouter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/store"
	"io"
	"net/http"
	"context"
)

// ProviderTemplate defines how to communicate with a generic LLM provider.
// It is loaded from JSON files in .hattiebot/providers/.
type ProviderTemplate struct {
	Name            string                 `json:"name"`
	Description     string                 `json:"description"`
	BaseURLTemplate string                 `json:"base_url_template"` // e.g. "{{base_url}}/api/generate"
	Method          string                 `json:"method"`            // GET, POST
	Headers         map[string]string      `json:"headers"`           // Static headers
	BodyTemplate    map[string]interface{} `json:"request_body_template"` // JSON body structure
	ResponsePath    string                 `json:"response_path"`     // dot notation path to content, e.g. "response" or "choices.0.message.content"
	PreRequestCmd   string                 `json:"pre_request_cmd"`   // Optional shell command
	PostRequestCmd  string                 `json:"post_request_cmd"`  // Optional shell command
}

// ProviderRegistry manages the loading and retrieval of ProviderTemplates.
type ProviderRegistry struct {
	templates map[string]ProviderTemplate
	configDir string
	mu        sync.RWMutex
}

// NewProviderRegistry creates a registry monitoring the given config dir.
func NewProviderRegistry(configDir string) *ProviderRegistry {
	return &ProviderRegistry{
		templates: make(map[string]ProviderTemplate),
		configDir: configDir,
	}
}

// LoadTemplates reads all *.json files from .hattiebot/providers/.
func (r *ProviderRegistry) LoadTemplates() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	providersDir := filepath.Join(r.configDir, "providers")
	if err := os.MkdirAll(providersDir, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(providersDir)
	if err != nil {
		return err
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(providersDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue // Log error?
		}
		var tmpl ProviderTemplate
		// Strip comments if any (simple JSON unmarshal for now)
		if err := json.Unmarshal(data, &tmpl); err != nil {
			continue // Log error?
		}
		if tmpl.Name != "" {
			r.templates[tmpl.Name] = tmpl
		}
	}
	return nil
}

// GetTemplate returns a copy of the requested template.
func (r *ProviderRegistry) GetTemplate(name string) (ProviderTemplate, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.templates[name]
	return t, ok
}

// SaveTemplate writes a template to disk.
func (r *ProviderRegistry) SaveTemplate(tmpl ProviderTemplate) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	providersDir := filepath.Join(r.configDir, "providers")
	if err := os.MkdirAll(providersDir, 0755); err != nil {
		return err
	}

	path := filepath.Join(providersDir, tmpl.Name+".json")
	data, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}
	r.templates[tmpl.Name] = tmpl
	return nil
}

// GenericProviderClient implements core.LLMClient using a template and instance config.
type GenericProviderClient struct {
	Template ProviderTemplate
	Instance store.LLMProviderEntry
	Route    store.ModelRouteEntry
	GetEnv   func(string) string
}

// Helper to render string templates
func renderString(tpl string, data map[string]interface{}) (string, error) {
	t, err := template.New("").Parse(tpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Helper to recursively render map templates
func renderMap(tmpl map[string]interface{}, data map[string]interface{}) (map[string]interface{}, error) {
	out := make(map[string]interface{})
	for k, v := range tmpl {
		switch val := v.(type) {
		case string:
			res, err := renderString(val, data)
			if err != nil {
				return nil, err
			}
			out[k] = res
		case map[string]interface{}:
			res, err := renderMap(val, data)
			if err != nil {
				return nil, err
			}
			out[k] = res
		default:
			out[k] = val
		}
	}
	return out, nil
}

func (c *GenericProviderClient) ChatCompletion(ctx context.Context, messages []core.Message) (string, error) {
	// 1. Prepare Data for Template
	// Flatten messages to prompt string for simple templates, or keep as list for advanced
	prompt := ""
	for _, m := range messages {
		prompt += fmt.Sprintf("%s: %s\n", m.Role, m.Content)
	}

	// Resolve API Key
	apiKey := ""
	if c.Instance.APIKeyEnv != "" {
		apiKey = c.GetEnv(c.Instance.APIKeyEnv)
	}

	data := map[string]interface{}{
		"model":    c.Route.Model,
		"prompt":   prompt,
		"base_url": c.Instance.BaseURL,
		"api_key":  apiKey,
	}

	// 2. Render URL
	url, err := renderString(c.Template.BaseURLTemplate, data)
	if err != nil {
		return "", fmt.Errorf("render url: %w", err)
	}

	// 3. Render Body
	bodyMap, err := renderMap(c.Template.BodyTemplate, data)
	if err != nil {
		return "", fmt.Errorf("render body: %w", err)
	}
	bodyBytes, _ := json.Marshal(bodyMap)

	// 4. Pre-request hook (optional)
	if c.Template.PreRequestCmd != "" {
		// Run shell cmd... (omitted for now for safety/complexity, can add later)
	}

	// 5. Make Request
	req, err := http.NewRequestWithContext(ctx, c.Template.Method, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", err
	}
	for k, v := range c.Template.Headers {
		req.Header.Set(k, v)
		if k == "Authorization" && strings.Contains(v, "{{api_key}}") {
			req.Header.Set(k, strings.ReplaceAll(v, "{{api_key}}", apiKey))
		}
	}
	// Default auth header if not specified but key exists?
	if apiKey != "" && req.Header.Get("Authorization") == "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("provider error %d: %s", resp.StatusCode, string(respBody))
	}

	// 6. Parse Response
	var result interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}

	// Navigate Path (e.g. "response" or "choices.0.message.content")
	parts := strings.Split(c.Template.ResponsePath, ".")
	current := result
	for _, part := range parts {
		if m, ok := current.(map[string]interface{}); ok {
			if val, exists := m[part]; exists {
				current = val
			} else {
				return "", fmt.Errorf("field %s not found in object", part)
			}
		} else if l, ok := current.([]interface{}); ok {
			// Handle array index
			var idx int
			if _, err := fmt.Sscanf(part, "%d", &idx); err == nil {
				if idx >= 0 && idx < len(l) {
					current = l[idx]
				} else {
					return "", fmt.Errorf("index %d out of bounds (len %d)", idx, len(l))
				}
			} else {
				return "", fmt.Errorf("expected array index, got %s", part)
			}
		} else {
			return "", fmt.Errorf("cannot navigate %s on %T", part, current)
		}
	}

	if s, ok := current.(string); ok {
		return s, nil
	}
	return fmt.Sprintf("%v", current), nil // Allow non-string result converted to string
}

func (c *GenericProviderClient) ChatCompletionWithTools(ctx context.Context, messages []core.Message, tools []core.ToolDefinition) (string, []core.ToolCall, error) {
	return "", nil, fmt.Errorf("generic provider tools not yet implemented")
}

func (c *GenericProviderClient) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, fmt.Errorf("generic provider embed not yet implemented")
}
