// register-tool inserts a tool into tools_registry. Used for testing and for the agent to register new tools via run_terminal_cmd.
// Usage: HATTIEBOT_CONFIG_DIR=/data register-tool <name> <binary_path> [description]
// Or: register-tool <name> <binary_path> [description]  (uses default config dir)
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hattiebot/hattiebot/internal/config"
	"github.com/hattiebot/hattiebot/internal/store"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: register-tool <name> <binary_path> [description]\n")
		os.Exit(1)
	}
	name := os.Args[1]
	binaryPath := os.Args[2]
	description := ""
	if len(os.Args) > 3 {
		description = os.Args[3]
	}
	cfg := config.New("")
	ctx := context.Background()
	db, err := store.Open(ctx, cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()
	_, err = db.InsertTool(ctx, name, binaryPath, description, "{}")
	if err != nil {
		fmt.Fprintf(os.Stderr, "insert tool: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("registered", name)
}
