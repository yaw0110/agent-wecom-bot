package agent

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// StreamEvent represents a single event from Claude Code's stream-json output.
type StreamEvent struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype,omitempty"`
	Text      string `json:"text,omitempty"`
	Thinking  string `json:"thinking,omitempty"`
	Result    string `json:"result,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`

	// Tool events
	Name   string          `json:"name,omitempty"`
	Input  json.RawMessage `json:"input,omitempty"`
	Output string          `json:"output,omitempty"`

	// Metadata
	IsError bool `json:"is_error,omitempty"`
}

// InputEvent represents a message sent to Claude Code via stdin.
type InputEvent struct {
	Type    string `json:"type"`
	Message string `json:"message,omitempty"`
}

// AgentResult is the result of processing a message through Claude Code.
type AgentResult struct {
	Success   bool
	Result    string
	SessionID string
	CostUsd   float64
	Duration  time.Duration
	Error     string
}

// ClaudeProcess manages a single Claude Code subprocess.
type ClaudeProcess struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	sessionID string
	mu        sync.Mutex
	cancel    context.CancelFunc

	// Config
	workDir        string
	model          string
	allowedTools   []string
	permissionMode string
	systemPrompt   string
}

// NewClaudeProcess creates a new Claude Code subprocess.
// apiBaseURL and apiKey are for 中转站 relay (ANTHROPIC_BASE_URL / ANTHROPIC_API_KEY).
func NewClaudeProcess(workDir, model, apiBaseURL, apiKey string, allowedTools []string, permissionMode, systemPrompt string) (*ClaudeProcess, error) {
	// Build command arguments
	args := []string{
		"--bare",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--model", model,
		"--permission-mode", permissionMode,
		"--no-session-persistence",
	}

	// Add allowed tools
	if len(allowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(allowedTools, ","))
	}

	// Add system prompt
	if systemPrompt != "" {
		args = append(args, "--append-system-prompt", systemPrompt)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = workDir

	// Set environment: pass through current env + relay-specific vars
	cmd.Env = os.Environ()
	if apiBaseURL != "" {
		cmd.Env = append(cmd.Env, "ANTHROPIC_BASE_URL="+apiBaseURL)
	}
	if apiKey != "" {
		cmd.Env = append(cmd.Env, "ANTHROPIC_API_KEY="+apiKey)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to start claude process: %w", err)
	}

	// Read stderr in background (for debugging)
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Printf("[claude-stderr] %s", scanner.Text())
		}
	}()

	return &ClaudeProcess{
		cmd:            cmd,
		stdin:          stdin,
		stdout:         stdout,
		stderr:         stderr,
		cancel:         cancel,
		workDir:        workDir,
		model:          model,
		allowedTools:   allowedTools,
		permissionMode: permissionMode,
		systemPrompt:   systemPrompt,
	}, nil
}

// Process sends a message to Claude Code and returns the result.
// If this is a continuation, pass the previous session ID via resumeSessionID.
func (p *ClaudeProcess) Process(ctx context.Context, message string) (*AgentResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	resultCh := make(chan *AgentResult, 1)
	errCh := make(chan error, 1)

	go func() {
		result, err := p.processMessage(message)
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- result
	}()

	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("processing timed out: %w", ctx.Err())
	case err := <-errCh:
		return nil, err
	case result := <-resultCh:
		return result, nil
	}
}

// processMessage is the internal message processing logic.
func (p *ClaudeProcess) processMessage(message string) (*AgentResult, error) {
	// Send the user message as a stream-json event
	input := InputEvent{
		Type:    "user",
		Message: message,
	}

	encoder := json.NewEncoder(p.stdin)
	if err := encoder.Encode(input); err != nil {
		return nil, fmt.Errorf("failed to write to claude stdin: %w", err)
	}

	// Parse the stream-json output
	decoder := json.NewDecoder(p.stdout)
	var resultBuffer strings.Builder
	var resultSessionID string
	startTime := time.Now()

	for {
		var event StreamEvent
		if err := decoder.Decode(&event); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to decode claude output: %w", err)
		}

		// Track session ID
		if event.SessionID != "" {
			resultSessionID = event.SessionID
		}

		switch event.Type {
		case "thinking":
			// Skip thinking events (core of the "no thinking chain" requirement)
			continue

		case "assistant":
			// Collect text chunks
			resultBuffer.WriteString(event.Text)

		case "tool_use":
			// Tool usage is internal — skip forwarding to user
			continue

		case "tool_result":
			// Internal — skip
			continue

		case "result":
			// Final result
			duration := time.Since(startTime)

			if event.IsError {
				return &AgentResult{
					Success:   false,
					Result:    "",
					SessionID: resultSessionID,
					Error:     event.Error,
					Duration:  duration,
				}, nil
			}

			// Use the result field if present, otherwise use buffer
			finalResult := event.Result
			if finalResult == "" {
				finalResult = resultBuffer.String()
			}

			// Safety net: strip any remaining thinking artifacts
			finalResult = stripThinking(finalResult)

			return &AgentResult{
				Success:   true,
				Result:    finalResult,
				SessionID: resultSessionID,
				Duration:  duration,
			}, nil

		case "error":
			return &AgentResult{
				Success: false,
				Error:   event.Error,
			}, nil
		}
	}

	// If we got here, Claude exited without a result event
	bufferResult := stripThinking(resultBuffer.String())
	if bufferResult != "" {
		return &AgentResult{
			Success:   true,
			Result:    bufferResult,
			SessionID: resultSessionID,
			Duration:  time.Since(startTime),
		}, nil
	}

	return &AgentResult{
		Success: false,
		Error:   "claude process exited without producing a result",
	}, nil
}

// SessionID returns the current session ID.
func (p *ClaudeProcess) SessionID() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sessionID
}

// IsRunning checks if the process is still alive.
func (p *ClaudeProcess) IsRunning() bool {
	return p.cmd != nil && p.cmd.ProcessState == nil
}

// Stop terminates the Claude Code subprocess.
func (p *ClaudeProcess) Stop() error {
	p.cancel()

	// Close stdin to signal EOF
	if p.stdin != nil {
		p.stdin.Close()
	}

	// Wait for process to exit
	if p.cmd != nil && p.cmd.ProcessState == nil {
		return p.cmd.Wait()
	}
	return nil
}

// SendStdin sends a raw message to the claude process stdin.
func (p *ClaudeProcess) SendStdin(data interface{}) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return json.NewEncoder(p.stdin).Encode(data)
}
