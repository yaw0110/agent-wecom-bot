package session

import (
	"fmt"
	"sync"
	"time"
)

// SessionEntry represents a mapping between a WeCom user and a Claude Code session.
type SessionEntry struct {
	ClaudeSessionID string
	WecomUserID     string
	ChatType        string
	ChatID          string
	LastActive      time.Time
	CreatedAt       time.Time
}

// Manager manages WeCom user ↔ Claude session mappings.
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*SessionEntry
	ttl      time.Duration
	stopCh   chan struct{}
}

// NewManager creates a new session manager with the given TTL.
// It starts a background cleanup goroutine.
func NewManager(ttlMinutes int) *Manager {
	m := &Manager{
		sessions: make(map[string]*SessionEntry),
		ttl:      time.Duration(ttlMinutes) * time.Minute,
		stopCh:   make(chan struct{}),
	}
	go m.cleanupLoop()
	return m
}

// sessionKey builds a lookup key from chat type and user/chat identifiers.
// Single chat → "single:{userid}"
// Group chat → "group:{chatid}"
func sessionKey(chatType, userID, chatID string) string {
	if chatType == "group" {
		return fmt.Sprintf("group:%s", chatID)
	}
	return fmt.Sprintf("single:%s", userID)
}

// Get retrieves an existing Claude session ID for a WeCom user.
// Returns empty string if no session exists.
func (m *Manager) Get(chatType, userID, chatID string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry, ok := m.sessions[sessionKey(chatType, userID, chatID)]
	if !ok {
		return "", false
	}

	// Check if expired
	if time.Since(entry.LastActive) > m.ttl {
		return "", false
	}

	// Update last active time
	entry.LastActive = time.Now()
	return entry.ClaudeSessionID, true
}

// Set stores a Claude session ID for a WeCom user.
func (m *Manager) Set(chatType, userID, chatID, claudeSessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := sessionKey(chatType, userID, chatID)
	now := time.Now()

	if entry, ok := m.sessions[key]; ok {
		entry.ClaudeSessionID = claudeSessionID
		entry.LastActive = now
	} else {
		m.sessions[key] = &SessionEntry{
			ClaudeSessionID: claudeSessionID,
			WecomUserID:     userID,
			ChatType:        chatType,
			ChatID:          chatID,
			LastActive:      now,
			CreatedAt:       now,
		}
	}
}

// Delete removes a session mapping.
func (m *Manager) Delete(chatType, userID, chatID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, sessionKey(chatType, userID, chatID))
}

// cleanup removes expired sessions.
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for key, entry := range m.sessions {
		if now.Sub(entry.LastActive) > m.ttl {
			delete(m.sessions, key)
		}
	}
}

// cleanupLoop runs cleanup every 5 minutes.
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.cleanup()
		case <-m.stopCh:
			return
		}
	}
}

// Stop stops the background cleanup goroutine.
func (m *Manager) Stop() {
	close(m.stopCh)
}

// Len returns the number of active sessions.
func (m *Manager) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}
