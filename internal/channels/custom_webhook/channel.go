package custom_webhook

import (
	"context"
	"fmt"

	"github.com/hattiebot/hattiebot/internal/gateway"
)

// Channel forwards webhook replies to the admin via the default channel.
type Channel struct {
	Gateway       *gateway.Gateway
	DefaultChannel string
	AdminUserID   string
}

// New creates a channel that forwards replies to the admin.
func New(gw *gateway.Gateway, defaultChannel, adminUserID string) *Channel {
	return &Channel{
		Gateway:        gw,
		DefaultChannel: defaultChannel,
		AdminUserID:    adminUserID,
	}
}

func (c *Channel) Name() string {
	return "custom_webhook"
}

func (c *Channel) Start(ctx context.Context, ingress chan<- gateway.Message) error {
	return nil
}

func (c *Channel) Send(msg gateway.Message) error {
	if c.Gateway == nil || c.DefaultChannel == "" || c.AdminUserID == "" {
		return nil
	}
	return c.Gateway.Broadcast(context.Background(), c.DefaultChannel, c.AdminUserID, msg.Content, "")
}

func (c *Channel) SendProactive(userID, content string) error {
	return fmt.Errorf("custom_webhook: SendProactive not supported")
}
