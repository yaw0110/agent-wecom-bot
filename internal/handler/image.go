package handler

import (
	"log"

	"github.com/jhyan/agent-wecom-bot/internal/wecom"
)

// ImageHandler processes image messages.
// In v1, we inform users that image processing is not yet supported.
type ImageHandler struct {
	wecomClient *wecom.Client
}

// NewImageHandler creates a new ImageHandler.
func NewImageHandler(wecomClient *wecom.Client) *ImageHandler {
	return &ImageHandler{
		wecomClient: wecomClient,
	}
}

// Handle responds to an image message with an unsupported notice.
func (h *ImageHandler) Handle(msg *wecom.TextMessage) {
	log.Printf("[handler] received image from user=%s (v1: unsupported)", msg.From.UserID)

	reply := "抱歉，我目前还不支持图片识别功能。请发送文字描述。"
	streamID := wecom.GenerateStreamID()
	if err := h.wecomClient.ReplyStream(streamID, reply, true); err != nil {
		h.wecomClient.ReplyText(reply)
	}
}
