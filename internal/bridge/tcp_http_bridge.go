package bridge

import (
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/metrics"
)

type TCPClient struct {
	Conn   net.Conn
	UserID string
}

type UDPBroadcaster interface {
	BroadcastToUser(userID string, event BroadcastEvent)
}

type BroadcastEvent struct {
	UserID    string
	EventType string
	Data      interface{}
}

type Bridge struct {
	logger         *logger.Logger
	clients        map[string][]*TCPClient
	udpBroadcaster UDPBroadcaster
	sessionManager SessionManager
	clientsLock    sync.RWMutex
	eventChan      chan Event
	stopChan       chan struct{}
}

type SessionManager interface {
	GetSubscribedClients() []string
	IsSubscribed(clientID string) bool
	GetSessionByClientID(clientID string) (any, bool)
}

type Session interface {
	GetUserID() string
	GetDeviceType() string
}

func NewBridge(log *logger.Logger) *Bridge {
	return &Bridge{
		logger:         log,
		clients:        make(map[string][]*TCPClient),
		udpBroadcaster: nil,
		eventChan:      make(chan Event, 100),
		stopChan:       make(chan struct{}),
	}
}

func (b *Bridge) Start() {
	b.logger.Info("bridge_started")
	go b.processEvents()
}

func (b *Bridge) Stop() {
	b.logger.Info("bridge_stopping")
	close(b.stopChan)
}

func (b *Bridge) SetUDPBroadcaster(broadcaster UDPBroadcaster) {
	b.clientsLock.Lock()
	defer b.clientsLock.Unlock()
	b.udpBroadcaster = broadcaster
	b.logger.Info("udp_broadcaster_set")
}

func (b *Bridge) SetSessionManager(sm SessionManager) {
	b.clientsLock.Lock()
	defer b.clientsLock.Unlock()
	b.sessionManager = sm
	b.logger.Info("session_manager_set")
}

func (b *Bridge) RegisterTCPClient(conn net.Conn, userID string) {
	b.clientsLock.Lock()
	defer b.clientsLock.Unlock()

	client := &TCPClient{
		Conn:   conn,
		UserID: userID,
	}

	b.clients[userID] = append(b.clients[userID], client)
	b.logger.Info("tcp_client_registered",
		"user_id", userID,
		"client_addr", conn.RemoteAddr().String(),
		"total_clients", len(b.clients[userID]),
	)
}

func (b *Bridge) UnregisterTCPClient(conn net.Conn, userID string) {
	b.clientsLock.Lock()
	defer b.clientsLock.Unlock()

	clients := b.clients[userID]
	for i, client := range clients {
		if client.Conn == conn {
			b.clients[userID] = append(clients[:i], clients[i+1:]...)
			break
		}
	}

	if len(b.clients[userID]) == 0 {
		delete(b.clients, userID)
	}

	b.logger.Info("tcp_client_unregistered",
		"user_id", userID,
		"client_addr", conn.RemoteAddr().String(),
		"remaining_clients", len(b.clients[userID]),
	)
}

func (b *Bridge) NotifyProgressUpdate(event ProgressUpdateEvent) {
	data := map[string]interface{}{
		"manga_id":       event.MangaID,
		"chapter_id":     event.ChapterID,
		"status":         event.Status,
		"last_read_date": event.LastReadDate,
	}

	b.eventChan <- Event{
		Type:      EventTypeProgressUpdate,
		UserID:    event.UserID,
		Data:      data,
		Timestamp: event.LastReadDate,
	}

	b.logger.Debug("progress_update_queued",
		"user_id", event.UserID,
		"manga_id", event.MangaID,
		"chapter_id", event.ChapterID,
	)

	b.broadcastUpdateEvent(event.UserID, "updated", event.MangaID, event.ChapterID, "outgoing")

	if b.udpBroadcaster != nil {
		b.udpBroadcaster.BroadcastToUser(event.UserID, BroadcastEvent{
			EventType: "progress_update",
			Data:      data,
		})
	}
}

func (b *Bridge) NotifyLibraryUpdate(event LibraryUpdateEvent) {
	data := map[string]interface{}{
		"manga_id": event.MangaID,
		"action":   event.Action,
	}

	b.eventChan <- Event{
		Type:   EventTypeLibraryUpdate,
		UserID: event.UserID,
		Data:   data,
	}

	b.logger.Debug("library_update_queued",
		"user_id", event.UserID,
		"manga_id", event.MangaID,
		"action", event.Action,
	)

	b.broadcastUpdateEvent(event.UserID, event.Action, event.MangaID, 0, "outgoing")

	if b.udpBroadcaster != nil {
		b.udpBroadcaster.BroadcastToUser(event.UserID, BroadcastEvent{
			EventType: "library_update",
			Data:      data,
		})
	}
}

func (b *Bridge) broadcastUpdateEvent(userID, action, mangaTitle string, chapter int, direction string) {
	if b.sessionManager == nil {
		return
	}

	subscribedClients := b.sessionManager.GetSubscribedClients()
	if len(subscribedClients) == 0 {
		return
	}

	b.clientsLock.RLock()
	clients := b.clients[userID]
	b.clientsLock.RUnlock()

	for _, clientID := range subscribedClients {
		if !b.sessionManager.IsSubscribed(clientID) {
			continue
		}

		sessionAny, ok := b.sessionManager.GetSessionByClientID(clientID)
		if !ok {
			continue
		}

		session, ok := sessionAny.(Session)
		if !ok {
			continue
		}

		if session.GetUserID() != userID {
			continue
		}

		updateEvent := map[string]interface{}{
			"type": "update_event",
			"payload": map[string]interface{}{
				"timestamp":   generateTimestamp(),
				"direction":   direction,
				"action":      action,
				"manga_title": mangaTitle,
				"chapter":     chapter,
				"device_type": session.GetDeviceType(),
			},
		}

		messageBytes, err := json.Marshal(updateEvent)
		if err != nil {
			b.logger.Error("failed_to_marshal_update_event", "error", err.Error())
			continue
		}

		message := string(messageBytes) + "\n"

		for _, client := range clients {
			if _, err := client.Conn.Write([]byte(message)); err != nil {
				b.logger.Warn("failed_to_send_update_event",
					"user_id", userID,
					"error", err.Error())
			}
		}
	}
}

func (b *Bridge) BroadcastToUser(userID string, event Event) {
	b.clientsLock.RLock()
	clients := b.clients[userID]
	b.clientsLock.RUnlock()

	if len(clients) == 0 {
		b.logger.Debug("no_tcp_clients_for_user", "user_id", userID)
		return
	}

	messageBytes, err := json.Marshal(event)
	if err != nil {
		b.logger.Error("failed_to_marshal_event",
			"user_id", userID,
			"error", err.Error(),
		)
		return
	}

	message := string(messageBytes) + "\n"
	successCount := 0
	failCount := 0

	for _, client := range clients {
		_, err := client.Conn.Write([]byte(message))
		if err != nil {
			b.logger.Warn("failed_to_send_to_client",
				"user_id", userID,
				"client_addr", client.Conn.RemoteAddr().String(),
				"error", err.Error(),
			)
			failCount++
			metrics.IncrementBroadcastFails()
		} else {
			successCount++
			metrics.IncrementBroadcasts()
		}
	}

	metrics.SetActiveConnections(int64(b.GetTotalConnectionCount()))

	b.logger.Info("event_broadcast_complete",
		"user_id", userID,
		"event_type", event.Type,
		"success_count", successCount,
		"fail_count", failCount,
	)
}

func (b *Bridge) GetActiveUserCount() int {
	b.clientsLock.RLock()
	defer b.clientsLock.RUnlock()
	return len(b.clients)
}

func (b *Bridge) GetTotalConnectionCount() int {
	b.clientsLock.RLock()
	defer b.clientsLock.RUnlock()

	total := 0
	for _, clients := range b.clients {
		total += len(clients)
	}
	return total
}

func (b *Bridge) processEvents() {
	for {
		select {
		case event := <-b.eventChan:
			b.BroadcastToUser(event.UserID, event)
		case <-b.stopChan:
			b.logger.Info("bridge_stopped")
			return
		}
	}
}

func generateTimestamp() string {
	return time.Now().Format(time.RFC3339)
}
