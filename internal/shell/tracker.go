package shell

import (
	"sync"
	"time"
)

// SessionInfo holds metadata about an active shell session on the farmer side.
// The farmer tracks sessions to log audit events when sessions end.
type SessionInfo struct {
	SessionID   string    `json:"session_id"`
	SproutID    string    `json:"sprout_id"`
	Pubkey      string    `json:"pubkey"`
	RoleName    string    `json:"role"`
	Shell       string    `json:"shell,omitempty"`
	StartedAt   time.Time `json:"started_at"`
	DoneSubject string    `json:"-"`
}

// Tracker keeps track of active shell sessions on the farmer side.
type Tracker struct {
	mu       sync.Mutex
	sessions map[string]*SessionInfo
}

// NewTracker creates a new session tracker.
func NewTracker() *Tracker {
	return &Tracker{
		sessions: make(map[string]*SessionInfo),
	}
}

// Add registers a new active session.
func (t *Tracker) Add(info *SessionInfo) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.sessions[info.SessionID] = info
}

// Remove removes a session and returns its info, or nil if not found.
func (t *Tracker) Remove(sessionID string) *SessionInfo {
	t.mu.Lock()
	defer t.mu.Unlock()
	info, ok := t.sessions[sessionID]
	if !ok {
		return nil
	}
	delete(t.sessions, sessionID)
	return info
}

// Get returns session info for the given ID, or nil if not found.
func (t *Tracker) Get(sessionID string) *SessionInfo {
	t.mu.Lock()
	defer t.mu.Unlock()
	info := t.sessions[sessionID]
	return info
}

// Active returns the number of active sessions.
func (t *Tracker) Active() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.sessions)
}

// List returns a copy of all active sessions.
func (t *Tracker) List() []*SessionInfo {
	t.mu.Lock()
	defer t.mu.Unlock()
	result := make([]*SessionInfo, 0, len(t.sessions))
	for _, info := range t.sessions {
		result = append(result, info)
	}
	return result
}
