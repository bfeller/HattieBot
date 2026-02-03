package agent

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/hattiebot/hattiebot/internal/config"
	"github.com/hattiebot/hattiebot/internal/store"
)

// StaticInstructions are prepended to the system prompt (safety, tool use, architecture reference).
const StaticInstructions = `
You have access to tools. Use them when the user asks to list files, read files, run commands, create tools, or similar; do not output shell commands as a code block for the user to run—invoke list_dir, read_file, run_terminal_cmd, etc. within the conversation. Prefer structured tool output (JSON).
Do not execute destructive commands (rm -rf /) without user confirmation.
You run inside a container; the host is not directly accessible. Use mounted paths for persistence.

Create Tools Autonomously:
When the user asks for a new capability (e.g. "make a tool that does X"):
1. USE 'autohand_cli' to write the Go source in $CONFIG_DIR/tools/<toolname>/main.go (or use the Config Dir path from RUNTIME). Provide a detailed instruction to it (e.g. "Write a Go tool that..."). It is a specialized coding agent; delegate the coding to it.
2. Build it: "go build -o $CONFIG_DIR/bin/<toolname> $CONFIG_DIR/tools/<toolname>" (use the Config Dir from RUNTIME if $CONFIG_DIR is empty).
3. TEST IT: Run the binary with sample input to verify it works. If it fails or errors, DELETE the source file ($CONFIG_DIR/tools/<toolname>/main.go) and use the 'autohand_cli' tool again to write fixed code from scratch. This prevents stale code from persisting.
4. Only after it passes your test, run "register_tool" with the tool name, binary path, and description.
5. Finally, USE the tool to fulfill the user's request.
NEVER ask the user to run commands for you. You must execute the build, test, and register commands yourself.
Always make sure your builds complete successfully before considering your job done. Verify the output of your build commands.

Problem-solving:
If you need a tool you don't have, create it using the steps above. Do not stop at "I can't do X".

Multi-step diagnosis: When investigating an issue (e.g. "why didn't my reminder send?"), run ALL diagnostic tools in the SAME turn before replying. Do NOT output text like "Let me dig deeper" or "I'll check the logs" and then stop—emit the tool calls (read_logs, self_reflect, system_status) immediately. Only after you have the results should you summarize for the user.

Self-Improvement:
When you need a new capability, decide: new tool (new binary/behavior), new sub-mind (focused workflow with its own prompt/tools), existing tool/submind (use or resume), or user help.
- Tool: for one-off actions or reusable CLI-style behavior → create Go binary, validate, register.
- Sub-mind: for multi-step workflows (e.g. "plan then execute") or isolated context → use manage_submind create then spawn_submind. You can copy from $CONFIG_DIR/templates/submind_example.json as a scaffold.

Context Management:
You can manage your own context by loading and unloading documents.
- If you need specific knowledge (e.g. how to write tools, project architecture) that isn't in your immediate context, use 'manage_context_doc' with action="list" to see available documents, then action="toggle" active=true to load one.
- "Prime" yourself with this knowledge, complete the task, and then UNLOAD it (action="toggle" active=false) to keep your context clean.
- If you learn something valuable that you might need later (e.g. a complex procedure), create a new context document for it.

In your final reply, never include raw XML-like tags such as <function_calls>; allow the platform to render tool outputs.

Critical: If you intend to run more tools (e.g. read_logs, self_reflect, system_status), you MUST emit those tool calls in the same response. Never output text promising to "dig deeper" or "check the logs" and then stop—the loop will end and the user will not get the diagnosis. Run all needed tools first, then summarize.

Status updates: You CAN return both text and tool calls in a single response. When you do, the user sees your text immediately while tools run. Use this sparingly: (a) when you make a major decision about your approach (e.g. "Switching to plan B—checking the scheduler logs"), or (b) when processing has taken several tool rounds and the user has had no feedback. Do NOT include a status update for every tool call—only when it would help the user understand progress.

Self-modification log: When you modify core code (internal/*, cmd/*, Dockerfile, etc.) or config that lives in the workspace, call log_self_modification immediately after. Include file paths, change_type (core_code or config), and a brief description of what you changed and why. This log survives rebuilds—if a software update wipes your changes, you or the user can reference it via read_self_modification_log to re-apply them. Do NOT log changes to $CONFIG_DIR/tools (registered tools)—those persist in the data volume.

Custom webhooks: You can add webhook endpoints for external services (GitHub, Stripe, etc.) without editing the main codebase. Use add_webhook_route with path (e.g. /webhook/github), id, secret_header, secret_env, and auth_type (header or hmac_sha256). The config lives in $CONFIG_DIR/webhook_routes.json and survives rebuilds. Use list_webhook_routes to see current routes. After adding, the endpoint is active immediately—no restart needed. Replies to webhook messages are forwarded to the admin.
`

// BuildSystemPrompt builds the system prompt using SOUL.md as the primary identity source.
func BuildSystemPrompt(ctx context.Context, db *store.DB, cfg *config.Config, userID string) (string, error) {
	// Load SOUL.md (Identity) - this is now the primary identity source
	soul, err := LoadIdentity(cfg.ConfigDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load SOUL.md: %v\n", err)
	}
	identityBlock := FormatIdentityPrompt(soul)

	// Inject Active Job Context
	job, _ := db.GetActiveJob(ctx, userID)
	jobCtx := ""
	if job != nil {
		jobCtx = fmt.Sprintf("\n\n== EPIC CONTEXT / ACTIVE JOB ==\nTitle: %s\nStatus: %s\nDescription: %s\n", job.Title, job.Status, job.Description)
		if job.Status == "blocked" {
			jobCtx += fmt.Sprintf("BLOCKED REASON: %s\n[ACTION REQUIRED]: This job is BLOCKED. You must prioritize resolving this block or asking the user for help.\n", job.BlockedReason)
		}
		jobCtx += "===============================\n"
	}


	// Inject Broken Tools (repair queue)
	broken, _ := db.ListBrokenTools(ctx)
	if len(broken) > 0 {
		jobCtx += "\n\n== BROKEN TOOLS ==\n"
		for _, t := range broken {
			jobCtx += fmt.Sprintf("- %s: %s\n", t.Name, t.LastError)
		}
		jobCtx += "[ACTION]: Consider repairing or deprecating. Use spawn_submind with mode tool_creation and the tool name and last_error.\n===============================\n"
	}
	
	// Inject Registered Tools (so LLM knows how to use them via execute_registered_tool)
	// We allow injection of all tools since the total count is usually small. If it grows large, we might summarize.
	regTools, _ := db.AllTools(ctx)
	if len(regTools) > 0 {
		jobCtx += "\n\n== REGISTERED TOOLS ==\nTo use these, call 'execute_registered_tool' with {\"name\": \"<name>\", \"args\": { ... }}\n"
		for _, t := range regTools {
			jobCtx += fmt.Sprintf("- %s: %s\n  Schema: %s\n", t.Name, t.Description, t.InputSchema)
		}
		jobCtx += "===============================\n"
	}

	// Inject Context Documents (Active: full content; Inactive: summary list)
	allDocs, _ := db.ListContextDocs(ctx)
	activeDocs := ""
	inactiveDocs := ""
	for _, doc := range allDocs {
		if doc.IsActive {
			activeDocs += fmt.Sprintf("### %s\n%s\n\n", doc.Title, doc.Content)
		} else {
			inactiveDocs += fmt.Sprintf("- %s: %s\n", doc.Title, doc.Description)
		}
	}

	if activeDocs != "" {
		jobCtx += "\n\n== ACTIVE CONTEXT DOCUMENTS ==\n" + activeDocs + "===============================\n"
	}
	if inactiveDocs != "" {
		jobCtx += "\n\n== AVAILABLE CONTEXT DOCUMENTS ==\n(Load these using 'manage_context_doc' with action='activate' ONLY if needed for current task)\n" + inactiveDocs + "===============================\n"
	}

	// Dynamic Runtime Info (ConfigDir is critical for tool creation—use this path in commands)
	now := time.Now().Format(time.RFC1123)
	runtimeBlock := fmt.Sprintf("\n\n== RUNTIME ==\nTime: %s\nOS: %s\nWorkspace: %s\nConfig Dir: %s\nAgent Name: %s\n", now, runtime.GOOS, cfg.WorkspaceDir, cfg.ConfigDir, cfg.AgentName)

	return identityBlock + runtimeBlock + jobCtx + "\n" + strings.TrimSpace(StaticInstructions), nil
}
