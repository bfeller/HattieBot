# HattieBot Tool Creation Guide

To extend your capabilities, you can create new tools.

## 1. Tool Contract
- **Language**: Go
- **Input**: JSON object via Stdin.
- **Output**: JSON object via Stdout.
- **Exit Code**: 0 for success, non-zero for failure.

## 2. Directory Structure
- Source code: $CONFIG_DIR/tools/<tool_name>/main.go
- Binary: $CONFIG_DIR/bin/<tool_name>

## 3. Workflow
1. **Plan**: Decide on the tool name and input schema.
2. **Code**: Use 'autohand_cli' to write the code.
   - Prompt: "Write a Go tool named <name> that takes <args> and does <action>..."
3. **Build**: Run 'go build -o $CONFIG_DIR/bin/<name> $CONFIG_DIR/tools/<name>'
4. **Test**: Run the binary manually with sample JSON input.
   - 'echo \'{"arg": "val"}\' | $CONFIG_DIR/bin/<name>'
5. **Register**: Use 'register_tool' with the name, path, and description.

## 4. Code Template (MUST USE)
Use this structure to ensure the tool reads JSON from Stdin correctly:

```go
package main

import (
    "encoding/json"
    "fmt"
    "io"
    "os"
)

func main() {
    // Read all of Stdin
    inputData, err := io.ReadAll(os.Stdin)
    if err != nil {
        fail(err)
    }

    var args struct {
        // Define fields here matching the input tool arguments
        ExampleArg string `json:"example_arg"` 
    }
    if len(inputData) > 0 {
        if err := json.Unmarshal(inputData, &args); err != nil {
            // It's okay to start with empty args if JSON is invalid/empty, depending on tool
             // or fail(fmt.Errorf("invalid json: %v", err))
        }
    }

    // ... Do work ...
    result := map[string]string{"status": "ok", "arg_received": args.ExampleArg}
    
    // Output JSON to Stdout
    json.NewEncoder(os.Stdout).Encode(result)
}

func fail(err error) {
    json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
    os.Exit(1)
}
```

## 5. Example
Input: `{"url": "https://example.com"}`
Output: `{"content": "..."}`
Error Output: `{"error": "failed to fetch"}`
