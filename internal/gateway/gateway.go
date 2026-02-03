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
	Channel    string // "admin_term", "nextcloud_talk", etc.
	ThreadID   string // "stream:topic", "pm:user", etc.
	ReplyToID  string // Optional ID to reply to
	Autonomous bool   // When true, agent's reply is not auto-routed; agent must use notify_user to send
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
	channels   map[string]Channel
	ingress    chan Message
	handler    func(ctx context.Context, msg Message) (string, error)
	mu         sync.RWMutex
	turnsMu    sync.Mutex
	inFlight   map[string]bool
	pending    map[string][]Message
}

// threadKey returns a key for per-thread serialization
func threadKey(m Message) string {
	return ThreadKey(m)
}

// ThreadKey returns a key for per-thread serialization. Exported so the agent loop can fetch pending messages.
func ThreadKey(m Message) string {
	if m.ThreadID != "" {
		return m.Channel + ":" + m.ThreadID
	}
	return m.Channel + ":user:" + m.SenderID
}

// GetPendingAndClear returns and removes any messages that arrived while this turn was in progress.
// The agent loop calls this between tool rounds so the model can see new user messages (e.g. "stop").
func (g *Gateway) GetPendingAndClear(threadKey string) []Message {
	g.turnsMu.Lock()
	defer g.turnsMu.Unlock()
	msgs := g.pending[threadKey]
	delete(g.pending, threadKey)
	return msgs
}

// New creates a new Gateway
func New(handler func(ctx context.Context, msg Message) (string, error)) *Gateway {
	return &Gateway{
		channels: make(map[string]Channel),
		ingress:  make(chan Message, 100), // Buffer somewhat to prevent blocking
		handler:  handler,
		inFlight: make(map[string]bool),
		pending:  make(map[string][]Message),
	}
}

// Register adds a channel to the gateway
func (g *Gateway) Register(c Channel) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.channels[c.Name()] = c
}

// PushIngress delivers a message into the gateway from an external source (e.g. HTTP webhook).
// It is non-blocking: if the ingress buffer is full, the message is dropped and false is returned.
func (g *Gateway) PushIngress(msg Message) bool {
	select {
	case g.ingress <- msg:
		return true
	default:
		return false
	}
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

// processIngress reads messages from channels and sends them to the agent handler.
// Per-thread serialization: only one turn at a time per thread. Messages that arrive
// while a turn is in progress are queued and injected into the conversation between
// tool rounds, so the agent can see them (e.g. "stop") and respond.
func (g *Gateway) processIngress(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-g.ingress:
			tk := threadKey(msg)
			g.turnsMu.Lock()
			if g.inFlight[tk] {
				g.pending[tk] = append(g.pending[tk], msg)
				g.turnsMu.Unlock()
				continue
			}
			g.inFlight[tk] = true
			g.turnsMu.Unlock()
			go g.runTurn(ctx, msg)
		}
	}
}

func (g *Gateway) runTurn(ctx context.Context, m Message) {
	tk := threadKey(m)
	defer func() {
		g.turnsMu.Lock()
		delete(g.inFlight, tk)
		next := g.pending[tk]
		if len(next) > 0 {
			g.pending[tk] = next[1:]
			g.inFlight[tk] = true
			g.turnsMu.Unlock()
			go g.runTurn(ctx, next[0])
		} else {
			delete(g.pending, tk)
			g.turnsMu.Unlock()
		}
	}()
	replyContent, err := g.handler(ctx, m)
	if err != nil {
		replyContent = fmt.Sprintf("Error: %v", err)
	}
	if m.Autonomous {
		fmt.Printf("[Gateway] Autonomous task completed (reply not routed): %q\n", replyContent)
		return
	}
	g.routeReply(m, replyContent)
}

// RouteReply sends content back to the appropriate channel. Exported so the agent loop can send intermediate status updates.
func (g *Gateway) RouteReply(originalMsg Message, content string) {
	g.routeReply(originalMsg, content)
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
		SenderID:   "hattiebot", // Self
		Content:    content,
		Channel:    originalMsg.Channel,
		ThreadID:   originalMsg.ThreadID,
		ReplyToID:  originalMsg.ReplyToID,
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
