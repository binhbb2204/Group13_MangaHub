package tcp

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type ClientSession struct {
	SessionID        string
	DeviceType       string
	DeviceName       string
	ConnectedAt      time.Time
	LastHeartbeat    time.Time
	MessagesSent     int64
	MessagesReceived int64
	LastSyncTime     time.Time
	LastSyncManga    string
	LastSyncChapter  int
}

type SessionManager struct {
	sessions        map[string]*ClientSession
	clientToSession map[string]string
	mu              sync.RWMutex
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:        make(map[string]*ClientSession),
		clientToSession: make(map[string]string),
	}
}

func (sm *SessionManager) CreateSession(clientID, userID, deviceType, deviceName string) *ClientSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	session := &ClientSession{
		SessionID:        generateSessionID(deviceName, deviceType),
		DeviceType:       deviceType,
		DeviceName:       deviceName,
		ConnectedAt:      time.Now(),
		LastHeartbeat:    time.Now(),
		MessagesSent:     0,
		MessagesReceived: 0,
	}
	sm.sessions[session.SessionID] = session
	sm.clientToSession[clientID] = session.SessionID
	return session
}

func (sm *SessionManager) GetSession(sessionID string) (*ClientSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	session, exists := sm.sessions[sessionID]
	return session, exists
}

func (sm *SessionManager) GetSessionByClientID(clientID string) (*ClientSession, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	sessionID, exists := sm.clientToSession[clientID]
	if !exists {
		return nil, false
	}
	session, exists := sm.sessions[sessionID]
	return session, exists
}

func (sm *SessionManager) UpdateHeartbeat(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if session, exists := sm.sessions[sessionID]; exists {
		session.LastHeartbeat = time.Now()
	}
}

func (sm *SessionManager) IncrementMessagesSent(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if session, exists := sm.sessions[sessionID]; exists {
		session.MessagesSent++
	}
}

func (sm *SessionManager) IncrementMessagesReceived(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if session, exists := sm.sessions[sessionID]; exists {
		session.MessagesReceived++
	}
}

func (sm *SessionManager) UpdateLastSync(sessionID, mangaID string, chapter int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if session, exists := sm.sessions[sessionID]; exists {
		session.LastSyncTime = time.Now()
		session.LastSyncManga = mangaID
		session.LastSyncChapter = chapter
	}
}

func (sm *SessionManager) RemoveSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, sessionID)
	for clientID, sid := range sm.clientToSession {
		if sid == sessionID {
			delete(sm.clientToSession, clientID)
			break
		}
	}
}

func (sm *SessionManager) RemoveSessionByClientID(clientID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sessionID, exists := sm.clientToSession[clientID]; exists {
		delete(sm.sessions, sessionID)
		delete(sm.clientToSession, clientID)
	}
}

func (sm *SessionManager) GetAllSessions() []*ClientSession {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	sessions := make([]*ClientSession, 0, len(sm.sessions))
	for _, session := range sm.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

func (sm *SessionManager) CleanupStale(timeout time.Duration) []string {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	now := time.Now()
	staleIDs := make([]string, 0)
	for sessionID, session := range sm.sessions {
		if now.Sub(session.LastHeartbeat) > timeout {
			staleIDs = append(staleIDs, sessionID)
			delete(sm.sessions, sessionID)
		}
	}
	return staleIDs
}

func (sm *SessionManager) GetSessionCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.sessions)
}

func generateSessionID(deviceName, deviceType string) string {
	now := time.Now()
	timestamp := now.Format("02012006T150405")
	random := randomString(4)
	sanitizedName := sanitize(deviceName)
	sanitizedType := sanitize(deviceType)
	return "sess_" + sanitizedName + "_" + sanitizedType + "_" + timestamp + "_" + random
}

func sanitize(s string) string {
	result := ""
	for _, ch := range s {
		if ch == ' ' {
			result += "_"
		} else if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
			result += string(ch)
		} else if ch >= 'A' && ch <= 'Z' {
			result += string(ch + 32)
		}
	}
	if result == "" {
		result = "unknown"
	}
	return result
}

func randomString(length int) string {
	bytes := make([]byte, length/2+1)
	if _, err := rand.Read(bytes); err != nil {
		return hex.EncodeToString([]byte{byte(time.Now().UnixNano() % 256)})[:length]
	}
	return hex.EncodeToString(bytes)[:length]
}
