package handler

import (
	"log"

	"github.com/jhyan/agent-wecom-bot/internal/wecom"
)

// EventHandler handles events from WeCom (enter_chat, etc.).
type EventHandler struct {
	wecomClient *wecom.Client
}

// NewEventHandler creates a new EventHandler.
func NewEventHandler(wecomClient *wecom.Client) *EventHandler {
	return &EventHandler{
		wecomClient: wecomClient,
	}
}

// HandleEnterChat sends a welcome message when a user enters the bot's chat.
func (h *EventHandler) HandleEnterChat(userID string) {
	log.Printf("[handler] user=%s entered chat, sending welcome", userID)

	welcome := "你好！我是 AI 助手，基于 Claude Code 驱动。\n\n" +
		"你可以直接发送文字消息与我交流，我可以帮助你：\n" +
		"- 编写和审查代码\n" +
		"- 回答技术问题\n" +
		"- 文件操作和项目管理\n" +
		"- 其他开发相关任务\n\n" +
		"请问有什么可以帮你的？"

	streamID := wecom.GenerateStreamID()
	if err := h.wecomClient.ReplyStream(streamID, welcome, true); err != nil {
		h.wecomClient.ReplyText(welcome)
	}
}
