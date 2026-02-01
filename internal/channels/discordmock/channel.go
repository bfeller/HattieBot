package discordmock

import (
	"context"
	"fmt"
	"time"

	"github.com/hattiebot/hattiebot/internal/gateway"
)

// Channel implements a mock channel for verification
type Channel struct {
}

func New() *Channel {
	return &Channel{}
}

func (c *Channel) Name() string {
	return "discord_mock"
}

func (c *Channel) Start(ctx context.Context, ingress chan<- gateway.Message) error {
	fmt.Println("DiscordMock: Starting (simulated)")
	
	// Simulate receiving a message after a delay
	go func() {
		time.Sleep(2 * time.Second)
		ingress <- gateway.Message{
			SenderID: "simulated_user",
			Content:  "Hello from simulated Discord!",
			Channel:  c.Name(),
		}
	}()

	<-ctx.Done()
	return nil
}

func (c *Channel) Send(msg gateway.Message) error {
	fmt.Printf("[DiscordMock] Sending: %s\n", msg.Content)
	return nil
}

func (c *Channel) SendProactive(userID, content string) error {
	fmt.Printf("[DiscordMock] Proactive to %s: %s\n", userID, content)
	return nil
}
