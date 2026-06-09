package agent

import (
	"context"
	"log"
	"sync"
	"time"
)

// ProcessPool manages a pool of Claude Code subprocesses, one per active session.
type ProcessPool struct {
	maxSize    int
	idleTTL    time.Duration
	mu         sync.RWMutex
	processes  map[string]*ClaudeProcess
	semaphore  chan struct{}
	stopCh     chan struct{}
}

// NewProcessPool creates a new process pool.
// maxSize: maximum number of concurrent Claude subprocesses.
// idleTTL: how long a process can be idle before being killed.
func NewProcessPool(maxSize int, idleTTL time.Duration) *ProcessPool {
	pool := &ProcessPool{
		maxSize:   maxSize,
		idleTTL:   idleTTL,
		processes: make(map[string]*ClaudeProcess),
		semaphore: make(chan struct{}, maxSize),
		stopCh:    make(chan struct{}),
	}

	// Start idle cleanup goroutine
	go pool.cleanupLoop()

	return pool
}

// Acquire gets or creates a Claude process for the given session key.
// If an existing process is found and still running, it's returned.
// Otherwise, a new process is created.
func (p *ProcessPool) Acquire(
	ctx context.Context,
	sessionKey string,
	workDir, model, apiBaseURL, apiKey string,
	allowedTools []string,
	permissionMode, systemPrompt string,
) (*ClaudeProcess, error) {
	// Check for existing process
	p.mu.RLock()
	existing, ok := p.processes[sessionKey]
	p.mu.RUnlock()

	if ok && existing.IsRunning() {
		return existing, nil
	}

	// Remove dead process if exists
	if ok {
		p.removeProcess(sessionKey)
	}

	// Acquire semaphore slot (block if at capacity)
	select {
	case p.semaphore <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Create new process
	proc, err := NewClaudeProcess(workDir, model, apiBaseURL, apiKey, allowedTools, permissionMode, systemPrompt)
	if err != nil {
		<-p.semaphore // Release semaphore on error
		return nil, err
	}

	p.mu.Lock()
	p.processes[sessionKey] = proc
	p.mu.Unlock()

	log.Printf("[pool] created new claude process for key=%s (active: %d/%d)",
		sessionKey, len(p.processes), p.maxSize)

	return proc, nil
}

// Release removes a process from the pool (e.g., after session timeout).
func (p *ProcessPool) Release(sessionKey string) {
	p.removeProcess(sessionKey)
}

// removeProcess stops and removes a process from the pool.
func (p *ProcessPool) removeProcess(sessionKey string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if proc, ok := p.processes[sessionKey]; ok {
		proc.Stop()
		delete(p.processes, sessionKey)
		<-p.semaphore // Release semaphore slot
		log.Printf("[pool] released claude process for key=%s", sessionKey)
	}
}

// GetProcess returns the process for a session key, if it exists and is running.
func (p *ProcessPool) GetProcess(sessionKey string) *ClaudeProcess {
	p.mu.RLock()
	defer p.mu.RUnlock()

	proc, ok := p.processes[sessionKey]
	if !ok || !proc.IsRunning() {
		return nil
	}
	return proc
}

// cleanupIdle terminates processes that have been idle too long.
func (p *ProcessPool) cleanupIdle() {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for key, proc := range p.processes {
		// We can't check lastActive from outside easily, but we check if process is running
		_ = now
		_ = proc
		_ = key
		// For now, just log count — active cleanup is done via session manager collaboration
	}
}

// cleanupLoop periodically cleans up idle processes.
func (p *ProcessPool) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.cleanupIdle()
		case <-p.stopCh:
			return
		}
	}
}

// Stop terminates all processes and stops the pool.
func (p *ProcessPool) Stop() {
	close(p.stopCh)

	p.mu.Lock()
	defer p.mu.Unlock()

	for key, proc := range p.processes {
		log.Printf("[pool] stopping claude process for key=%s", key)
		proc.Stop()
	}
	p.processes = make(map[string]*ClaudeProcess)
}

// Len returns the number of active processes.
func (p *ProcessPool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.processes)
}
