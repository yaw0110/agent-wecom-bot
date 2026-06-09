package agent

import "regexp"

var (
	// <thinking>...</thinking> blocks
	reThinking = regexp.MustCompile(`<thinking>[\s\S]*?</thinking>`)

	// ```thinking ... ``` code blocks
	reThinkingCodeBlock = regexp.MustCompile("```thinking\\n[\\s\\S]*?\\n```")

	// Lines starting with "思考：" or "Thought:" etc.
	reThinkPrefix = regexp.MustCompile(`(?m)^(思考|思考过程|思考步骤|Thought|Thinking|Reasoning):?.*(\n|$)`)
)
