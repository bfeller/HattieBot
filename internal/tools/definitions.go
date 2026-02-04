package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/hattiebot/hattiebot/internal/config"
	"github.com/hattiebot/hattiebot/internal/core"
	"regexp"
	"github.com/hattiebot/hattiebot/internal/gateway"
	"github.com/hattiebot/hattiebot/internal/secrets"
	"github.com/hattiebot/hattiebot/internal/health"
	"github.com/hattiebot/hattiebot/internal/openrouter"
	"github.com/hattiebot/hattiebot/internal/registry"
	"github.com/hattiebot/hattiebot/internal/store"
	"github.com/hattiebot/hattiebot/internal/tools/builtin"
	"github.com/hattiebot/hattiebot/internal/tools/nextcloud"
)

func init() {
	registry.RegisterExecutor("default", func(cfg *config.Config, db *store.DB, client core.LLMClient) (core.ToolExecutor, error) {
		return &Executor{
			WorkspaceDir: cfg.WorkspaceDir,
			DocsDir:      cfg.DocsDir,
			ConfigDir:    cfg.ConfigDir,
			Config:       cfg,
			DB:           db,
			Client:       client,
		}, nil
	})
}

// Init registers dynamic built-in tools that require dependencies.
func Init(db *store.DB) {
	builtin.Register(builtin.NewManageJobTool(db))
}

// BuiltinToolDefs returns OpenRouter tool definitions for all built-in tools.
func BuiltinToolDefs() []openrouter.ToolDefinition {
	defs := []openrouter.ToolDefinition{}
	for _, t := range builtin.Registry {
		defs = append(defs, t.Definition())
	}
	
	// Legacy static definitions (to be refactored)
	legacyDefs := []openrouter.ToolDefinition{
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "run_terminal_cmd",
				Description: "Execute a shell command in a configurable working directory. Capture stdout, stderr, and exit code. Sandboxing is the Docker container.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"work_dir": map[string]string{"type": "string", "description": "Working directory (default: workspace root)"},
						"command":  map[string]string{"type": "string", "description": "Shell command to run. Use environment variables (e.g. $MY_SECRET) for secrets."},
						"env_vars": map[string]string{"type": "string", "description": "Environment variables to set. Map variable names to values (or {{secret:Title}} refs)."},
					},
					"required": []string{"command"},
				},
			},
			Policy: "restricted",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "read_file",
				Description: "Read the contents of a file. Path is relative to workspace.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]string{"type": "string", "description": "Relative path to file"},
					},
					"required": []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "write_file",
				Description: "Write content to a file. Overwrites if exists. Path is relative to workspace.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path":    map[string]string{"type": "string", "description": "Relative path to file"},
						"content": map[string]string{"type": "string", "description": "Content to write"},
					},
					"required": []string{"path", "content"},
				},
			},
			Policy: "restricted",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "list_dir",
				Description: "List directory entries (name, is_dir). Path is relative to workspace.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]string{"type": "string", "description": "Relative path to directory (default: .)"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "read_architecture",
				Description: "Read the architecture docs (docs/architecture.md, docs/tools.md, docs/creating-tools.md).",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "autohand_cli",
				Description: "Invoke Autohand Code CLI with an instruction to write or edit code. OpenRouter should be configured via env.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"instruction": map[string]string{"type": "string", "description": "Natural language instruction for the CLI"},
						"env_vars": map[string]string{"type": "string", "description": "Environment variables to set. Map variable names to values (or {{secret:Title}} refs)."},
					},
					"required": []string{"instruction"},
				},
			},
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "execute_registered_tool",
				Description: "Run a tool registered in tools_registry by name. Pass JSON args; the binary receives them on stdin.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]string{"type": "string", "description": "Tool name in registry"},
						"args": map[string]interface{}{"type": "object", "description": "JSON object of arguments"},
						"env_vars": map[string]string{"type": "string", "description": "Environment variables to set."},
					},
					"required": []string{"name"},
				},
			},
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "register_tool",
				Description: "Register a new tool that you have built. The binary must exist and follow the JSON-in/JSON-out contract.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name":        map[string]string{"type": "string", "description": "Name of the tool (e.g. 'fetch_url')"},
						"binary_path": map[string]string{"type": "string", "description": "Absolute path to the executable binary"},
						"description": map[string]string{"type": "string", "description": "Description of what the tool does"},
						"input_schema": map[string]string{"type": "string", "description": "JSON Schema for the arguments (optional)"},
						"force_update": map[string]interface{}{"type": "boolean", "description": "Set to true to overwrite existing tool"},
					},
					"required": []string{"name", "binary_path", "description"},
				},
			},
			Policy: "restricted",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "delete_tool",
				Description: "Delete a registered tool by name.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name": map[string]string{"type": "string", "description": "Name of the tool to delete"},
					},
					"required": []string{"name"},
				},
			},
			Policy: "admin_only",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "approve_user",
				Description: "Approve a pending user or change their trust level (admin only).",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"user_id": map[string]string{"type": "string", "description": "User ID to approve"},
						"level":   map[string]interface{}{"type": "string", "enum": []string{"trusted", "admin", "guest", "restricted", "blocked"}, "description": "New trust level (default: trusted)"},
					},
					"required": []string{"user_id"},
				},
			},
			Policy: "admin_only",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "block_user",
				Description: "Block a user from accessing the bot (admin only).",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"user_id": map[string]string{"type": "string", "description": "User ID to block"},
					},
					"required": []string{"user_id"},
				},
			},
			Policy: "admin_only",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "list_users",
				Description: "List users known to the bot (admin only).",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"filter_level": map[string]interface{}{"type": "string", "enum": []string{"trusted", "admin", "guest", "restricted", "blocked"}, "description": "Filter by trust level"},
					},
				},
			},
			Policy: "admin_only",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "manage_user_preference",
				Description: "Persistent memory for key-value facts about the current user. Use this to remember user details (name, preferences, context).",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action":   map[string]interface{}{"type": "string", "enum": []string{"set", "get", "search"}, "description": "Action: set, get, or search"},
						"key":      map[string]string{"type": "string", "description": "Unique key for the fact"},
						"value":    map[string]string{"type": "string", "description": "The fact content (for set)"},
						"category": map[string]string{"type": "string", "description": "Category tag (optional)"},
						"query":    map[string]string{"type": "string", "description": "Search query"},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "memorize",
				Description: "Store a text chunk into long-term vector memory.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"content": map[string]string{"type": "string", "description": "Text to memorize"},
						"source":  map[string]string{"type": "string", "description": "Origin of the memory (e.g. user, reflection)"},
					},
					"required": []string{"content"},
				},
			},
			Policy: "safe",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "recall_memories",
				Description: "Search long-term memory for relevant chunks using vector similarity.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]string{"type": "string", "description": "Search query"},
						"limit": map[string]interface{}{"type": "integer", "description": "Max results (default 5)"},
					},
					"required": []string{"query"},
				},
			},
			Policy: "safe",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "run_sandboxed",
				Description: "Execute a command securely inside a disposable Docker container.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"image":    map[string]string{"type": "string", "description": "Docker image (default: debian:bookworm-slim)"},
						"command":  map[string]string{"type": "string", "description": "Command to run. Use env vars for secrets."},
						"work_dir": map[string]string{"type": "string", "description": "Working directory inside container"},
						"env_vars": map[string]string{"type": "string", "description": "Environment variables to set inside container."},
					},
					"required": []string{"command"},
				},
			},
			Policy: "restricted",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "manage_schedule",
				Description: "Create, list, or delete scheduled reminders and recurring tasks. remind=message user; execute_tool=run tool directly; agent_prompt=agent reasons and acts (use autonomous=true for background tasks like 'check email and file receipts').",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action":         map[string]interface{}{"type": "string", "enum": []string{"create", "list", "delete", "pause"}, "description": "Action to perform"},
						"description":    map[string]string{"type": "string", "description": "What to remind or do"},
						"action_type":    map[string]interface{}{"type": "string", "enum": []string{"remind", "execute_tool", "agent_prompt"}, "description": "remind=message user; execute_tool=run tool; agent_prompt=agent reasons/acts"},
						"schedule_type":  map[string]interface{}{"type": "string", "enum": []string{"once", "daily", "weekly", "hourly"}, "description": "Frequency"},
						"run_at":         map[string]string{"type": "string", "description": "ISO datetime for 'once', or time like '09:00' for recurring"},
						"id":             map[string]interface{}{"type": "integer", "description": "Plan ID (for delete/pause)"},
						"prompt":         map[string]string{"type": "string", "description": "For agent_prompt: task prompt (e.g. 'Run self-reflection')"},
						"autonomous":     map[string]string{"type": "boolean", "description": "For agent_prompt: true=run silently, notify only via notify_user"},
						"tool":           map[string]string{"type": "string", "description": "For execute_tool: tool name (e.g. self_reflect)"},
						"tool_args":      map[string]interface{}{"type": "object", "description": "For execute_tool: JSON args for the tool"},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "install_skill",
				Description: "Install a new skill or tool package (go, brew, npm).",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"manager": map[string]interface{}{"type": "string", "enum": []string{"go", "brew", "npm"}, "description": "Package manager to use"},
						"package": map[string]string{"type": "string", "description": "Package name (e.g. github.com/user/repo@latest, jq)"},
					},
					"required": []string{"manager", "package"},
				},
			},
			Policy: "restricted",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "list_skills",
				Description: "List installed skills.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "system_status",
				Description: "Get comprehensive system status including health of all components, message count, log entries, and recent errors.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "read_logs",
				Description: "Read recent system logs with optional filtering by level and component.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"level":     map[string]interface{}{"type": "string", "enum": []string{"error", "warn", "info"}, "description": "Filter by log level"},
						"component": map[string]string{"type": "string", "description": "Filter by component (db, llm, gateway, compactor)"},
						"limit":     map[string]string{"type": "integer", "description": "Max entries to return (default 50, max 200)"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "log_self_modification",
				Description: "Record a self-modification entry when you change core code (internal/*, cmd/*, Dockerfile) or workspace config. This log survives rebuilds—if a software update wipes your changes, you or the user can reference it to re-apply them. Do NOT log changes to $CONFIG_DIR/tools (registered tools).",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"file_paths":  map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}, "description": "Paths you modified (e.g. internal/scheduler/runner.go)"},
						"change_type": map[string]interface{}{"type": "string", "enum": []string{"core_code", "config", "registered_tool"}, "description": "Type of change"},
						"description": map[string]string{"type": "string", "description": "Brief summary of what changed and why"},
						"context":     map[string]string{"type": "string", "description": "Optional: user request or trigger"},
					},
					"required": []string{"file_paths", "change_type", "description"},
				},
			},
			Policy: "restricted",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "read_self_modification_log",
				Description: "Read the self-modification changelog. Use when the user asks what changes you've made, or when you need to reference past optimizations that may have been lost in a rebuild.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"limit": map[string]interface{}{"type": "integer", "description": "Max entries to return (default 20)"},
					},
				},
			},
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "list_webhook_routes",
				Description: "List configured webhook routes from $CONFIG_DIR/webhook_routes.json. Use to see what external webhook endpoints are registered.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "add_webhook_route",
				Description: "Add a webhook route for external services (GitHub, Stripe, etc.). Path must start with /webhook/ and not be /webhook/talk. Secret is read from env var (secret_env). Auth type: header (exact match) or hmac_sha256 (GitHub-style).",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path":          map[string]string{"type": "string", "description": "URL path (e.g. /webhook/github)"},
						"id":            map[string]string{"type": "string", "description": "Short identifier (e.g. github)"},
						"secret_header": map[string]string{"type": "string", "description": "Header name for secret/signature"},
						"secret_env":    map[string]string{"type": "string", "description": "Env var name for secret value (optional)"},
						"secret_source": map[string]string{"type": "string", "description": "Source of secret: 'env' or 'passwords' (default: env)"},
						"secret_key":    map[string]string{"type": "string", "description": "Key name for the secret (e.g. secret title in Passwords app)"},
						"auth_type":     map[string]interface{}{"type": "string", "enum": []string{"header", "hmac_sha256"}, "description": "Auth type"},
						"target_tool":   map[string]string{"type": "string", "description": "Name of the tool to execute (required)"},
						"target_args":   map[string]string{"type": "string", "description": "JSON arguments for the tool. Use {{payload}} for webhook body."},
					},
					"required": []string{"path", "id", "secret_header", "auth_type", "target_tool"},
				},
			},
			Policy: "restricted",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "remove_webhook_route",
				Description: "Remove a webhook route by path or id.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path_or_id": map[string]string{"type": "string", "description": "Path (e.g. /webhook/github) or id (e.g. github)"},
					},
					"required": []string{"path_or_id"},
				},
			},
			Policy: "restricted",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "self_reflect",
				Description: "Trigger a self-reflection analysis to review system health and identify any issues. Only suggests improvements if there are clear problems.",
				Parameters: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "notify_user",
				Description: "Send a message to the user. Use when running autonomously to notify about errors, anomalies, or important findings. If the task completes successfully with nothing notable, do NOT call this.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"message": map[string]string{"type": "string", "description": "Message to send to the user"},
					},
					"required": []string{"message"},
				},
			},
			Policy: "restricted",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "spawn_submind",
				Description: "Spawn a focused sub-mind for a specific task. Use for tool creation, code analysis, reflection, planning, or custom modes. Pass session_id to resume an existing session.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"mode":       map[string]string{"type": "string", "description": "Sub-mind mode (reflection, tool_creation, code_analysis, planning, or custom)"},
						"task":       map[string]string{"type": "string", "description": "Task description for the sub-mind"},
						"session_id": map[string]string{"type": "integer", "description": "If provided, resume this sub-mind session instead of starting a new one."},
					},
					"required": []string{"mode", "task"},
				},
			},
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "manage_submind",
				Description: "Create, update, delete, or list sub-mind modes.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action":        map[string]interface{}{"type": "string", "enum": []string{"create", "update", "delete", "list", "list_sessions"}, "description": "Action to perform"},
						"name":          map[string]string{"type": "string", "description": "Sub-mind name (for create/update/delete)"},
						"system_prompt": map[string]string{"type": "string", "description": "System prompt for the sub-mind (for create/update)"},
						"allowed_tools": map[string]interface{}{"type": "array", "items": map[string]string{"type": "string"}, "description": "Tools available to sub-mind"},
						"max_turns":     map[string]string{"type": "integer", "description": "Maximum turns (default 10)"},
					},
					"required": []string{"action"},
				},
			},
			Policy: "admin_only",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "manage_llm_provider",
				Description: "Manage generic LLM provider templates and routing configuration. Use this to add support for Ollama, vLLM, etc.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action":        map[string]interface{}{"type": "string", "enum": []string{"list_templates", "get_template", "save_template", "list_providers", "register_provider", "set_route"}, "description": "Action to perform"},
						"template_name": map[string]string{"type": "string", "description": "Name of template (for start/get/save)"},
						"template_body": map[string]interface{}{"type": "object", "description": "JSON body of ProviderTemplate (for save)"},
						"provider_name": map[string]string{"type": "string", "description": "Name of provider instance (e.g. 'my_ollama')"},
						"provider_config": map[string]interface{}{"type": "object", "description": "JSON body of LLMProviderEntry (type, api_key_env, base_url)"},
						"route":         map[string]string{"type": "string", "description": "Route key (default: 'default')"},
						"model":         map[string]string{"type": "string", "description": "Target model ID"},
					},
					"required": []string{"action"},
				},
			},
			Policy: "admin_only",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "manage_embedding_provider",
				Description: "Manage embedding provider configuration and default provider. Use this to add or switch embedding services (e.g. EmbeddingGood).",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action":         map[string]interface{}{"type": "string", "enum": []string{"list_providers", "register_provider", "set_default"}, "description": "Action to perform"},
						"provider_name":  map[string]string{"type": "string", "description": "Name of provider instance (e.g. 'embeddinggood')"},
						"provider_config": map[string]interface{}{"type": "object", "description": "JSON body of EmbeddingProviderEntry (type, base_url_env, api_key_env, dimension)"},
					},
					"required": []string{"action"},
				},
			},
			Policy: "admin_only",
		},
		// Nextcloud Tools
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "request_nextcloud_ocs",
				Description: "Execute a Nextcloud OCS API request (Provisioning API, etc.) as the bot/admin. Use for managing users, groups, apps that have OCS endpoints.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"method":   map[string]interface{}{"type": "string", "enum": []string{"GET", "POST", "PUT", "DELETE"}, "description": "HTTP Method"},
						"endpoint": map[string]string{"type": "string", "description": "API endpoint (e.g. /cloud/users). /ocs/v1.php is prepended automatically."},
						"params":   map[string]interface{}{"type": "object", "description": "Query params (GET) or Form fields (POST/PUT). Map strings to strings."},
					},
					"required": []string{"method", "endpoint"},
				},
			},
			Policy: "restricted", // Admin/Bot power
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "list_nextcloud_files",
				Description: "List files in Nextcloud via WebDAV.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]string{"type": "string", "description": "Path to list (e.g. / or /Documents)"},
					},
					"required": []string{"path"},
				},
			},
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "read_nextcloud_file",
				Description: "Read file content from Nextcloud via WebDAV.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]string{"type": "string", "description": "Path to file"},
					},
					"required": []string{"path"},
				},
			},
		},

		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "manage_context_doc",
				Description: "Create, update, delete, list, or toggle active context documents. Content from active documents is injected into the system prompt.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action":      map[string]interface{}{"type": "string", "enum": []string{"create", "update", "delete", "list", "read", "toggle"}, "description": "Action to perform"},
						"title":       map[string]string{"type": "string", "description": "Document title (unique)"},
						"content":     map[string]string{"type": "string", "description": "Document content (Markdown preferred)"},
						"description": map[string]string{"type": "string", "description": "Brief description of the document"},
						"active":      map[string]interface{}{"type": "boolean", "description": "For toggle: true to activate, false to deactivate"},
					},
					"required": []string{"action"},
				},
			},
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "get_secret",
				Description: "Retrieve a secure reference to a password/secret from Nextcloud Passwords app.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"query": map[string]string{"type": "string", "description": "Title to search for"},
					},
					"required": []string{"query"},
				},
			},
			Policy: "restricted",
		},
		{
			Type: "function",
			Function: openrouter.FunctionSpec{
				Name:        "store_secret",
				Description: "Store a new secret in Nextcloud Passwords app and share it with Admin.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"title":    map[string]string{"type": "string", "description": "Title/Label for the secret"},
						"password": map[string]string{"type": "string", "description": "The secret/password"},
						"login":    map[string]string{"type": "string", "description": "Username (optional)"},
						"url":      map[string]string{"type": "string", "description": "URL (optional)"},
						"notes":    map[string]string{"type": "string", "description": "Notes (optional)"},
					},
					"required": []string{"title", "password"},
				},
			},
			Policy: "restricted",
		},
	}
	return append(defs, legacyDefs...)
}

// Helper to get user ID from context
func getUserID(ctx context.Context) (string, error) {
	uid := ctx.Value("user_id")
	if uid == nil {
		return "", fmt.Errorf("user context required")
	}
	return uid.(string), nil
}

// Executor runs a built-in or registered tool by name.
type Executor struct {
	WorkspaceDir    string
	DocsDir         string
	ConfigDir       string
	Config          *config.Config
	DB              *store.DB
	Client          core.LLMClient
	Embedder        core.EmbeddingClient // When set, memorize/recall use this; else fall back to Client.Embed
	Gateway         *gateway.Gateway
	Router          *gateway.Router // For notify_user (proactive delivery)
	LogStore        *store.LogStore
	HealthReg       *health.Registry
	TokenBudget     int
	Spawner         core.SubmindSpawner  // For spawning sub-minds
	SubmindRegistry core.SubmindRegistry // For managing sub-minds
	SecretStore     *secrets.MultiStore
}

func (e *Executor) SetSpawner(spawner core.SubmindSpawner) {
	e.Spawner = spawner
}

// embed returns embedding for text; uses Embedder when set (embedType document/query), else Client.Embed.
func (e *Executor) embed(ctx context.Context, text string, embedType string) ([]float32, error) {
	if e.Embedder != nil {
		return e.Embedder.Embed(ctx, text, embedType)
	}
	return e.Client.Embed(ctx, text)
}

// Execute runs the tool by name with the given JSON arguments; returns JSON result.
func (e *Executor) Execute(ctx context.Context, name, argsJSON string) (string, error) {
	// Safety timeout: prevent tools from hanging the agent loop indefinitely.
	// Default to 2 minutes, but allow known long-running tools (builds, CLI agents) more time.
	timeout := 2 * time.Minute
	if name == "run_terminal_cmd" || name == "autohand_cli" || name == "spawn_submind" {
		timeout = 15 * time.Minute
	}

	// Secret Resolution
	// Look for {{secret:key}} and replace with value from SecretStore (default source: passwords)
	if e.SecretStore != nil && strings.Contains(argsJSON, "{{secret:") {
		re := regexp.MustCompile(`\{\{secret:([^}]+)\}\}`)
		argsJSON = re.ReplaceAllStringFunc(argsJSON, func(match string) string {
			key := re.FindStringSubmatch(match)[1]
			// TODO: Support {{secret:source:key}} syntax? For now assume passwords app.
			// If key starts with "env:", use env source.
			source := "passwords"
			if strings.HasPrefix(key, "env:") {
				source = "env"
				key = strings.TrimPrefix(key, "env:")
			}
			
			val, err := e.SecretStore.GetSecret(source, key)
			if err != nil {
				return "ERROR_MISSING_SECRET" 
			}
			// JSON Escape: The secret is likely inside a JSON string value.
			// e.g. "env_vars": {"KEY": "{{secret:foo}}"} -> "env_vars": {"KEY": "value"}
			// We need to ensure 'value' is properly escaped for JSON.
			
			// Simple replace works if we trust the secret content not to break the JSON structure excessively
			// (e.g. if secret has quotes).
			b, _ := json.Marshal(val)
			s := string(b)
			// json.Marshal returns "value". We need inner content if we are substituting inside existing quotes?
			// The original regex match {{secret:...}} is usually inside quotes in the argsJSON string.
			// argsJSON: { "key": "{{secret:foo}}" }
			// replacement: "password" (with quotes) -> { "key": ""password"" } -> INVALID
			
			// If the original string had quotes around the placeholder, we should be careful.
			// But the regex doesn't match the quotes. 
			// match is {{secret:foo}}
			// context: "...": "{{secret:foo}}"
			
			// If we return the raw value, e.g. basicpassword, result is "...": "basicpassword". Good.
			// If raw value has quotes: pass"word, result is "...": "pass"word". BAD (invalid JSON).
			// So we MUST escape the value, BUT without the surrounding quotes from json.Marshal.
			
			if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
				// Strip surrounding quotes from Marshal result, but keep internal escaping (e.g. \" for ")
				return s[1 : len(s)-1]
			}
			return val
		})
		
		if strings.Contains(argsJSON, "ERROR_MISSING_SECRET") {
			return `{"error": "failed to resolve one or more secrets"}`, nil
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 1. Check updated builtin registry
	if tool, ok := builtin.Registry[name]; ok {
		return tool.Execute(ctx, argsJSON)
	}

	switch name {
	case "run_terminal_cmd":
		return RunTerminalTool(ctx, e.WorkspaceDir, argsJSON)
	case "read_file":
		return ReadFileTool(ctx, e.WorkspaceDir, argsJSON)
	case "write_file":
		return WriteFileTool(ctx, e.WorkspaceDir, argsJSON)
	case "list_dir":
		return ListDirTool(ctx, e.WorkspaceDir, argsJSON)
	case "read_architecture":
		return ReadArchitectureTool(ctx, e.DocsDir, argsJSON)

	case "autohand_cli":
		// Wrapper function inside autohand.go handles JSON parsing and env_vars extraction
		return AutohandCLITool(ctx, argsJSON)
	case "manage_context_doc":
		return ManageContextDocTool(ctx, e.DB, argsJSON)

	case "manage_user_preference":
		userID, err := getUserID(ctx)
		if err != nil {
			return ErrJSON(err), nil
		}

		var args struct {
			Action   string `json:"action"` // set, get, search
			Key      string `json:"key"`
			Value    string `json:"value"`
			Category string `json:"category"`
			Query    string `json:"query"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		switch args.Action {
		case "set":
			if err := e.DB.SetFact(ctx, userID, args.Key, args.Value, args.Category); err != nil {
				return ErrJSON(err), nil
			}
			return `{"status": "saved"}`, nil
		case "get":
			fact, err := e.DB.GetFact(ctx, userID, args.Key)
			if err != nil {
				return ErrJSON(err), nil
			}
			if fact == nil {
				return `{"error": "not found"}`, nil
			}
			b, _ := json.Marshal(fact)
			return string(b), nil
		case "search":
			facts, err := e.DB.SearchFacts(ctx, userID, args.Query)
			if err != nil {
				return ErrJSON(err), nil
			}
			b, _ := json.Marshal(facts)
			return string(b), nil
		default:
			return ErrJSON(fmt.Errorf("unknown action: %s", args.Action)), nil
		}
	case "memorize":
		var args struct {
			Content string `json:"content"`
			Source  string `json:"source"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		// Embed (prefer Embedder; fall back to LLM client)
		emb, err := e.embed(ctx, args.Content, "document")
		if err != nil {
			return ErrJSON(fmt.Errorf("embed failed: %w", err)), nil
		}
		// Store
		if err := e.DB.InsertChunk(ctx, args.Content, args.Source, emb); err != nil {
			return ErrJSON(err), nil
		}
		return `{"status": "memorized"}`, nil
	case "recall_memories":
		var args struct {
			Query string `json:"query"`
			Limit int    `json:"limit"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		if args.Limit <= 0 {
			args.Limit = 5
		}
		// Embed Query (prefer Embedder; fall back to LLM client)
		emb, err := e.embed(ctx, args.Query, "query")
		if err != nil {
			return ErrJSON(fmt.Errorf("embed failed: %w", err)), nil
		}
		chunks, err := e.DB.SearchChunks(ctx, emb, args.Limit)
		if err != nil {
			return ErrJSON(err), nil
		}
		// Return minimal JSON
		type RecallResult struct {
			Content string  `json:"content"`
			Score   float64 `json:"score"`
			Source  string  `json:"source"`
		}
		var results []RecallResult
		for _, c := range chunks {
			results = append(results, RecallResult{Content: c.Content, Score: c.Score, Source: c.Source})
		}
		b, _ := json.Marshal(results)
		return string(b), nil
	case "run_sandboxed":
		var args struct {
			Image    string            `json:"image"`
			Command  string            `json:"command"`
			WorkDir  string            `json:"work_dir"`
			EnvVars  map[string]string `json:"env_vars"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		if args.Image == "" {
			args.Image = "debian:bookworm-slim"
		}
		if args.WorkDir == "" {
			args.WorkDir = "/workspace"
		}

		// Security: Validate WorkDir?
		// Note: Host mounting /workspace:/workspace allows access to project source.
		
		cmdArgs := []string{"run", "--rm", "-i", "-v", "/workspace:/workspace", "-w", args.WorkDir}
		
		// Add env vars
		for k, v := range args.EnvVars {
			cmdArgs = append(cmdArgs, "-e", fmt.Sprintf("%s=%s", k, v))
		}
		
		cmdArgs = append(cmdArgs, args.Image, "/bin/sh", "-c", args.Command)
		cmd := exec.CommandContext(ctx, "docker", cmdArgs...)
		var out bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &stderr
		
		runErr := cmd.Run()
		
		resp := map[string]string{
			"stdout": out.String(),
			"stderr": stderr.String(),
		}
		if runErr != nil {
			resp["error"] = runErr.Error()
		}
		
		b, _ := json.Marshal(resp)
		return string(b), nil
	case "manage_schedule":
		userID, err := getUserID(ctx)
		if err != nil {
			return ErrJSON(err), nil
		}
		var args struct {
			Action       string                 `json:"action"`
			Description  string                 `json:"description"`
			ActionType   string                 `json:"action_type"`
			ScheduleType string                 `json:"schedule_type"`
			RunAt        string                 `json:"run_at"`
			ID           int64                  `json:"id"`
			Prompt       string                 `json:"prompt"`
			Autonomous   bool                   `json:"autonomous"`
			Tool         string                 `json:"tool"`
			ToolArgs     map[string]interface{} `json:"tool_args"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		switch args.Action {
		case "create":
			// Parse run_at into time
			var nextRun time.Time
			var err error
			if args.ScheduleType == "once" {
				nextRun, err = time.Parse(time.RFC3339, args.RunAt)
				if err != nil {
					// Try simpler formats
					nextRun, err = time.Parse("2006-01-02 15:04", args.RunAt)
				}
			} else {
				// For recurring, parse time and set to today or tomorrow
				t, parseErr := time.Parse("15:04", args.RunAt)
				if parseErr != nil {
					t = time.Now().Add(1 * time.Hour) // Default: 1 hour from now
				} else {
					now := time.Now()
					nextRun = time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
					if nextRun.Before(now) {
						nextRun = nextRun.Add(24 * time.Hour)
					}
				}
				if nextRun.IsZero() {
					nextRun = time.Now().Add(1 * time.Hour)
				}
			}
			if err != nil && nextRun.IsZero() {
				nextRun = time.Now().Add(24 * time.Hour) // Default fallback
			}
			actionType := args.ActionType
			if actionType == "" {
				actionType = "remind"
			}
			var actionPayload string
			switch actionType {
			case "execute_tool":
				if args.Tool == "" {
					return ErrJSON(fmt.Errorf("execute_tool requires tool name")), nil
				}
				toolArgs := args.ToolArgs
				if toolArgs == nil {
					toolArgs = map[string]interface{}{}
				}
				payload := map[string]interface{}{"tool": args.Tool, "args": toolArgs}
				if b, err := json.Marshal(payload); err == nil {
					actionPayload = string(b)
				}
			case "agent_prompt":
				payload := map[string]interface{}{"prompt": args.Prompt, "autonomous": args.Autonomous}
				if args.Prompt == "" {
					payload["prompt"] = args.Description
				}
				if b, err := json.Marshal(payload); err == nil {
					actionPayload = string(b)
				}
			}
			id, err := e.DB.CreatePlan(ctx, userID, args.Description, actionType, actionPayload, args.ScheduleType, args.RunAt, nextRun)
			if err != nil {
				return ErrJSON(err), nil
			}
			return fmt.Sprintf(`{"id": %d, "status": "scheduled", "next_run": "%s"}`, id, nextRun.Format(time.RFC3339)), nil
		case "list":
			plans, err := e.DB.ListPlans(ctx, userID, "active")
			if err != nil {
				return ErrJSON(err), nil
			}
			b, _ := json.Marshal(plans)
			return string(b), nil
		case "delete":
			if err := e.DB.DeletePlan(ctx, args.ID); err != nil {
				return ErrJSON(err), nil
			}
			return `{"status": "deleted"}`, nil
		case "pause":
			if err := e.DB.UpdatePlanStatus(ctx, args.ID, "paused"); err != nil {
				return ErrJSON(err), nil
			}
			return `{"status": "paused"}`, nil
		default:
			return ErrJSON(fmt.Errorf("unknown action: %s", args.Action)), nil
		}
	case "approve_user":
		return ApproveUser(ctx, e.DB, argsJSON)
	case "block_user":
		return BlockUser(ctx, e.DB, argsJSON)
	case "list_users":
		return ListUsers(ctx, e.DB, argsJSON)
	case "register_tool":
		var args struct {
			Name        string `json:"name"`
			BinaryPath  string `json:"binary_path"`
			Description string `json:"description"`
			InputSchema string `json:"input_schema"`
			ForceUpdate bool   `json:"force_update"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		// Check if exists
		existing, err := e.DB.ToolByName(ctx, args.Name)
		if err != nil {
			return ErrJSON(err), nil
		}
		if existing != nil {
			if !args.ForceUpdate {
				return `{"error": "tool already exists, set force_update=true to overwrite"}`, nil
			}
			if err := e.DB.DeleteTool(ctx, args.Name); err != nil {
				return ErrJSON(err), nil
			}
		}
		// Pre-deployment validation: run binary with sample input and require valid JSON stdout
		binaryPath := args.BinaryPath
		if !filepath.IsAbs(binaryPath) && e.WorkspaceDir != "" {
			binaryPath = filepath.Join(e.WorkspaceDir, filepath.Clean(binaryPath))
		}
		stdout, _, code, runErr := ExecuteRegisteredTool(ctx, binaryPath, "{}")
		if runErr != nil {
			return ErrJSON(fmt.Errorf("tool contract test failed: %w", runErr)), nil
		}
		if !ValidateToolOutput(stdout, code) {
			return ErrJSON(fmt.Errorf("tool failed contract test: output was not valid JSON (exit_code=%d)", code)), nil
		}
		id, err := e.DB.InsertTool(ctx, args.Name, args.BinaryPath, args.Description, args.InputSchema)
		if err != nil {
			return ErrJSON(err), nil
		}
		return fmt.Sprintf(`{"id": %d, "status": "registered"}`, id), nil
	case "delete_tool":
		var args struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		if err := e.DB.DeleteTool(ctx, args.Name); err != nil {
			return ErrJSON(err), nil
		}
		return `{"status": "deleted"}`, nil
	case "execute_registered_tool":
		var args struct {
			Name    string            `json:"name"`
			Args    json.RawMessage   `json:"args"`
			EnvVars map[string]string `json:"env_vars"`
		}
		if argsJSON != "" {
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				out, _ := json.Marshal(map[string]string{"error": err.Error()})
				return string(out), nil
			}
		}
		argsStr := "{}"
		if len(args.Args) > 0 {
			argsStr = string(args.Args)
		}
		result, err := ExecuteRegisteredToolByName(ctx, e.DB, e.WorkspaceDir, args.Name, argsStr, args.EnvVars)
		if err != nil {
			return result, err
		}
		// Health recording: validate tool stdout/exit_code and record success or failure (skip when result is lookup error)
		if e.DB != nil && args.Name != "" {
			var out struct {
				Stdout   string `json:"stdout"`
				ExitCode int    `json:"exit_code"`
				Error    string `json:"error"`
			}
			if jsonErr := json.Unmarshal([]byte(result), &out); jsonErr == nil && out.Error == "" {
				if ValidateToolOutput(out.Stdout, out.ExitCode) {
					_ = e.DB.RecordToolSuccess(ctx, args.Name)
				} else {
					errMsg := "invalid output or non-zero exit"
					if out.Stdout != "" && len(out.Stdout) < 200 {
						errMsg = out.Stdout
					}
					_ = e.DB.RecordToolFailure(ctx, args.Name, errMsg)
				}
			}
		}
		return result, nil
	case "install_skill":
		return InstallSkillTool(ctx, e.ConfigDir, argsJSON)
	case "list_skills":
		return ListSkillsTool(ctx, e.ConfigDir)
	case "system_status":
		gatherer := &SystemStatusGatherer{
			DB:          e.DB,
			LogStore:    e.LogStore,
			Gateway:     e.Gateway,
			Compactor:   nil, // Will be set if available
			Client:      e.Client.(*openrouter.Client),
			HealthReg:   e.HealthReg,
			TokenBudget: e.TokenBudget,
		}
		return SystemStatusTool(ctx, gatherer)
	case "read_logs":
		if e.LogStore == nil {
			return `{"error": "log store not configured"}`, nil
		}
		return ReadLogsTool(ctx, e.LogStore, argsJSON)
	case "log_self_modification":
		var args struct {
			FilePaths   []string `json:"file_paths"`
			ChangeType  string   `json:"change_type"`
			Description string   `json:"description"`
			Context     string   `json:"context"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		if len(args.FilePaths) == 0 {
			return ErrJSON(fmt.Errorf("file_paths cannot be empty")), nil
		}
		if args.ChangeType != "core_code" && args.ChangeType != "config" && args.ChangeType != "registered_tool" {
			return ErrJSON(fmt.Errorf("change_type must be core_code, config, or registered_tool")), nil
		}
		if err := e.DB.InsertSelfModification(ctx, args.FilePaths, args.ChangeType, args.Description, args.Context); err != nil {
			return ErrJSON(err), nil
		}
		return `{"status": "logged"}`, nil
	case "read_self_modification_log":
		var args struct {
			Limit int `json:"limit"`
		}
		if argsJSON != "" {
			_ = json.Unmarshal([]byte(argsJSON), &args)
		}
		entries, err := e.DB.ListSelfModifications(ctx, args.Limit)
		if err != nil {
			return ErrJSON(err), nil
		}
		if len(entries) == 0 {
			return `{"entries": [], "message": "No self-modifications recorded yet."}`, nil
		}
		type entry struct {
			ID          int64    `json:"id"`
			CreatedAt   string   `json:"created_at"`
			FilePaths   []string `json:"file_paths"`
			ChangeType  string   `json:"change_type"`
			Description string   `json:"description"`
			Context     string   `json:"context,omitempty"`
		}
		var out []entry
		for _, sm := range entries {
			out = append(out, entry{ID: sm.ID, CreatedAt: sm.CreatedAt, FilePaths: sm.FilePaths, ChangeType: sm.ChangeType, Description: sm.Description, Context: sm.Context})
		}
		b, _ := json.MarshalIndent(map[string]interface{}{"entries": out}, "", "  ")
		return string(b), nil
	case "list_webhook_routes":
		if e.ConfigDir == "" {
			return `{"error": "config dir not configured"}`, nil
		}
		routes, err := store.LoadWebhookRoutes(e.ConfigDir)
		if err != nil {
			return ErrJSON(err), nil
		}
		if routes == nil {
			routes = []store.WebhookRoute{}
		}
		b, _ := json.MarshalIndent(map[string]interface{}{"routes": routes}, "", "  ")
		return string(b), nil
	case "add_webhook_route":
		if e.ConfigDir == "" {
			return ErrJSON(fmt.Errorf("config dir not configured")), nil
		}
		var args struct {
			Path         string `json:"path"`
			ID           string `json:"id"`
			SecretHeader string `json:"secret_header"`
			SecretEnv    string `json:"secret_env"`
			SecretSource string `json:"secret_source"`
			SecretKey    string `json:"secret_key"`
			AuthType     string `json:"auth_type"`
			TargetTool   string `json:"target_tool"`
			TargetArgs   string `json:"target_args"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		if !strings.HasPrefix(args.Path, "/webhook/") || args.Path == "/webhook/talk" {
			return ErrJSON(fmt.Errorf("path must start with /webhook/ and cannot be /webhook/talk")), nil
		}
		if args.AuthType != "header" && args.AuthType != "hmac_sha256" {
			return ErrJSON(fmt.Errorf("auth_type must be header or hmac_sha256")), nil
		}
		routes, _ := store.LoadWebhookRoutes(e.ConfigDir)
		if routes == nil {
			routes = []store.WebhookRoute{}
		}
		for _, r := range routes {
			if r.Path == args.Path || r.ID == args.ID {
				return ErrJSON(fmt.Errorf("route with path %s or id %s already exists", args.Path, args.ID)), nil
			}
		}
		routes = append(routes, store.WebhookRoute{
			Path:         args.Path,
			ID:           args.ID,
			SecretHeader: args.SecretHeader,
			SecretEnv:    args.SecretEnv,
			SecretSource: args.SecretSource,
			SecretKey:    args.SecretKey,
			AuthType:     args.AuthType,
			TargetTool:   args.TargetTool,
			TargetArgs:   args.TargetArgs,
		})
		if err := store.SaveWebhookRoutes(e.ConfigDir, routes); err != nil {
			return ErrJSON(err), nil
		}
		return `{"status": "added", "path": "` + args.Path + `"}`, nil
	case "remove_webhook_route":
		if e.ConfigDir == "" {
			return ErrJSON(fmt.Errorf("config dir not configured")), nil
		}
		var args struct {
			PathOrID string `json:"path_or_id"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		routes, err := store.LoadWebhookRoutes(e.ConfigDir)
		if err != nil {
			return ErrJSON(err), nil
		}
		if routes == nil {
			return ErrJSON(fmt.Errorf("no routes to remove")), nil
		}
		var filtered []store.WebhookRoute
		for _, r := range routes {
			if r.Path != args.PathOrID && r.ID != args.PathOrID {
				filtered = append(filtered, r)
			}
		}
		if len(filtered) == len(routes) {
			return ErrJSON(fmt.Errorf("no route found for %s", args.PathOrID)), nil
		}
		if err := store.SaveWebhookRoutes(e.ConfigDir, filtered); err != nil {
			return ErrJSON(err), nil
		}
		return `{"status": "removed", "path_or_id": "` + args.PathOrID + `"}`, nil
	case "self_reflect":
		if e.Spawner == nil {
			return `{"error": "sub-mind spawner not configured"}`, nil
		}
		// Gather system status first
		gatherer := &SystemStatusGatherer{
			DB:          e.DB,
			LogStore:    e.LogStore,
			Gateway:     e.Gateway,
			HealthReg:   e.HealthReg,
			TokenBudget: e.TokenBudget,
		}
		status, err := gatherer.Gather(ctx)
		if err != nil {
			return ErrJSON(err), nil
		}
		statusJSON, _ := json.MarshalIndent(status, "", "  ")
		userID := ""
		if uid := ctx.Value("user_id"); uid != nil {
			userID = uid.(string)
		}
		result, err := e.Spawner.SpawnSubmind(ctx, userID, "reflection", string(statusJSON), 0)
		if err != nil {
			return ErrJSON(err), nil
		}
		out, _ := json.MarshalIndent(result, "", "  ")
		return string(out), nil
	case "notify_user":
		userID, err := getUserID(ctx)
		if err != nil {
			return ErrJSON(err), nil
		}
		var args struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil || args.Message == "" {
			return ErrJSON(fmt.Errorf("message required")), nil
		}
		if e.Router == nil {
			return ErrJSON(fmt.Errorf("router not configured")), nil
		}
		if err := e.Router.RouteMessage(ctx, userID, args.Message, ""); err != nil {
			return ErrJSON(err), nil
		}
		return `{"status": "sent"}`, nil
	case "spawn_submind":
		if e.Spawner == nil {
			return `{"error": "sub-mind spawner not configured"}`, nil
		}
		userID := ""
		if uid := ctx.Value("user_id"); uid != nil {
			userID = uid.(string)
		}
		var args struct {
			Mode      string `json:"mode"`
			Task      string `json:"task"`
			SessionID int64  `json:"session_id"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		result, err := e.Spawner.SpawnSubmind(ctx, userID, args.Mode, args.Task, args.SessionID)
		if err != nil {
			return ErrJSON(err), nil
		}
		out, _ := json.MarshalIndent(result, "", "  ")
		return string(out), nil
	case "manage_submind":
		if e.SubmindRegistry == nil {
			return `{"error": "sub-mind registry not configured"}`, nil
		}
		var args struct {
			Action       string   `json:"action"`
			Name         string   `json:"name"`
			SystemPrompt string   `json:"system_prompt"`
			AllowedTools []string `json:"allowed_tools"`
			MaxTurns     int      `json:"max_turns"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		switch args.Action {
		case "list":
			list := e.SubmindRegistry.List()
			out, _ := json.MarshalIndent(list, "", "  ")
			return string(out), nil
		case "create", "update":
			// Validate: cannot include blocked tools
			for _, tool := range args.AllowedTools {
				if BlockedTools[tool] {
					return fmt.Sprintf(`{"error": "cannot grant blocked tool: %s"}`, tool), nil
				}
			}
			cfg := core.SubMindConfig{
				Name:         args.Name,
				SystemPrompt: args.SystemPrompt,
				AllowedTools: args.AllowedTools,
				MaxTurns:     args.MaxTurns,
				Protected:    false, // Agent-created are not protected
			}
			if err := e.SubmindRegistry.Add(cfg); err != nil {
				return ErrJSON(err), nil
			}
			return fmt.Sprintf(`{"status": "%sd", "name": "%s"}`, args.Action, args.Name), nil
		case "delete":
			if err := e.SubmindRegistry.Delete(args.Name); err != nil {
				return ErrJSON(err), nil
			}
			return fmt.Sprintf(`{"status": "deleted", "name": "%s"}`, args.Name), nil
		case "list_sessions":
			if e.DB == nil {
				return `{"error": "database not configured"}`, nil
			}
			uid := ""
			if v := ctx.Value("user_id"); v != nil {
				uid = v.(string)
			}
			sessions, err := e.DB.ListSubmindSessions(ctx, uid, "")
			if err != nil {
				return ErrJSON(err), nil
			}
			b, _ := json.MarshalIndent(sessions, "", "  ")
			return string(b), nil
		default:
			return `{"error": "action must be create, update, delete, list, or list_sessions"}`, nil
		}
	case "manage_llm_provider":
		return ManageLLMProviderTool(ctx, e.ConfigDir, argsJSON)
	case "manage_embedding_provider":
		return ManageEmbeddingProviderTool(ctx, e.ConfigDir, argsJSON)
	
	// Nextcloud Tools
	case "request_nextcloud_ocs":
		if e.Config == nil {
			return ErrJSON(fmt.Errorf("config not available")), nil
		}
		var args struct {
			Method   string            `json:"method"`
			Endpoint string            `json:"endpoint"`
			Params   map[string]string `json:"params"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		return nextcloud.RequestNextcloudOCS(e.Config, args.Method, args.Endpoint, args.Params)
	case "list_nextcloud_files":
		if e.Config == nil {
			return ErrJSON(fmt.Errorf("config not available")), nil
		}
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		return nextcloud.ListNextcloudFiles(e.Config, args.Path)
	case "read_nextcloud_file":
		if e.Config == nil {
			return ErrJSON(fmt.Errorf("config not available")), nil
		}
		var args struct {
			Path string `json:"path"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		return nextcloud.ReadNextcloudFile(e.Config, args.Path)
	case "get_secret":
		if e.Config == nil {
			return ErrJSON(fmt.Errorf("config not available")), nil
		}
		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		return nextcloud.GetNextcloudSecret(e.Config, args.Query)
	case "store_secret":
		if e.Config == nil {
			return ErrJSON(fmt.Errorf("config not available")), nil
		}
		var args struct {
			Title    string `json:"title"`
			Password string `json:"password"`
			Login    string `json:"login"`
			URL      string `json:"url"`
			Notes    string `json:"notes"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		return nextcloud.StoreSecret(e.Config, args.Title, args.Password, args.Login, args.URL, args.Notes)
	case "manage_trust":
		var args struct {
			Action string `json:"action"`
			Type   string `json:"type"`
			Value  string `json:"value"`
			Notes  string `json:"notes"`
		}
		if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
			return ErrJSON(err), nil
		}
		switch args.Action {
		case "add":
			if err := e.DB.AddTrustedIdentity(ctx, args.Type, args.Value, args.Notes); err != nil {
				return ErrJSON(err), nil
			}
			return `{"status": "added"}`, nil
		case "remove":
			if err := e.DB.RemoveTrustedIdentity(ctx, args.Type, args.Value); err != nil {
				return ErrJSON(err), nil
			}
			return `{"status": "removed"}`, nil
		case "check":
			trusted, err := e.DB.CheckTrustedIdentity(ctx, args.Type, args.Value)
			if err != nil {
				return ErrJSON(err), nil
			}
			return fmt.Sprintf(`{"trusted": %v}`, trusted), nil
		case "list":
			identities, err := e.DB.ListTrustedIdentities(ctx, args.Type)
			if err != nil {
				return ErrJSON(err), nil
			}
			b, _ := json.Marshal(identities)
			return string(b), nil
		default:
			return ErrJSON(fmt.Errorf("unknown action: %s", args.Action)), nil
		}

	default:
		out, _ := json.Marshal(map[string]string{"error": "unknown tool: " + name})
		return string(out), nil
	}
}

func ErrJSON(err error) string {
	b, _ := json.Marshal(map[string]string{"error": err.Error()})
	return string(b)
}

// parseDuration parses human-readable durations like "1h", "2d", "30m"
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, fmt.Errorf("duration is empty")
	}
	// Check for day suffix (not supported by time.ParseDuration)
	if len(s) > 1 && s[len(s)-1] == 'd' {
		days := s[:len(s)-1]
		var d int
		if _, err := fmt.Sscanf(days, "%d", &d); err != nil {
			return 0, err
		}
		return time.Duration(d) * 24 * time.Hour, nil
	}
	// Standard Go duration parsing for h, m, s
	return time.ParseDuration(s)
}
