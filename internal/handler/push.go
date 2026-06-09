package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/jhyan/agent-wecom-bot/internal/wecom"
)

// PushRequest is the payload for pushing a message via the local API.
type PushRequest struct {
	SessionKey string `json:"session_key"` // "single:zhangsan" or "group:wrXXX"
	Content    string `json:"content"`
	UserID     string `json:"user_id"`     // alternative: single chat user
	ChatID     string `json:"chat_id"`     // alternative: group chat id
	ChatType   string `json:"chat_type"`   // "single" or "group"
}

// PushHandler handles local HTTP push requests from Claude Code cron tasks.
type PushHandler struct {
	wecomClient *wecom.Client
}

// NewPushHandler creates a new PushHandler.
func NewPushHandler(wecomClient *wecom.Client) *PushHandler {
	return &PushHandler{wecomClient: wecomClient}
}

// ServeHTTP handles POST /api/push requests.
func (h *PushHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PushRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("[push] bad request: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if req.Content == "" {
		http.Error(w, "content is required", http.StatusBadRequest)
		return
	}

	// Build session key from individual fields if not provided
	if req.SessionKey == "" {
		if req.ChatType == "single" && req.UserID != "" {
			req.SessionKey = "single:" + req.UserID
		} else if req.ChatType == "group" && req.ChatID != "" {
			req.SessionKey = "group:" + req.ChatID
		}
	}

	if req.SessionKey == "" {
		log.Printf("[push] missing target: %+v", req)
		http.Error(w, "target (session_key or chat_type+user_id/chat_id) is required", http.StatusBadRequest)
		return
	}

	log.Printf("[push] sending to %s: %s", req.SessionKey, truncate(req.Content, 100))

	// Send via WeCom WebSocket
	if err := h.wecomClient.ReplyText(req.Content); err != nil {
		log.Printf("[push] failed to send: %v", err)
		http.Error(w, "send failed", http.StatusInternalServerError)
		return
	}

	w.Write([]byte(`{"status":"ok"}`))
}

// BuildPushInstruction returns the system prompt snippet that tells Claude Code
// how to push cron results back to the user via the local API.
func BuildPushInstruction(sessionKey, pushBaseURL string) string {
	return strings.TrimSpace(`
## Push API for Scheduled Tasks

You can schedule cron tasks (using /cron or --cron). When a scheduled task completes
and needs to notify the user, call the local push API:

` + "```" + `
curl -s -H "Content-Type: application/json" \
  -d '{"session_key": "` + sessionKey + `", "content": "your message here"}' \
  "` + pushBaseURL + `/api/push"
` + "```" + `

The push API will deliver the message to the user who initiated this conversation.
Always use this when a scheduled task has results to report.
`)
}
