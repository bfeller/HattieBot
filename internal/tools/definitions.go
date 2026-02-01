package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hattiebot/hattiebot/internal/config"
	"github.com/hattiebot/hattiebot/internal/core"
	"github.com/hattiebot/hattiebot/internal/gateway"
	"github.com/hattiebot/hattiebot/internal/health"
	"github.com/hattiebot/hattiebot/internal/openrouter"
	"github.com/hattiebot/hattiebot/internal/registry"
	"github.com/hattiebot/hattiebot/internal/store"
	"github.com/hattiebot/hattiebot/internal/tools/builtin"
)

func init() {
	registry.RegisterExecutor("default", func(cfg *config.Config, db *store.DB, client core.LLMClient) (core.ToolExecutor, error) {
		return &Executor{
			WorkspaceDir: cfg.WorkspaceDir,
			DocsDir:      cfg.DocsDir,
			ConfigDir:    cfg.ConfigDir,
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
						"command":  map[string]string{"type": "string", "description": "Shell command to run"},
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
						"command":  map[string]string{"type": "string", "description": "Command to run"},
						"work_dir": map[string]string{"type": "string", "description": "Working directory inside container"},
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
				Description: "Create, list, or delete scheduled reminders and recurring tasks. Examples: 'remind me tomorrow to take pills', 'check email every day'.",
				Parameters: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"action":         map[string]interface{}{"type": "string", "enum": []string{"create", "list", "delete", "pause"}, "description": "Action to perform"},
						"description":    map[string]string{"type": "string", "description": "What to remind or do"},
						"action_type":    map[string]interface{}{"type": "string", "enum": []string{"remind", "execute_tool"}, "description": "Type: remind (message) or execute_tool"},
						"schedule_type":  map[string]interface{}{"type": "string", "enum": []string{"once", "daily", "weekly", "hourly"}, "description": "Frequency"},
						"run_at":         map[string]string{"type": "string", "description": "ISO datetime for 'once', or time like '09:00' for recurring"},
						"id":             map[string]interface{}{"type": "integer", "description": "Plan ID (for delete/pause)"},
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
	DB              *store.DB
	Client          core.LLMClient
	Embedder        core.EmbeddingClient // When set, memorize/recall use this; else fall back to Client.Embed
	Gateway         *gateway.Gateway
	LogStore        *store.LogStore
	HealthReg       *health.Registry
	TokenBudget     int
	Spawner         core.SubmindSpawner  // For spawning sub-minds
	SubmindRegistry core.SubmindRegistry // For managing sub-minds
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
		return AutohandCLITool(ctx, argsJSON)

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
			Image    string `json:"image"`
			Command  string `json:"command"`
			WorkDir  string `json:"work_dir"`
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
		
		cmdArgs := []string{"run", "--rm", "-i", "-v", "/workspace:/workspace", "-w", args.WorkDir, args.Image, "/bin/sh", "-c", args.Command}
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
			Action       string `json:"action"`       // create, list, delete, pause
			Description  string `json:"description"`  // what to remind
			ActionType   string `json:"action_type"`  // remind, execute_tool
			ScheduleType string `json:"schedule_type"` // once, daily, weekly, hourly
			RunAt        string `json:"run_at"`       // ISO datetime or time
			ID           int64  `json:"id"`
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
			id, err := e.DB.CreatePlan(ctx, userID, args.Description, actionType, "", args.ScheduleType, args.RunAt, nextRun)
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
			Name string          `json:"name"`
			Args json.RawMessage `json:"args"`
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
		result, err := ExecuteRegisteredToolByName(ctx, e.DB, e.WorkspaceDir, args.Name, argsStr)
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
