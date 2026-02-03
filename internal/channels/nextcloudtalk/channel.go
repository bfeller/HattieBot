package nextcloudtalk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hattiebot/hattiebot/internal/gateway"
)

const ChannelName = "nextcloud_talk"

// Config holds Nextcloud Talk channel settings (Hattie user sends via chat API).
type Config struct {
	BaseURL        string // Nextcloud base URL, e.g. http://nextcloud
	BotUser        string // Hattie user (Nextcloud user) for Basic Auth
	BotAppPassword string // Hattie user app password
}

// Channel implements gateway.Channel for Nextcloud Talk (webhook receive via HattieBridge, chat API send as Hattie user).
type Channel struct {
	cfg        Config
	httpClient *http.Client
}

// New creates a new Nextcloud Talk channel.
func New(cfg Config) *Channel {
	return &Channel{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Channel) Name() string {
	return ChannelName
}

// Start does not run a poll loop; webhooks are received by the HTTP server and pushed to ingress.
func (c *Channel) Start(ctx context.Context, ingress chan<- gateway.Message) error {
	<-ctx.Done()
	return nil
}

// Send posts a message to the Nextcloud Talk room via chat API as the Hattie user.
func (c *Channel) Send(msg gateway.Message) error {
	roomToken := msg.ThreadID
	if roomToken == "" {
		roomToken = msg.ReplyToID
	}
	// ReplyToID may be "roomToken:messageId"; use only the room token for the send URL
	if idx := strings.Index(roomToken, ":"); idx > 0 {
		roomToken = roomToken[:idx]
	}
	if roomToken == "" {
		return fmt.Errorf("nextcloud_talk: no room token (ThreadID or ReplyToID)")
	}
	// Parse reply ID if present, but we intentionally ignore it to avoid threaded/quoted replies
	// (User preference: keeps chat cleaner).
	/*
	replyToID := 0
	if idx := strings.Index(msg.ReplyToID, ":"); idx > 0 {
		if n, err := fmt.Sscanf(msg.ReplyToID[idx+1:], "%d", &replyToID); err == nil && n == 1 {
			// replyToID set
		}
	}
	*/
	return c.sendToRoom(roomToken, msg.Content, 0)
}

// sendToRoom posts a message via Talk chat API (Basic Auth as Hattie user).
func (c *Channel) sendToRoom(roomToken, message string, replyToID int) error {
	base := strings.TrimSuffix(c.cfg.BaseURL, "/")
	url := base + "/ocs/v2.php/apps/spreed/api/v1/chat/" + roomToken
	body := map[string]interface{}{
		"message": message,
	}
	if replyToID > 0 {
		body["replyTo"] = replyToID
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.cfg.BotUser, c.cfg.BotAppPassword)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OCS-APIRequest", "true")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusCreated {
		return nil
	}
	bodyRead, _ := io.ReadAll(resp.Body)
	errMsg := fmt.Sprintf("nextcloud_talk send: %s %s", resp.Status, string(bodyRead))
	if resp.StatusCode == http.StatusUnauthorized {
		errMsg += " (check NextcloudBotUser/BotAppPassword)"
	}
	return fmt.Errorf("%s", errMsg)
}

// SendProactive sends a message to a user. Without a room mapping we cannot send to a specific user;
// the caller may pass userID as a known room token for "DM" rooms, or we fail.
func (c *Channel) SendProactive(userID, content string) error {
	if userID != "" && !strings.Contains(userID, "@") {
		return c.sendToRoom(userID, content, 0)
	}
	return fmt.Errorf("nextcloud_talk: proactive send requires room token as userID (no user-to-room mapping)")
}
