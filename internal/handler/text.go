package handler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jhyan/agent-wecom-bot/internal/agent"
	"github.com/jhyan/agent-wecom-bot/internal/wecom"
)

// TextHandler processes text messages from WeCom users.
type TextHandler struct {
	wecomClient  *wecom.Client
	processPool  *agent.ProcessPool
	workDir      string
	model        string
	apiBaseURL   string
	apiKey       string
	allowedTools []string
	permMode     string
	systemPrompt string
	timeout      time.Duration
	pushPort     int
}

// NewTextHandler creates a new TextHandler.
func NewTextHandler(
	wecomClient *wecom.Client,
	processPool *agent.ProcessPool,
	workDir, model, apiBaseURL, apiKey string,
	allowedTools []string,
	permMode, systemPrompt string,
	timeoutSec int,
	pushPort int,
) *TextHandler {
	return &TextHandler{
		wecomClient:  wecomClient,
		processPool:  processPool,
		workDir:      workDir,
		model:        model,
		apiBaseURL:   apiBaseURL,
		apiKey:       apiKey,
		allowedTools: allowedTools,
		permMode:     permMode,
		systemPrompt: systemPrompt,
		timeout:      time.Duration(timeoutSec) * time.Second,
		pushPort:     pushPort,
	}
}

// Handle processes a text message from WeCom.
// Flow: WeCom text → Claude Code subprocess → strip thinking → reply
func (h *TextHandler) Handle(msg *wecom.TextMessage) {
	log.Printf("[handler] received text from user=%s chattype=%s: %s",
		msg.From.UserID, msg.ChatType, truncate(msg.Content, 100))

	// Build session key and push callback instruction
	sessionKey := h.buildSessionKey(msg)
	pushInstruction := BuildPushInstruction(sessionKey, fmt.Sprintf("http://localhost:%d", h.pushPort))

	// Build final system prompt with push callback info
	finalSystemPrompt := h.systemPrompt
	if finalSystemPrompt != "" {
		finalSystemPrompt += "\n\n"
	}
	finalSystemPrompt += pushInstruction

	// Acquire or create a Claude process for this session
	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	proc, err := h.processPool.Acquire(
		ctx,
		sessionKey,
		h.workDir,
		h.model,
		h.apiBaseURL,
		h.apiKey,
		h.allowedTools,
		h.permMode,
		h.systemPrompt,
	)
	if err != nil {
		log.Printf("[handler] failed to acquire claude process: %v", err)
		h.sendError(msg, "抱歉，无法连接到 AI 引擎，请稍后再试。")
		return
	}

	// Send typing indicator
	streamID := wecom.GenerateStreamID()
	if err := h.wecomClient.ReplyStream(streamID, "正在思考...", false); err != nil {
		log.Printf("[handler] failed to send typing indicator: %v", err)
	}

	// Process through Claude Code (with push instruction)
	result, err := proc.Process(ctx, msg.Content)
	if err != nil {
		log.Printf("[handler] claude process error: %v", err)
		h.sendError(msg, "抱歉，处理您的消息时出现了错误。")
		return
	}

	if !result.Success {
		log.Printf("[handler] claude returned error: %s", result.Error)
		h.sendError(msg, "抱歉，处理您的消息时出现了错误。")
		return
	}

	// Log duration and cost
	log.Printf("[handler] claude response in %v (session=%s)", result.Duration, result.SessionID)

	// Send final answer
	answer := result.Result
	if answer == "" {
		answer = "已处理完毕。"
	}

	if err := h.wecomClient.ReplyStream(streamID, answer, true); err != nil {
		log.Printf("[handler] failed to send reply: %v", err)
		// Fallback to direct text reply
		h.wecomClient.ReplyText(answer)
	}
}

// buildSessionKey creates a unique key for this conversation.
func (h *TextHandler) buildSessionKey(msg *wecom.TextMessage) string {
	if msg.ChatType == "group" {
		return fmt.Sprintf("group:%s", msg.ChatID)
	}
	return fmt.Sprintf("single:%s", msg.From.UserID)
}

// sendError sends an error message to the user.
func (h *TextHandler) sendError(msg *wecom.TextMessage, errMsg string) {
	streamID := wecom.GenerateStreamID()
	if err := h.wecomClient.ReplyStream(streamID, errMsg, true); err != nil {
		h.wecomClient.ReplyText(errMsg)
	}
}

// truncate truncates a string for logging.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
