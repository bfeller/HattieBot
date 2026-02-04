# HattieBot Architecture

HattieBot is a self-improving, persistent agentic system designed to run indefinitely in a containerized environment.

## 1. Runtime Environment

- **Container**: The agent runs in Docker. The host is abstracted away.
- **Persistence**: 
  - **Config Dir** (`/data` or `~/.hattiebot`): Contains the DB (`hattiebot.db`), `config.json`, `system_purpose.txt`, `providers/` (LLM templates), and `subminds.json`.
  - **Workspace**: The working directory for file operations and code generation.
  - **Log Store**: Structured logs are stored in the DB for self-reflection.

## 2. Core Components

### A. The Agent Loop
The central brain (`internal/agent/loop.go`) runs a continuous Read-Evaluate-Think-Act loop:
1. **Observe**: Read new messages from Gateway (TUI, Slack, etc.).
2. **Context**: Select relevant history + Epic Context (Active Job) + Active Sub-mind state.
3. **Think**: Call LLM (via Dynamic Router) to generate a response or tool call.
4. **Act**: Execute tool or complete turn.
5. **Persist**: Save state (messages, job status, sub-mind checkpoints) to DB.

### B. Memory & State
- **Episodic Memory**: Recent conversation history (sliding window).
- **Epic Memory (Jobs)**: Long-running tasks (`jobs` table). The agent always knows its active "Job" (e.g., "Refactor API").
- **Semantic Memory**: `memory_chunks` table (sqlite-vec) for long-term recall (`memorize`, `recall_memories`).
- **User Preference**: Key-Value facts about the user (`facts` table).
- **Sub-Mind Sessions**: Checkpointed sessions for focused tasks (`submind_sessions`).

### C. Dynamic LLM Router
The agent is not tied to a single provider.
- **Router**: `internal/llmrouter` selects the best model for the task.
- **Provider Registry**: Loads templates from `$CONFIG_DIR/providers/*.json`.
- **Logic**: 
    - Usage: `manage_llm_provider` tool.
    - Supports: OpenRouter, Ollama, vLLM, Anthropic, etc.

### D. Sub-Mind Orchestration
For complex tasks, the agent spawns "Sub-Minds" - specialized loops with restricted tools and specific prompts.
- **Registry**: Loaded from `$CONFIG_DIR/subminds.json`.
- **Persistence**: Sessions are saved to DB. If the system restarts, sub-minds can be resumed.
- **Usage**: `spawn_submind`, `manage_submind`.

## 3. Directory Layout

- **$CONFIG_DIR/**
  - `hattiebot.db`: SQLite database (Jobs, Logs, Tools, Memories).
  - `providers/`: JSON templates for LLM providers (e.g. `ollama.json`).
  - `subminds.json`: Definitions of sub-mind modes.
  - `webhook_routes.json`: Configurable webhook endpoints (path, id, secret_header, secret_env, auth_type).
  - `tools/`: Source code for agent-created tools.
  - `bin/`: Compiled binaries for agent-created tools.

## 4. Built-in Tools

The agent has a powerful set of native capabilities:

### Core & Filesystem
- `run_terminal_cmd`: Execute shell commands (sandboxed).
- `read_file`, `write_file`: Manage file content.
- `list_dir`: Explore workspace.
- `read_architecture`: Read these docs.
- `read_logs`: Inspect system logs for debugging.

### Task Management (Epic Memory)
- `manage_job`: Create/Update/List long-running tasks. Supports blocking tasks and snoozing.
- `manage_schedule`: Schedule reminders, direct tool execution, or agent prompts. Action types: `remind` (message user), `execute_tool` (run tool directly), `agent_prompt` (agent reasons and acts; use `autonomous=true` for background tasks).

### Sub-Minds & Self-Improvement
- `spawn_submind`: Start a focused session (coding, planning, reflection).
- `manage_submind`: Create new sub-mind modes.
- `self_reflect`: Analyze system health.

### Memory & Knowledge
- `manage_user_preference`: Remember facts about the user.
- `memorize` / `recall_memories`: Vector-based long-term memory.

### System & Extensions
- `manage_llm_provider`: Configure new LLM backends.
- `install_skill`: Install external packages (go, brew, npm).
- `register_tool`: Register a new binary as a tool.
- `execute_registered_tool`: Run a registered binary.
- `system_status`: Check component health.

### Admin
- `list_users`, `approve_user`, `block_user`: User management.
- `manage_trust`: Manage Circle of Trust (trusted emails, phone numbers, API keys).

### Proactive Notification
- `notify_user`: Send a message to the user. Used by autonomous tasks when something needs attention.

### Configurable Webhooks
- `list_webhook_routes`: List registered webhook endpoints.
   - `add_webhook_route`: Add a webhook endpoint (path, id, secret_header, secret_env, secret_source, secret_key, auth_type, target_tool).
   - `remove_webhook_route`: Remove a webhook route by path or id.

## 5. Extension Points

1. **New Tools**: The agent can write Go code, build it, and register it via `register_tool`. These persist in `$CONFIG_DIR/tools`.
2. **New Sub-Minds**: The agent can define new workflow modes via `manage_submind`.
4. **Configurable Webhooks**: The agent can add webhook endpoints for external services (GitHub, Stripe, etc.) via `add_webhook_route`. Routes are stored in `$CONFIG_DIR/webhook_routes.json`.
   - **Security**: Webhooks MUST target a specific tool (`target_tool`). They cannot route directly to the chat stream.
   - **Secrets**: Can be read from env or Nextcloud Passwords app.
   - **Auth**: Supports `header` (exact match) and `hmac_sha256`.

5. **Trust Management**: The agent maintains a table of `trusted_identities`. Tools receiving external input (e.g., email hooks, SMS) should verify the source against this valid list using `manage_trust` (check action) before taking sensitive actions. 

6. **Autonomous Scheduled Tasks**: The scheduler supports `agent_prompt` with `autonomous=true`. The agent runs its full loop without user interaction; it must call `notify_user` only when something needs attention. Otherwise the task completes silently.
