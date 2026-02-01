package gateway

import (
	"context"
	"fmt"
	"log"

	"github.com/hattiebot/hattiebot/internal/store"
)

// Router handles message routing logic based on urgency and user preferences.
type Router struct {
	Gateway *Gateway
	DB      *store.DB
}

// NewRouter creates a new router.
func NewRouter(g *Gateway, db *store.DB) *Router {
	return &Router{
		Gateway: g,
		DB:      db,
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
	targetChannel := "zulip"      // Default fallback
	targetID := userID           // Default ID (usually email for Zulip)

	// If urgent and we have phone, try SMS (if adapter exists)
	// For now, we don't have SMS adapter, so we use Terminal or log special alert.
	// But let's assume if phone exists, we pretend to send SMS via logging for now, 
	// or route to a special "sms_mock" channel if registered.
	
	if urgency == "urgent" && phoneNumber != "" {
		// Try routing to SMS channel if exists
		// For MVP, since we don't have SMS adapter, we just prefix [SMS: +1234]
		content = fmt.Sprintf("[SMS to %s]: %s", phoneNumber, content)
		// Still send to Zulip/Default as fallback or primary for now until real SMS adapter is built.
		// Alternatively, if Admin Terminal is active, send there? 
		// Let's stick to Zulip PM for now but with the modified content to show we "would" have used SMS.
	}

	// 3. Dispatch
	// We need to know if "zulip" or "terminal" or whatever is the right channel for this user.
	// We can look up user's "Platform" from users table if needed.
	user, err := r.DB.GetUser(ctx, userID)
	if err == nil && user.Platform != "" {
		// Map platform to channel name roughly
		if user.Platform == "terminal" {
			targetChannel = "admin_term"
		} else if user.Platform == "zulip" || user.Platform == "email" {
			targetChannel = "zulip"
		}
	}

	return r.Gateway.Broadcast(ctx, targetChannel, targetID, content, urgency)
}
