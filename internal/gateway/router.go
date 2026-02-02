package gateway

import (
	"context"
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

	// Map user platform to channel when known
	user, err := r.DB.GetUser(ctx, userID)
	if err == nil && user.Platform != "" {
		if user.Platform == "terminal" {
			targetChannel = "admin_term"
		} else if user.Platform == "nextcloud_talk" {
			targetChannel = "nextcloud_talk"
		}
	}

	return r.Gateway.Broadcast(ctx, targetChannel, targetID, content, urgency)
}
