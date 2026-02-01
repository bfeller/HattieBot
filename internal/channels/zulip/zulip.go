package zulip

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/hattiebot/hattiebot/internal/gateway"
)

// Config holds credentials for the Zulip bot
type Config struct {
	SiteURL string
	Email   string
	APIKey  string
}

// Client implements the Channel interface for Zulip
type Client struct {
	cfg        Config
	httpClient *http.Client
}

// New creates a new Zulip client
func New(cfg Config) *Client {
	return &Client{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) Name() string {
	return "zulip"
}

// Start begins the event polling loop
func (c *Client) Start(ctx context.Context, ingress chan<- gateway.Message) error {
	if c.cfg.SiteURL == "" || c.cfg.Email == "" || c.cfg.APIKey == "" {
		fmt.Println("Zulip: Not configured, skipping.")
		<-ctx.Done()
		return nil
	}

	fmt.Printf("Zulip: Starting poll loop for %s\n", c.cfg.Email)

	queueID, lastEventID, err := c.registerQueue()
	if err != nil {
		return fmt.Errorf("failed to register queue: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			events, newLastID, err := c.pollEvents(queueID, lastEventID)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) || os.IsTimeout(err) {
					continue
				}
				fmt.Printf("Zulip poll error: %v. Re-registering...\n", err)
				time.Sleep(5 * time.Second)
				queueID, lastEventID, err = c.registerQueue()
				if err != nil {
					fmt.Printf("Zulip re-register failed: %v\n", err)
					time.Sleep(10 * time.Second)
				}
				continue
			}

			if newLastID > -1 {
				lastEventID = newLastID
			}

			for _, event := range events {
				// Only process messages
				if event.Type != "message" {
					continue
				}
				// Ignore self
				if event.Message.SenderEmail == c.cfg.Email {
					continue
				}
				
				// Calculate reply-to ID (topic for stream, user for PM)
				// For simplicity using sender_email as ID for now
				ingress <- gateway.Message{
					SenderID:  event.Message.SenderEmail,
					Content:   event.Message.Content,
					Channel:   c.Name(),
					ThreadID:  calculateThreadID(event.Message),
					ReplyToID: calculateReplyTo(event.Message),
				}
			}
		}
	}
}

// Send posts a message to Zulip
func (c *Client) Send(msg gateway.Message) error {
	// ReplyToID format: "stream:stream_id:topic_name" or "private:email" (or just email for legacy)
	
	vals := url.Values{}
	
    // Default to private/email if no prefix
	targetType := "private"
	to := msg.ReplyToID
    topic := ""

    if len(msg.ReplyToID) > 7 && msg.ReplyToID[:7] == "stream:" {
        // stream:123:My Topic
        // Split by colon, but topic might have colons, so split N times?
        // Let's use custom parsing
        rest := msg.ReplyToID[7:]
        if idx := -1; true {
             // Find first colon
             var i int
             for i = 0; i < len(rest); i++ {
                 if rest[i] == ':' {
                     idx = i
                     break
                 }
             }
             if idx != -1 {
                 streamID := rest[:idx]
                 topic = rest[idx+1:]
                 targetType = "stream"
                 to = streamID
             }
        }
    } else if len(msg.ReplyToID) > 8 && msg.ReplyToID[:8] == "private:" {
        targetType = "private"
        to = msg.ReplyToID[8:]
    }

	if to == "" {
		return fmt.Errorf("missing recipient for Zulip message")
	}

	vals.Set("type", targetType)
    if targetType == "private" {
	    vals.Set("to", fmt.Sprintf("[%q]", to)) // JSON array for private
    } else {
        vals.Set("to", to) // Stream ID is just string/int
        vals.Set("topic", topic)
    }
	vals.Set("content", msg.Content)

	req, err := http.NewRequest("POST", c.cfg.SiteURL+"/api/v1/messages", bytes.NewBufferString(vals.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.cfg.Email, c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("zulip send failed: %s %s", resp.Status, string(body))
	}
	return nil
}

func (c *Client) SendProactive(userID, content string) error {
	// Send PM to userID (which is email)
	vals := url.Values{}
	vals.Set("type", "private")
	vals.Set("to", fmt.Sprintf("[%q]", userID))
	vals.Set("content", content)

	req, err := http.NewRequest("POST", c.cfg.SiteURL+"/api/v1/messages", bytes.NewBufferString(vals.Encode()))
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.cfg.Email, c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("zulip proactive send failed: %s %s", resp.Status, string(body))
	}
	return nil
}

// Internal structures for JSON decoding

type registerResponse struct {
	QueueID     string `json:"queue_id"`
	LastEventID int    `json:"last_event_id"`
	Result      string `json:"result"`
	Msg         string `json:"msg"`
}

type eventResponse struct {
	Events []eventData `json:"events"`
	Result string      `json:"result"`
}

type eventData struct {
	ID      int         `json:"id"`
	Type    string      `json:"type"`
	Message messageData `json:"message"`
}

type messageData struct {
	SenderID    int    `json:"sender_id"`
	SenderEmail string `json:"sender_email"`
	Content     string `json:"content"`
	Type        string `json:"type"` // "stream" or "private"
	StreamID    int    `json:"stream_id"`
	Subject     string `json:"subject"` // Topic
}

func (c *Client) registerQueue() (string, int, error) {
	vals := url.Values{}
	vals.Set("event_types", `["message"]`)
	
	req, err := http.NewRequest("POST", c.cfg.SiteURL+"/api/v1/register", bytes.NewBufferString(vals.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.SetBasicAuth(c.cfg.Email, c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	var res registerResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", 0, err
	}
	if res.Result != "success" {
		return "", 0, fmt.Errorf("api error: %s", res.Msg)
	}
	return res.QueueID, res.LastEventID, nil
}

func (c *Client) pollEvents(queueID string, lastID int) ([]eventData, int, error) {
	vals := url.Values{}
	vals.Set("queue_id", queueID)
	vals.Set("last_event_id", fmt.Sprintf("%d", lastID))
	vals.Set("dont_block", "false") // Long poll

	req, err := http.NewRequest("GET", c.cfg.SiteURL+"/api/v1/events?"+vals.Encode(), nil)
	if err != nil {
		return nil, -1, err
	}
	req.SetBasicAuth(c.cfg.Email, c.cfg.APIKey)

	// Long poll timeout
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, -1, err
	}
	defer resp.Body.Close()

	var res eventResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, -1, err
	}
	if res.Result != "success" {
		return nil, -1, fmt.Errorf("poll error")
	}

	newLastID := lastID
	for _, e := range res.Events {
		if e.ID > newLastID {
			newLastID = e.ID
		}
	}
	return res.Events, newLastID, nil
}

func calculateThreadID(m messageData) string {
	if m.Type == "stream" {
		return fmt.Sprintf("stream:%d:%s", m.StreamID, m.Subject)
	}
	// For PMs, simple sender-based thread for now
	return fmt.Sprintf("pm:%s", m.SenderEmail)
}

func calculateReplyTo(m messageData) string {
	if m.Type == "stream" {
		return fmt.Sprintf("stream:%d:%s", m.StreamID, m.Subject)
	}
	// For "private", reply to sender_email
	return "private:" + m.SenderEmail
}
