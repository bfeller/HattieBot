package channels

import (
	"context"

	"github.com/hattiebot/hattiebot/internal/gateway"
)

// Channel represents a communication medium for the agent.
type Channel interface {
	// Name returns the unique name of the channel (e.g. "admin_term", "slack_webhook").
	Name() string

	// Start initializes the channel and starts listening for incoming messages (if applicable).
	// It pushes messages to the ingress channel.
	Start(ctx context.Context, ingress chan<- gateway.Message) error

	// Send transmits a message to the channel.
	Send(msg gateway.Message) error
}
