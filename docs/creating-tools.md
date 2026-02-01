# Creating New Tools (Go)

This guide explains how to create a new tool that the agent can call via `execute_registered_tool`. All agent-created tools should follow this contract so the core can run and test them consistently.

## Contract

1. **Language**: Go.
2. **Single package**: One `main` package per tool; no imports from other agent-written packages. Use only the standard library or well-known deps (e.g. `encoding/json`).
3. **Input**: JSON on stdin. Example: `{"path": "/tmp/foo.txt"}`.
4. **Output**: JSON on stdout. Example: `{"content": "..."}` or `{"error": "..."}`. Exit code 0 on success, non-zero on failure.
5. **Location**: Source in `$CONFIG_DIR/tools/<toolname>/main.go`. Binary: `$CONFIG_DIR/bin/<toolname>`.

## Steps

1. **Create the Go program**
   - Directory: `$CONFIG_DIR/tools/<toolname>/`
   - File: `main.go` with `package main`, `func main()`.
   - Read stdin as JSON; unmarshal into a struct; perform the action; marshal result to stdout; `os.Exit(0)` or `os.Exit(1)`.

2. **Build**
   ```bash
   go build -o $CONFIG_DIR/bin/<toolname> $CONFIG_DIR/tools/<toolname>
   ```
   If it fails, fix compile errors (e.g. with Autohand Code CLI or edits) and repeat.

3. **Functional test**
   - Run the binary with sample input: `echo '{"arg": "value"}' | $CONFIG_DIR/bin/<toolname>`.
   - Assert expected output or exit code (e.g. shell script or `go test` that exec's the binary).

4. **Register**
   - Run the `register_tool` tool function directly (preferred) or the CLI helper.
     ```bash
     register_tool(name="my_tool", binary_path="$CONFIG_DIR/bin/my_tool", description="...")
     ```

## Example

```go
// $CONFIG_DIR/tools/echo/main.go
package main

import (
	"encoding/json"
	"os"
)

func main() {
	var input struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
		os.Exit(1)
	}
	json.NewEncoder(os.Stdout).Encode(map[string]string{"reply": input.Message})
}
```

Build: `go build -o $CONFIG_DIR/bin/echo $CONFIG_DIR/tools/echo`  
Test: `echo '{"message":"hi"}' | $CONFIG_DIR/bin/echo`  
Register: Use `register_tool` tool.

## Structured output

All tools (including new Go tools) should return **parseable output** (JSON) so the agent can reason over success/error and fields without parsing free text. Document this in your tool's description.

## Checkpointing / rollback

When creating or modifying a tool, keep a copy of the previous binary or source (e.g. `tools/<toolname>/v1/` or a timestamped backup). To rollback, replace the current binary with the backup.
