package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/hattiebot/hattiebot/internal/store"
)

// Router handles message routing logic based on urgency and user preferences.
type Router struct {
	Gateway       *Gateway
	DB            *store.DB
	DefaultChannel string // e.g. "admin_term" or "nextcloud_talk"; used when user platform is unknown
}

// NewRouter creates a new router.
func NewRouter(g *Gateway, db *store.DB) *Router {
	return &Router{
		Gateway:        g,
		DB:             db,
		DefaultChannel: "admin_term",
	}
}

// RouteMessage routes a proactive message to the user based on urgency and available contact info.
func (r *Router) RouteMessage(ctx context.Context, userID, content, urgency string) error {
	// 1. Fetch Contact Info (Facts)
	// We look for phone_number or specific channel preferences
	facts, err := r.DB.SearchFacts(ctx, userID, "contact_info")
	if err != nil {
		log.Printf("[ROUTER] Failed to fetch facts for user %s: %v", userID, err)
		// Fallback to default routing
	}

	var phoneNumber string
	// var preferredChannel string // Future use

	for _, f := range facts {
		if f.Key == "phone_number" {
			phoneNumber = f.Value
		}
	}

	// 2. Logic
	targetChannel := r.DefaultChannel
	if targetChannel == "" {
		targetChannel = "admin_term"
	}
	targetID := userID

	if urgency == "urgent" && phoneNumber != "" {
		content = fmt.Sprintf("[SMS to %s]: %s", phoneNumber, content)
	}

	// Map user platform to channel when known; for nextcloud_talk, resolve room token
	user, err := r.DB.GetUser(ctx, userID)
	if err == nil && user.Platform != "" {
		if user.Platform == "terminal" {
			targetChannel = "admin_term"
		} else if user.Platform == "nextcloud_talk" {
			targetChannel = "nextcloud_talk"
			// Nextcloud Talk SendProactive requires room token, not user ID
			if user.Metadata != "" {
				var meta map[string]string
				if json.Unmarshal([]byte(user.Metadata), &meta) == nil && meta["last_room_token"] != "" {
					targetID = meta["last_room_token"]
				}
			}
		}
	}

	return r.Gateway.Broadcast(ctx, targetChannel, targetID, content, urgency)
}

// GetTargetForUser returns the channel and threadID for routing messages to a user.
// Used by PushAgentPrompt and other callers that need to know where to deliver.
func (r *Router) GetTargetForUser(ctx context.Context, userID string) (channel, threadID string) {
	channel = r.DefaultChannel
	if channel == "" {
		channel = "admin_term"
	}
	threadID = userID

	user, err := r.DB.GetUser(ctx, userID)
	if err == nil && user.Platform != "" {
		if user.Platform == "terminal" {
			channel = "admin_term"
			threadID = "terminal:console"
		} else if user.Platform == "nextcloud_talk" {
			channel = "nextcloud_talk"
			if user.Metadata != "" {
				var meta map[string]string
				if json.Unmarshal([]byte(user.Metadata), &meta) == nil && meta["last_room_token"] != "" {
					threadID = meta["last_room_token"]
				}
			}
		}
	}
	return channel, threadID
}

// PushAgentPrompt pushes a scheduled agent task into the gateway for the agent to process.
// When autonomous is true, the agent's reply is not auto-routed; it must use notify_user to send.
func (r *Router) PushAgentPrompt(ctx context.Context, userID, prompt string, autonomous bool, planID int64) bool {
	channel, threadID := r.GetTargetForUser(ctx, userID)
	if autonomous {
		threadID = fmt.Sprintf("scheduler:plan_%d", planID)
	}
	msg := Message{
		SenderID:   userID,
		Content:    "[Scheduled Task] " + prompt,
		Channel:    channel,
		ThreadID:   threadID,
		ReplyToID:  threadID,
		Autonomous: autonomous,
	}
	return r.Gateway.PushIngress(msg)
}
