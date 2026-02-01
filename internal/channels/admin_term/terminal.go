package adminterm

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hattiebot/hattiebot/internal/gateway"
)

// TerminalChannel implements a simple stdin/stdout channel for the admin
type TerminalChannel struct {
}

func New() *TerminalChannel {
	return &TerminalChannel{}
}

func (t *TerminalChannel) Name() string {
	return "admin_term"
}

func (t *TerminalChannel) Start(ctx context.Context, ingress chan<- gateway.Message) error {
	fmt.Println("HattieBot — Admin Terminal (Enter to send, Ctrl+C to exit)")
	fmt.Println()

	// Use a scanner in a goroutine so we can respect ctx.Done()
	// Note: os.Stdin read is not easily interruptible in Go without closing stdin, which we don't want to do globally potentially.
	// But since this is the main process typically, it's fine.
	
	scanner := bufio.NewScanner(os.Stdin)
	
	go func() {
		for {
			fmt.Print("Admin: ")
			if !scanner.Scan() {
				// EOF or error
				return
			}
			text := strings.TrimSpace(scanner.Text())
			if text == "" {
				continue
			}

			// Send to gateway
			ingress <- gateway.Message{
				SenderID: "admin",
				Content:  text,
				Channel:  t.Name(),
				ThreadID: "terminal:console",
			}
		}
	}()

	<-ctx.Done()
	return nil
}

func (t *TerminalChannel) Send(msg gateway.Message) error {
	// Just print to stdout
	// Clear current line if possible to avoid prompt messing up, but for now simple print
	fmt.Printf("\r\033[K") // Clear line
	fmt.Printf("HattieBot: %s\n\n", msg.Content)
	fmt.Print("Admin: ") // Restore prompt
	return nil
}

func (t *TerminalChannel) SendProactive(userID, content string) error {
	// Terminal is single user (mostly), so just print warning
	fmt.Printf("\r\033[K") // Clear line
	fmt.Printf("\n[PROACTIVE ALERT] To %s: %s\n\n", userID, content)
	fmt.Print("Admin: ") // Restore prompt
	return nil
}
