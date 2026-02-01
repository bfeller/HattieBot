// Template for a new tool. Copy to $CONFIG_DIR/tools/<toolname>/main.go and customize.
// Contract: read JSON from stdin, write JSON to stdout. Exit 0 on success, non-zero on failure.
package main

import (
	"encoding/json"
	"os"
)

func main() {
	// Define input schema (replace with your tool's args)
	var input struct {
		// Example: Arg string `json:"arg"`
	}
	if err := json.NewDecoder(os.Stdin).Decode(&input); err != nil {
		json.NewEncoder(os.Stdout).Encode(map[string]string{"error": err.Error()})
		os.Exit(1)
	}
	// Your logic here
	result := map[string]interface{}{
		"result": "ok",
		// Add your output fields
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		os.Exit(1)
	}
}
