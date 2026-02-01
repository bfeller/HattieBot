package gateway

import (
	"context"
	"fmt"
	"sync"
)

// Message represents a generic message flowing through the gateway
type Message struct {
	SenderID   string
	Content    string
	Channel    string // "terminal", "zulip", etc.
	ThreadID   string // "stream:topic", "pm:user", etc.
	ReplyToID  string // Optional ID to reply to
}

// Channel defines the interface for all communication channels
type Channel interface {
	// Name returns the unique name of the channel
	Name() string
	// Start begins listening for messages. It should block until ctx is canceled.
	// It receives a channel to pipe messages into the gateway.
	Start(ctx context.Context, ingress chan<- Message) error
	// Send sends a message back to the channel (reply)
	Send(msg Message) error
	// SendProactive sends a message to a user or thread without a preceding request
	SendProactive(userID, content string) error
}

// Gateway manages multiple channels and routes messages to the Agent
type Gateway struct {
	channels map[string]Channel
	ingress  chan Message
	handler  func(ctx context.Context, msg Message) (string, error)
	mu       sync.RWMutex
}

// New creates a new Gateway
func New(handler func(ctx context.Context, msg Message) (string, error)) *Gateway {
	return &Gateway{
		channels: make(map[string]Channel),
		ingress:  make(chan Message, 100), // Buffer somewhat to prevent blocking
		handler:  handler,
	}
}

// Register adds a channel to the gateway
func (g *Gateway) Register(c Channel) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.channels[c.Name()] = c
}

// StartAll starts all registered channels and the ingress processor
func (g *Gateway) StartAll(ctx context.Context) error {
	var wg sync.WaitGroup

	// Start Ingress Processor
	wg.Add(1)
	go func() {
		defer wg.Done()
		g.processIngress(ctx)
	}()

	// Start Channels
	g.mu.RLock()
	for _, c := range g.channels {
		wg.Add(1)
		go func(ch Channel) {
			defer wg.Done()
			if err := ch.Start(ctx, g.ingress); err != nil {
				fmt.Printf("Error in channel %s: %v\n", ch.Name(), err)
			}
		}(c)
	}
	g.mu.RUnlock()

	<-ctx.Done()
	wg.Wait()
	return nil
}

// processIngress reads messages from channels and sends them to the agent handler
func (g *Gateway) processIngress(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-g.ingress:
			// Process message via Handler (the Agent)
			// This typically runs in a goroutine per message to handle concurrency
			go func(m Message) {
				replyContent, err := g.handler(ctx, m)
				if err != nil {
					replyContent = fmt.Sprintf("Error: %v", err)
				}
				
				// Route reply back to the source channel
				g.routeReply(m, replyContent)
			}(msg)
		}
	}
}

// routeReply sends the agent's response back to the appropriate channel
func (g *Gateway) routeReply(originalMsg Message, content string) {
	fmt.Printf("[Gateway] Routing reply to %s: %q\n", originalMsg.Channel, content)
	g.mu.RLock()
	ch, ok := g.channels[originalMsg.Channel]
	g.mu.RUnlock()

	if !ok {
		fmt.Printf("Error: Channel %s not found for reply\n", originalMsg.Channel)
		return
	}

	reply := Message{
		SenderID:  "hattiebot", // Self
		Content:   content,
		Channel:   originalMsg.Channel,
		ReplyToID: originalMsg.ReplyToID, // Or however specific channels thread messages
	}

	if err := ch.Send(reply); err != nil {
		fmt.Printf("Error sending reply to %s: %v\n", ch.Name(), err)
	}
}
// Broadcast sends a proactive message to a user via the specified channel.
func (g *Gateway) Broadcast(ctx context.Context, channelName, userID, content, urgency string) error {
	g.mu.RLock()
	ch, ok := g.channels[channelName]
	g.mu.RUnlock()

	if !ok {
		return fmt.Errorf("channel %s not found", channelName)
	}

	// Urgency handling could be here or in router. 
	// For now, simple pass-through to SendProactive.
	if urgency == "urgent" {
		content = "🚨 URGENT: " + content
	}

	return ch.SendProactive(userID, content)
}
