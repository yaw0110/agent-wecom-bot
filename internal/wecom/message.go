package wecom

import (
	"fmt"
	"time"
)

// GenerateStreamID creates a unique stream ID for reply streaming.
func GenerateStreamID() string {
	return fmt.Sprintf("stream_%d", time.Now().UnixNano())
}

// BuildMarkdown formats a plain text as a WeCom-compatible markdown reply.
// WeCom supports limited markdown: bold, links, code blocks, and lists.
func BuildMarkdown(text string) string {
	// Simple formatting — just wrap the text, WeCom handles the rendering
	return text
}

// TruncateMessage limits message length to avoid exceeding WeCom limits.
func TruncateMessage(msg string, maxLen int) string {
	if len(msg) <= maxLen {
		return msg
	}

	// Truncate at maxLen and add ellipsis
	truncated := msg[:maxLen-3]
	return truncated + "..."
}

// GetUserMention returns a formatted user mention string.
func GetUserMention(userID string) string {
	return fmt.Sprintf("<@%s>", userID)
}
