package tui

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// RunChat runs a simple REPL: prompt "You: ", read a line, Enter sends to the agent, print response.
// Ctrl+C to exit. No TUI library. The agent is independent; this is just the direct-access console.
func RunChat(onSubmit func(string) (string, error)) error {
	scan := bufio.NewScanner(os.Stdin)
	fmt.Fprintln(os.Stderr, "HattieBot: chat mode")
	flush()
	fmt.Println("HattieBot — chat (Enter to send, Ctrl+C to exit)")
	fmt.Println()
	flush()

	for {
		fmt.Print("You: ")
		flush()
		if !scan.Scan() {
			return scan.Err()
		}
		line := strings.TrimSpace(scan.Text())
		if line == "" {
			continue
		}

		reply, err := onSubmit(line)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}
		fmt.Println("HattieBot:", reply)
		fmt.Println()
	}
}
