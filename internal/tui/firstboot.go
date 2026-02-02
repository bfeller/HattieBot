package tui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hattiebot/hattiebot/internal/agent"
	"github.com/hattiebot/hattiebot/internal/config"
	"github.com/hattiebot/hattiebot/internal/store"
)

func flush() {
	os.Stdout.Sync()
	os.Stderr.Sync()
}

// RunFirstBoot runs a simple first-boot setup: prompts on stdout, input from stdin.
// Asks: OpenRouter key, model, then (once connected) bot name, who it's talking to, purpose.
func RunFirstBoot(cfg *config.Config) error {
	scan := bufio.NewScanner(os.Stdin)

	fmt.Fprintln(os.Stderr, "HattieBot: first-boot setup")
	flush()

	fmt.Println("HattieBot — first run setup")
	fmt.Println()
	flush()

	fmt.Print("OpenRouter API key: ")
	flush()
	if !scan.Scan() {
		return scan.Err()
	}
	apiKey := strings.TrimSpace(scan.Text())
	if apiKey == "" {
		return fmt.Errorf("API key is required")
	}

	fmt.Print("Model (e.g. moonshotai/kimi-k2.5): ")
	flush()
	if !scan.Scan() {
		return scan.Err()
	}
	model := strings.TrimSpace(scan.Text())
	if model == "" {
		return fmt.Errorf("model is required")
	}

	fmt.Println()
	fmt.Print("Workspace Directory (default: ~/.hattiebot): ")
	flush()
	scan.Scan()
	workspaceDir := strings.TrimSpace(scan.Text())
	if workspaceDir == "" {
		home, _ := os.UserHomeDir()
		workspaceDir = filepath.Join(home, ".hattiebot")
	}

	fmt.Println()
	fmt.Println("WARNING: HattieBot is an autonomous agent capable of executing commands and creating files.")
	fmt.Println("It is designed to be helpful, but you are responsible for its actions.")
	fmt.Print("Do you accept the risks associated with running this agent? (yes/no): ")
	flush()
	if !scan.Scan() {
		return scan.Err()
	}
	if strings.ToLower(strings.TrimSpace(scan.Text())) != "yes" {
		return fmt.Errorf("risk not accepted; exiting")
	}
	riskAccepted := true

	fmt.Println()
	fmt.Println("Tell me about the bot:")
	flush()

	fmt.Print("What is the bot's name? ")
	flush()
	if !scan.Scan() {
		return scan.Err()
	}
	name := strings.TrimSpace(scan.Text())
	if name == "" {
		return fmt.Errorf("bot name is required")
	}

	fmt.Print("Who is it talking to? ")
	flush()
	if !scan.Scan() {
		return scan.Err()
	}
	audience := strings.TrimSpace(scan.Text())
	if audience == "" {
		return fmt.Errorf("who the bot is talking to is required")
	}

	fmt.Println("What is its purpose? (one or more lines, empty line when done)")
	flush()
	var purposeLines []string
	for scan.Scan() {
		line := scan.Text()
		if line == "" {
			break
		}
		purposeLines = append(purposeLines, line)
	}
	if err := scan.Err(); err != nil {
		return err
	}
	purpose := strings.TrimSpace(strings.Join(purposeLines, "\n"))
	if purpose == "" {
		return fmt.Errorf("purpose is required")
	}

	fmt.Println()
	fmt.Print("Primary/Admin User ID (who owns this bot?) [admin]: ")
	flush()
	scan.Scan()
	adminID := strings.TrimSpace(scan.Text())
	if adminID == "" {
		adminID = "admin"
	}

	fmt.Println("Saving config and generating SOUL.md...")
	if err := store.SaveConfigFile(cfg.ConfigDir, &store.ConfigFile{
		OpenRouterAPIKey: apiKey,
		Model:            model,
		AgentName:        name,
		WorkspaceDir:     workspaceDir,
		RiskAccepted:     riskAccepted,
		AdminUserID:      adminID,
	}); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	
	// Generate SOUL.md directly (no LLM rewrite needed - user provides the content)
	if err := agent.WriteSoul(cfg.ConfigDir, name, audience, purpose); err != nil {
		return fmt.Errorf("write SOUL.md: %w", err)
	}

	fmt.Println("Done. Config and SOUL.md saved to", cfg.ConfigDir)
	fmt.Println("Starting chat — Enter to send, Ctrl+C to exit.")
	return nil
}
