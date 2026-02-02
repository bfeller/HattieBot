package webhookserver

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/hattiebot/hattiebot/internal/gateway"
)

const NextcloudTalkChannel = "nextcloud_talk"

// Nextcloud Talk webhook payload (Activity Streams 2.0–style).
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

// VerifyTalkSignature checks HMAC-SHA256(X-Nextcloud-Talk-Random + body, secret) == X-Nextcloud-Talk-Signature (lowercase).
func VerifyTalkSignature(body []byte, random, signature, secret string) bool {
	if secret == "" || random == "" || signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(random))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(strings.ToLower(signature)), []byte(expected))
}

// Server serves webhook and health endpoints.
type Server struct {
	Addr               string
	NextcloudSecret    string
	PushIngress        func(gateway.Message) bool
	HealthPath         string
	WebhookTalkPath    string
	ChatPath           string
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
	random := r.Header.Get("X-Nextcloud-Talk-Random")
	signature := r.Header.Get("X-Nextcloud-Talk-Signature")
	if !VerifyTalkSignature(body, random, signature, s.NextcloudSecret) {
		log.Printf("[WebhookServer] nextcloud talk webhook: invalid signature")
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
