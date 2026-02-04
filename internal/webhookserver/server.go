package webhookserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"strings"


	"github.com/hattiebot/hattiebot/internal/gateway"
	"github.com/hattiebot/hattiebot/internal/core"

	"github.com/hattiebot/hattiebot/internal/secrets"
	"github.com/hattiebot/hattiebot/internal/store"
)

const NextcloudTalkChannel = "nextcloud_talk"

const HattieBridgeSecretHeader = "X-HattieBridge-Secret"

// Nextcloud Talk webhook payload (Activity Streams 2.0–style, same format from HattieBridge or Talk bot).
type talkWebhook struct {
	Type   string          `json:"type"`
	Actor  *talkActor      `json:"actor"`
	Object *talkObject     `json:"object"`
	Target *talkTarget     `json:"target"`
}

type talkActor struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type talkObject struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type talkTarget struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// object.content is JSON with "message" and "parameters"
type talkContent struct {
	Message string `json:"message"`
}

// Server serves webhook and health endpoints.
type Server struct {
	Addr               string
	HattieBridgeSecret string
	PushIngress        func(gateway.Message) bool
	HealthPath         string
	WebhookTalkPath    string
	ChatPath           string

	ConfigDir          string // for dynamic webhook routes
	SecretStore        *secrets.MultiStore
	ToolExecutor       core.ToolExecutor
}

// Run starts the HTTP server and blocks.
func (s *Server) Run() error {
	mux := http.NewServeMux()
	if s.HealthPath == "" {
		s.HealthPath = "/health"
	}
	if s.WebhookTalkPath == "" {
		s.WebhookTalkPath = "/webhook/talk"
	}
	if s.ChatPath == "" {
		s.ChatPath = "/chat"
	}

	mux.HandleFunc(s.HealthPath, s.handleHealth)
	mux.HandleFunc(s.WebhookTalkPath, s.handleNextcloudTalk)
	if s.ConfigDir != "" {
		mux.HandleFunc("/webhook/", s.handleDynamicWebhook)
	}
	mux.HandleFunc(s.ChatPath, s.handleChat)

	log.Printf("[WebhookServer] listening on %s", s.Addr)
	return http.ListenAndServe(s.Addr, mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Optional: accept JSON { "content": "...", "sender_id": "...", "thread_id": "..." } and push to ingress
	// For now return 501 so clients know it's not implemented
	http.Error(w, "not implemented", http.StatusNotImplemented)
}

func (s *Server) handleNextcloudTalk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	secret := r.Header.Get(HattieBridgeSecretHeader)
	if s.HattieBridgeSecret == "" || secret != s.HattieBridgeSecret {
		log.Printf("[WebhookServer] nextcloud talk webhook: invalid or missing X-HattieBridge-Secret")
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var payload talkWebhook
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("[WebhookServer] nextcloud talk webhook: invalid JSON: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Only process chat messages: type "Create" and object.name "message"
	if payload.Type != "Create" || payload.Object == nil || payload.Object.Name != "message" {
		w.WriteHeader(http.StatusOK)
		return
	}

	actorID := ""
	if payload.Actor != nil {
		actorID = payload.Actor.ID
	}
	actorID = normalizeNextcloudUserID(actorID)
	roomToken := ""
	if payload.Target != nil {
		roomToken = payload.Target.ID
	}
	content := ""
	if payload.Object.Content != "" {
		var tc talkContent
		if err := json.Unmarshal([]byte(payload.Object.Content), &tc); err == nil && tc.Message != "" {
			content = tc.Message
		} else {
			content = payload.Object.Content
		}
	}
	if content == "" {
		w.WriteHeader(http.StatusOK)
		return
	}

	msg := gateway.Message{
		SenderID:  actorID,
		Content:   content,
		Channel:  NextcloudTalkChannel,
		ThreadID: roomToken,
		ReplyToID: roomToken,
	}
	if payload.Object.ID != "" {
		msg.ReplyToID = roomToken + ":" + payload.Object.ID
	}

	if s.PushIngress == nil {
		log.Printf("[WebhookServer] PushIngress not set, dropping message")
		w.WriteHeader(http.StatusOK)
		return
	}
	if !s.PushIngress(msg) {
		log.Printf("[WebhookServer] ingress buffer full, dropping message")
	} else {
		log.Printf("[WebhookServer] received Talk message from %s in room %s", msg.SenderID, msg.ThreadID)
	}
	w.WriteHeader(http.StatusOK)
}

func normalizeNextcloudUserID(actorID string) string {
	const prefix = "users/"
	if strings.HasPrefix(actorID, prefix) {
		return actorID[len(prefix):]
	}
	return actorID
}

const customWebhookChannel = "custom_webhook"
const maxWebhookBodySize = 50 * 1024 // 50KB

func (s *Server) handleDynamicWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	path := r.URL.Path
	if path == "/webhook/talk" {
		http.NotFound(w, r)
		return
	}
	routes, err := store.LoadWebhookRoutes(s.ConfigDir)
	if err != nil {
		log.Printf("[WebhookServer] dynamic webhook: failed to load routes: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	var route *store.WebhookRoute
	for i := range routes {
		if routes[i].Path == path {
			route = &routes[i]
			break
		}
	}
	if route == nil {
		http.NotFound(w, r)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxWebhookBodySize+1))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if len(body) > maxWebhookBodySize {
		body = body[:maxWebhookBodySize]
	}
	
	// Secret Resolution (Fail Closed)
	source := route.SecretSource
	if source == "" {
		source = "env"
	}
	key := route.SecretKey
	if key == "" {
		key = route.SecretEnv
	}
	
	var secret string
	if s.SecretStore != nil {
		var sErr error
		secret, sErr = s.SecretStore.GetSecret(source, key)
		if sErr != nil {
			log.Printf("[WebhookServer] dynamic webhook %s: failed to get secret from %s/%s: %v", path, source, key, sErr)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	} else {
		// Fallback for backward compat if store not injected (though it should be)
		if source == "env" {
			secret = os.Getenv(key)
		} else {
			log.Printf("[WebhookServer] dynamic webhook %s: secret store missing, cannot fetch from %s", path, source)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	}

	if secret == "" {
		log.Printf("[WebhookServer] dynamic webhook %s: secret not found (source: %s, key: %s)", path, source, key)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	headerVal := r.Header.Get(route.SecretHeader)
	if headerVal == "" {
		log.Printf("[WebhookServer] dynamic webhook %s: missing %s header", path, route.SecretHeader)
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	switch route.AuthType {
	case "hmac_sha256":
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(headerVal), []byte(expected)) {
			log.Printf("[WebhookServer] dynamic webhook %s: HMAC validation failed", path)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	case "header":
		fallthrough
	default:
		if headerVal != secret {
			log.Printf("[WebhookServer] dynamic webhook %s: header mismatch", path)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	
	// Secure Routing: Enforce TargetTool
	if route.TargetTool == "" {
		log.Printf("[WebhookServer] dynamic webhook %s: missing target_tool", path)
		http.Error(w, "configuration error", http.StatusInternalServerError)
		return
	}
	
	// Construct Arguments
	argsJSON := route.TargetArgs
	if argsJSON == "" {
		argsJSON = "{}"
	}
	if strings.Contains(argsJSON, "{{payload}}") {
		// Encode body as JSON string to safely embed in JSON template
		b, _ := json.Marshal(string(body))
		// Replace {{payload}} with "body_content" (including quotes)
		argsJSON = strings.ReplaceAll(argsJSON, "{{payload}}", string(b))
	}

	// Execute Tool
	log.Printf("[WebhookServer] triggering tool %s for webhook %s", route.TargetTool, path)
	if s.ToolExecutor != nil {
		result, runErr := s.ToolExecutor.Execute(r.Context(), route.TargetTool, argsJSON)
		if runErr != nil {
			log.Printf("[WebhookServer] tool execution failed: %v", runErr)
			// We return 200 to webhook caller to avoid retries on internal failure? 
			// Or 500? Use 200 to acknowledge receipt.
		} else {
			log.Printf("[WebhookServer] tool result: %s", result)
		}
	} else {
		log.Printf("[WebhookServer] dispatcher missing (ToolExecutor), dropping webhook")
	}

	w.WriteHeader(http.StatusOK)
}
