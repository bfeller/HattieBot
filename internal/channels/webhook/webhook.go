package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hattiebot/hattiebot/internal/gateway"
)

type Channel struct {
	URL string
}

func New(url string) *Channel {
	return &Channel{URL: url}
}

func (c *Channel) Name() string {
	return "webhook"
}

func (c *Channel) Start(ctx context.Context, ingress chan<- gateway.Message) error {
	// Webhook is outbound-only for now.
	return nil
}

func (c *Channel) Send(msg gateway.Message) error {
	payload, err := json.Marshal(map[string]string{
		"content": msg.Content,
		"sender":  msg.SenderID,
	})
	if err != nil {
		return err
	}

	resp, err := http.Post(c.URL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook failed with status: %d", resp.StatusCode)
	}
	return nil
}

func (c *Channel) SendProactive(userID, content string) error {
	// For webhook, proactive is same as regular send
	return c.Send(gateway.Message{SenderID: "system", Content: content})
}
