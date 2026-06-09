package session

import (
	"testing"
	"time"
)

func TestSetAndGet_SingleChat(t *testing.T) {
	m := NewManager(30)
	defer m.Stop()

	m.Set("single", "user1", "", "session-abc")
	sessionID, ok := m.Get("single", "user1", "")
	if !ok {
		t.Fatal("expected session to exist")
	}
	if sessionID != "session-abc" {
		t.Errorf("expected session-abc, got %s", sessionID)
	}
}

func TestSetAndGet_GroupChat(t *testing.T) {
	m := NewManager(30)
	defer m.Stop()

	m.Set("group", "", "chat-123", "session-xyz")
	sessionID, ok := m.Get("group", "", "chat-123")
	if !ok {
		t.Fatal("expected session to exist")
	}
	if sessionID != "session-xyz" {
		t.Errorf("expected session-xyz, got %s", sessionID)
	}
}

func TestGet_NonExistent(t *testing.T) {
	m := NewManager(30)
	defer m.Stop()

	_, ok := m.Get("single", "nonexistent", "")
	if ok {
		t.Fatal("expected no session for nonexistent user")
	}
}

func TestDelete(t *testing.T) {
	m := NewManager(30)
	defer m.Stop()

	m.Set("single", "user1", "", "session-abc")
	m.Delete("single", "user1", "")
	_, ok := m.Get("single", "user1", "")
	if ok {
		t.Fatal("expected session to be deleted")
	}
}

func TestUpdate_SameUser(t *testing.T) {
	m := NewManager(30)
	defer m.Stop()

	m.Set("single", "user1", "", "session-abc")
	m.Set("single", "user1", "", "session-def")

	sessionID, ok := m.Get("single", "user1", "")
	if !ok {
		t.Fatal("expected session to exist")
	}
	if sessionID != "session-def" {
		t.Errorf("expected session-def, got %s", sessionID)
	}
}

func TestCleanup_ExpiredSessions(t *testing.T) {
	m := &Manager{
		sessions: make(map[string]*SessionEntry),
		ttl:      50 * time.Millisecond,
		stopCh:   make(chan struct{}),
	}

	m.Set("single", "user1", "", "session-abc")
	m.Set("single", "user2", "", "session-xyz")

	time.Sleep(100 * time.Millisecond)
	m.cleanup()

	if m.Len() != 0 {
		t.Errorf("expected 0 sessions after cleanup, got %d", m.Len())
	}

	m.Stop()
}

func TestLen(t *testing.T) {
	m := NewManager(30)
	defer m.Stop()

	if m.Len() != 0 {
		t.Errorf("expected 0, got %d", m.Len())
	}

	m.Set("single", "user1", "", "session-a")
	m.Set("single", "user2", "", "session-b")

	if m.Len() != 2 {
		t.Errorf("expected 2, got %d", m.Len())
	}
}
