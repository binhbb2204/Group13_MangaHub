package bridge

import (
	"encoding/json"
	"net"
	"sync"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

type TCPClient struct {
	Conn   net.Conn
	UserID string
}

type Bridge struct {
	logger      *logger.Logger
	clients     map[string][]*TCPClient
	clientsLock sync.RWMutex
	eventChan   chan Event
	stopChan    chan struct{}
}

func NewBridge(log *logger.Logger) *Bridge {
	return &Bridge{
		logger:    log,
		clients:   make(map[string][]*TCPClient),
		eventChan: make(chan Event, 100),
		stopChan:  make(chan struct{}),
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
		} else {
			successCount++
		}
	}

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
