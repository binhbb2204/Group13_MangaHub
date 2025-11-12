package tcp

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type ClientSession struct {
	SessionID          string
	UserID             string
	DeviceType         string
	DeviceName         string
	ConnectedAt        time.Time
	LastHeartbeat      time.Time
	MessagesSent       int64
	MessagesReceived   int64
	LastSyncTime       time.Time
	LastSyncManga      string
	LastSyncMangaTitle string
	LastSyncChapter    int
	Subscribed         bool
	EventTypes         []string
}

func (cs *ClientSession) GetUserID() string {
	return cs.UserID
}

func (cs *ClientSession) GetDeviceType() string {
	return cs.DeviceType
}

type SessionManager struct {
	sessions        map[string]*ClientSession
	clientToSession map[string]string
	userToSessions  map[string][]string
	mu              sync.RWMutex
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions:        make(map[string]*ClientSession),
		clientToSession: make(map[string]string),
		userToSessions:  make(map[string][]string),
	}
}

func (sm *SessionManager) CreateSession(clientID, userID, deviceType, deviceName string) *ClientSession {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	session := &ClientSession{
		SessionID:        generateSessionID(deviceName, deviceType),
		UserID:           userID,
		DeviceType:       deviceType,
		DeviceName:       deviceName,
		ConnectedAt:      time.Now(),
		LastHeartbeat:    time.Now(),
		MessagesSent:     0,
		MessagesReceived: 0,
	}
	sm.sessions[session.SessionID] = session
	sm.clientToSession[clientID] = session.SessionID

	if _, exists := sm.userToSessions[userID]; !exists {
		sm.userToSessions[userID] = make([]string, 0)
	}
	sm.userToSessions[userID] = append(sm.userToSessions[userID], session.SessionID)

	return session
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

type sessionManagerAdapter struct {
	sm *SessionManager
}

func (sma *sessionManagerAdapter) GetSubscribedClients() []string {
	return sma.sm.GetSubscribedClients()
}

func (sma *sessionManagerAdapter) IsSubscribed(clientID string) bool {
	return sma.sm.IsSubscribed(clientID)
}

func (sma *sessionManagerAdapter) GetSessionByClientID(clientID string) (any, bool) {
	return sma.sm.GetSessionByClientID(clientID)
}

func (sm *SessionManager) AsInterface() interface {
	GetSubscribedClients() []string
	IsSubscribed(clientID string) bool
	GetSessionByClientID(clientID string) (any, bool)
} {
	return &sessionManagerAdapter{sm: sm}
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

func (sm *SessionManager) UpdateLastSyncWithTitle(sessionID, mangaID, mangaTitle string, chapter int) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if session, exists := sm.sessions[sessionID]; exists {
		session.LastSyncTime = time.Now()
		session.LastSyncManga = mangaID
		session.LastSyncMangaTitle = mangaTitle
		session.LastSyncChapter = chapter
	}
}

func (sm *SessionManager) RemoveSession(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	session, exists := sm.sessions[sessionID]
	if exists {
		userID := session.UserID
		sessions := sm.userToSessions[userID]
		for i, sid := range sessions {
			if sid == sessionID {
				sm.userToSessions[userID] = append(sessions[:i], sessions[i+1:]...)
				break
			}
		}
		if len(sm.userToSessions[userID]) == 0 {
			delete(sm.userToSessions, userID)
		}
	}

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
		session, exists := sm.sessions[sessionID]
		if exists {
			userID := session.UserID
			sessions := sm.userToSessions[userID]
			for i, sid := range sessions {
				if sid == sessionID {
					sm.userToSessions[userID] = append(sessions[:i], sessions[i+1:]...)
					break
				}
			}
			if len(sm.userToSessions[userID]) == 0 {
				delete(sm.userToSessions, userID)
			}
		}
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

func (sm *SessionManager) GetUserDeviceCount(userID string) int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	count := 0
	for _, sessionID := range sm.userToSessions[userID] {
		if _, exists := sm.sessions[sessionID]; exists {
			count++
		}
	}
	return count
}

func (sm *SessionManager) Subscribe(clientID string, eventTypes []string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sessionID, exists := sm.clientToSession[clientID]
	if !exists {
		return false
	}
	if session, exists := sm.sessions[sessionID]; exists {
		session.Subscribed = true
		if len(eventTypes) > 0 {
			session.EventTypes = eventTypes
		} else {
			session.EventTypes = []string{"progress", "library"}
		}
		return true
	}
	return false
}

func (sm *SessionManager) Unsubscribe(clientID string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sessionID, exists := sm.clientToSession[clientID]
	if !exists {
		return false
	}
	if session, exists := sm.sessions[sessionID]; exists {
		session.Subscribed = false
		session.EventTypes = nil
		return true
	}
	return false
}

func (sm *SessionManager) GetSubscribedClients() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	clients := make([]string, 0)
	for clientID, sessionID := range sm.clientToSession {
		if session, exists := sm.sessions[sessionID]; exists && session.Subscribed {
			clients = append(clients, clientID)
		}
	}
	return clients
}

func (sm *SessionManager) IsSubscribed(clientID string) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	sessionID, exists := sm.clientToSession[clientID]
	if !exists {
		return false
	}
	if session, exists := sm.sessions[sessionID]; exists {
		return session.Subscribed
	}
	return false
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
