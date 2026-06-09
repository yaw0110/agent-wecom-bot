package agent

import (
	"testing"
)

func TestStripThinking_RemovesThinkingTags(t *testing.T) {
	input := "Hello <thinking>this is a thought</thinking> world"
	expected := "Hello world"
	result := stripThinking(input)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestStripThinking_RemovesMultilineThinking(t *testing.T) {
	input := `Answer:
<thinking>
Step 1: Analyze the problem
Step 2: Consider solutions
Step 3: Pick the best one
</thinking>
Here is the final answer.`
	expected := "Answer:\n\nHere is the final answer."
	result := stripThinking(input)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestStripThinking_RemovesThinkingCodeBlock(t *testing.T) {
	input := "Let me solve this.\n```thinking\nI need to check the docs first\n```\nThe answer is 42."
	expected := "Let me solve this.\n\nThe answer is 42."
	result := stripThinking(input)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestStripThinking_RemovesChineseThoughtPrefix(t *testing.T) {
	input := "思考：我需要先查看文件内容。\n然后我找到了答案。"
	expected := "然后我找到了答案。"
	result := stripThinking(input)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestStripThinking_RemovesEnglishThoughtPrefix(t *testing.T) {
	input := "Thought: I need to analyze this.\nThe result is clear."
	expected := "The result is clear."
	result := stripThinking(input)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestStripThinking_EmptyInput(t *testing.T) {
	result := stripThinking("")
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestStripThinking_NoThinking(t *testing.T) {
	input := "This is a normal response without any thinking."
	result := stripThinking(input)
	if result != input {
		t.Errorf("expected %q, got %q", input, result)
	}
}

func TestStripThinking_OnlyThinking(t *testing.T) {
	input := "<thinking>just thinking</thinking>"
	result := stripThinking(input)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestStripThinking_MultipleThinkingBlocks(t *testing.T) {
	input := "A <thinking>first</thinking> B <thinking>second</thinking> C"
	expected := "A B C"
	result := stripThinking(input)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestStripThinking_TrimWhitespace(t *testing.T) {
	input := "  <thinking>思考</thinking>  Hello  "
	expected := "Hello"
	result := stripThinking(input)
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
