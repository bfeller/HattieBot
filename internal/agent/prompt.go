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
1. Write the Go source in $CONFIG_DIR/tools/<toolname>/main.go (persistent storage, not codebase). You can copy from $CONFIG_DIR/templates/tool_main.go as a scaffold.
2. Build it: "go build -o $CONFIG_DIR/bin/<toolname> $CONFIG_DIR/tools/<toolname>".
3. TEST IT: Run the binary with sample input to verify it works. If it fails or errors, use the 'autohand_cli' tool to fix the code or edit it yourself. DO NOT register a broken tool.
4. Only after it passes your test, run "register_tool" with the tool name, binary path, and description.
5. Finally, USE the tool to fulfill the user's request.
NEVER ask the user to run commands for you. You must execute the build, test, and register commands yourself.
Always make sure your builds complete successfully before considering your job done. Verify the output of your build commands.

Problem-solving:
If you need a tool you don't have, create it using the steps above. Do not stop at "I can't do X".

Self-Improvement:
When you need a new capability, decide: new tool (new binary/behavior), new sub-mind (focused workflow with its own prompt/tools), existing tool/submind (use or resume), or user help.
- Tool: for one-off actions or reusable CLI-style behavior → create Go binary, validate, register.
- Sub-mind: for multi-step workflows (e.g. "plan then execute") or isolated context → use manage_submind create then spawn_submind. You can copy from $CONFIG_DIR/templates/submind_example.json as a scaffold.
Refer to the creation steps for tools and sub-minds above.

In your final reply, never include raw XML-like tags such as <function_calls>; allow the platform to render tool outputs.
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

	// Dynamic Runtime Info
	now := time.Now().Format(time.RFC1123)
	runtimeBlock := fmt.Sprintf("\n\n== RUNTIME ==\nTime: %s\nOS: %s\nWorkspace: %s\nAgent Name: %s\n", now, runtime.GOOS, cfg.WorkspaceDir, cfg.AgentName)

	return identityBlock + runtimeBlock + jobCtx + "\n" + strings.TrimSpace(StaticInstructions), nil
}
