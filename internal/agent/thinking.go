package agent

import (
	"regexp"
	"strings"
)

// stripThinking removes thinking/thought content from Claude's responses.
// This is a safety net — the main filtering happens at the stream-json event level.
func stripThinking(text string) string {
	if text == "" {
		return text
	}

	// Remove <thinking> extended thinking blocks
	text = reThinking.ReplaceAllString(text, "")

	// Remove ```thinking code blocks
	text = reThinkingCodeBlock.ReplaceAllString(text, "")

	// Remove common thinking indicators
	text = reThinkPrefix.ReplaceAllString(text, "")

	// Clean up extra whitespace caused by removals
	// Collapse multiple spaces into one
	text = regexp.MustCompile(`[ ]{2,}`).ReplaceAllString(text, " ")
	// Collapse 3+ newlines into double newline (paragraph separation)
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")

	return strings.TrimSpace(text)
}
