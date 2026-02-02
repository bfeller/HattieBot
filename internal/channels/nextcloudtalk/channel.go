package nextcloudtalk

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hattiebot/hattiebot/internal/gateway"
)

const ChannelName = "nextcloud_talk"

// Config holds Nextcloud Talk bot settings.
type Config struct {
	BaseURL string // Nextcloud base URL, e.g. http://nextcloud
	Secret  string // Shared secret from occ talk:bot:install
}

// Channel implements gateway.Channel for Nextcloud Talk (webhook receive, OCS send).
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

// Send posts a message to the Nextcloud Talk room (room token from msg.ThreadID or msg.ReplyToID).
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
	return c.sendToRoom(roomToken, msg.Content, 0)
}

// sendToRoom posts a message to the given room with optional replyTo message ID.
func (c *Channel) sendToRoom(roomToken, message string, replyToID int) error {
	base := strings.TrimSuffix(c.cfg.BaseURL, "/")
	url := base + "/ocs/v2.php/apps/spreed/api/v1/bot/" + roomToken + "/message"
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

	random := make([]byte, 32)
	if _, err := rand.Read(random); err != nil {
		return err
	}
	randomHex := hex.EncodeToString(random)
	// Nextcloud verifies HMAC(random + message) with the shared secret; they use the "message" parameter, not the raw body.
	mac := hmac.New(sha256.New, []byte(c.cfg.Secret))
	mac.Write([]byte(randomHex))
	mac.Write([]byte(message))
	sig := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequest("POST", url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OCS-APIRequest", "true")
	req.Header.Set("X-Nextcloud-Talk-Bot-Random", randomHex)
	req.Header.Set("X-Nextcloud-Talk-Bot-Signature", sig)

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
		errMsg += " (check NEXTCLOUD_TALK_BOT_SECRET matches the secret used in occ talk:bot:install)"
	}
	return fmt.Errorf("%s", errMsg)
}

// SendProactive sends a message to a user. Without a room mapping we cannot send to a specific user;
// the caller may pass userID as a known room token for "DM" rooms, or we fail.
func (c *Channel) SendProactive(userID, content string) error {
	// Nextcloud Talk: proactive to "user" requires knowing the conversation token.
	// For simplicity we treat userID as room token when it looks like a token (e.g. not email).
	if userID != "" && !strings.Contains(userID, "@") {
		return c.sendToRoom(userID, content, 0)
	}
	return fmt.Errorf("nextcloud_talk: proactive send requires room token as userID (no user-to-room mapping)")
}
