package bridge

import (
	"encoding/json"
	"net"
	"sync"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/metrics"
)

type ProtocolType string

const (
	ProtocolTCP       ProtocolType = "tcp"
	ProtocolUDP       ProtocolType = "udp"
	ProtocolWebSocket ProtocolType = "websocket"
	ProtocolGRPC      ProtocolType = "grpc"
)

type ProtocolClient struct {
	ID           string
	Type         ProtocolType
	Conn         interface{}
	UserID       string
	DeviceType   string
	DeviceName   string
	SessionID    string
	ConnectedAt  time.Time
	LastActivity time.Time
}

type UnifiedBridge struct {
	logger          *logger.Logger
	clients         map[string][]*ProtocolClient
	grpcBroadcaster GRPCBroadcaster
	wsBroadcaster   WebSocketBroadcaster
	udpBroadcaster  UDPBroadcaster
	sessionManager  SessionManager
	clientsLock     sync.RWMutex
	eventChan       chan UnifiedEvent
	stopChan        chan struct{}
}

func NewUnifiedBridge(log *logger.Logger) *UnifiedBridge {
	return &UnifiedBridge{
		logger:    log,
		clients:   make(map[string][]*ProtocolClient),
		eventChan: make(chan UnifiedEvent, 1000),
		stopChan:  make(chan struct{}),
	}
}

func (ub *UnifiedBridge) Start() {
	ub.logger.Info("unified_bridge_started")
	go ub.processEvents()
}

func (ub *UnifiedBridge) Stop() {
	ub.logger.Info("unified_bridge_stopping")
	close(ub.stopChan)
}

func (ub *UnifiedBridge) SetGRPCBroadcaster(broadcaster GRPCBroadcaster) {
	ub.clientsLock.Lock()
	defer ub.clientsLock.Unlock()
	ub.grpcBroadcaster = broadcaster
	ub.logger.Info("grpc_broadcaster_set")
}

func (ub *UnifiedBridge) SetWebSocketBroadcaster(broadcaster WebSocketBroadcaster) {
	ub.clientsLock.Lock()
	defer ub.clientsLock.Unlock()
	ub.wsBroadcaster = broadcaster
	ub.logger.Info("websocket_broadcaster_set")
}

func (ub *UnifiedBridge) SetUDPBroadcaster(broadcaster UDPBroadcaster) {
	ub.clientsLock.Lock()
	defer ub.clientsLock.Unlock()
	ub.udpBroadcaster = broadcaster
	ub.logger.Info("udp_broadcaster_set")
}

func (ub *UnifiedBridge) SetSessionManager(sm SessionManager) {
	ub.clientsLock.Lock()
	defer ub.clientsLock.Unlock()
	ub.sessionManager = sm
	ub.logger.Info("session_manager_set")
}

func (ub *UnifiedBridge) RegisterProtocolClient(conn interface{}, userID string, protocol ProtocolType) string {
	ub.clientsLock.Lock()
	defer ub.clientsLock.Unlock()

	clientID := generateClientID(userID, protocol)
	client := &ProtocolClient{
		ID:           clientID,
		Type:         protocol,
		Conn:         conn,
		UserID:       userID,
		ConnectedAt:  time.Now(),
		LastActivity: time.Now(),
	}

	ub.clients[userID] = append(ub.clients[userID], client)
	metrics.IncrementConnectionCount(string(protocol))

	ub.logger.Info("protocol_client_registered",
		"user_id", userID,
		"protocol", protocol,
		"client_id", clientID,
		"total_clients", len(ub.clients[userID]),
	)

	return clientID
}

func (ub *UnifiedBridge) UnregisterProtocolClient(clientID string, userID string) {
	ub.clientsLock.Lock()
	defer ub.clientsLock.Unlock()

	clients := ub.clients[userID]
	for i, client := range clients {
		if client.ID == clientID {
			metrics.DecrementConnectionCount(string(client.Type))
			ub.clients[userID] = append(clients[:i], clients[i+1:]...)
			break
		}
	}

	if len(ub.clients[userID]) == 0 {
		delete(ub.clients, userID)
	}

	ub.logger.Info("protocol_client_unregistered",
		"user_id", userID,
		"client_id", clientID,
		"remaining_clients", len(ub.clients[userID]),
	)
}

func (ub *UnifiedBridge) BroadcastEvent(event UnifiedEvent) {
	select {
	case ub.eventChan <- event:
		ub.logger.Debug("event_queued", "type", event.Type, "user_id", event.UserID)
	default:
		ub.logger.Warn("event_channel_full", "type", event.Type, "user_id", event.UserID)
	}
}

func (ub *UnifiedBridge) processEvents() {
	for {
		select {
		case event := <-ub.eventChan:
			ub.routeEvent(event)
		case <-ub.stopChan:
			ub.logger.Info("unified_bridge_stopped")
			return
		}
	}
}

func (ub *UnifiedBridge) routeEvent(event UnifiedEvent) {
	ub.clientsLock.RLock()
	clients := ub.clients[event.UserID]
	wsBroadcaster := ub.wsBroadcaster
	grpcBroadcaster := ub.grpcBroadcaster
	udpBroadcaster := ub.udpBroadcaster
	ub.clientsLock.RUnlock()

	for _, client := range clients {
		go ub.sendToClient(client, event)
	}

	if wsBroadcaster != nil {
		go wsBroadcaster.BroadcastToUser(event.UserID, event)
	}

	if grpcBroadcaster != nil {
		go grpcBroadcaster.BroadcastToUser(event.UserID, event)
	}

	if udpBroadcaster != nil {
		go udpBroadcaster.BroadcastUnifiedEvent(event.UserID, event)
	}
}

func (ub *UnifiedBridge) sendToClient(client *ProtocolClient, event UnifiedEvent) {
	switch client.Type {
	case ProtocolTCP:
		ub.sendTCPEvent(client, event)
	case ProtocolWebSocket:
		ub.sendWebSocketEvent(client, event)
	case ProtocolGRPC:
		ub.sendGRPCEvent(client, event)
	case ProtocolUDP:
		ub.sendUDPEvent(client, event)
	}
}

func (ub *UnifiedBridge) sendTCPEvent(client *ProtocolClient, event UnifiedEvent) {
	conn, ok := client.Conn.(net.Conn)
	if !ok {
		ub.logger.Error("invalid_tcp_connection", "client_id", client.ID)
		return
	}

	messageBytes, err := json.Marshal(event)
	if err != nil {
		ub.logger.Error("failed_to_marshal_event", "error", err.Error())
		return
	}

	message := string(messageBytes) + "\n"
	if _, err := conn.Write([]byte(message)); err != nil {
		ub.logger.Warn("failed_to_send_tcp_event",
			"user_id", client.UserID,
			"error", err.Error())
		metrics.IncrementBroadcastFails()
	} else {
		metrics.IncrementBroadcasts()
	}
}

func (ub *UnifiedBridge) sendWebSocketEvent(client *ProtocolClient, event UnifiedEvent) {
	if ub.wsBroadcaster != nil {
		ub.wsBroadcaster.SendToConnection(client.ID, event)
	}
}

func (ub *UnifiedBridge) sendGRPCEvent(client *ProtocolClient, event UnifiedEvent) {
	if ub.grpcBroadcaster != nil {
		ub.grpcBroadcaster.BroadcastToUser(client.UserID, event)
	}
}

func (ub *UnifiedBridge) sendUDPEvent(client *ProtocolClient, event UnifiedEvent) {
	if ub.udpBroadcaster != nil {
		ub.udpBroadcaster.BroadcastUnifiedEvent(client.UserID, event)
	}
}

func (ub *UnifiedBridge) GetActiveUserCount() int {
	ub.clientsLock.RLock()
	defer ub.clientsLock.RUnlock()
	return len(ub.clients)
}

func (ub *UnifiedBridge) GetTotalConnectionCount() int {
	ub.clientsLock.RLock()
	defer ub.clientsLock.RUnlock()

	total := 0
	for _, clients := range ub.clients {
		total += len(clients)
	}
	return total
}

func (ub *UnifiedBridge) GetProtocolStats() map[ProtocolType]int {
	ub.clientsLock.RLock()
	defer ub.clientsLock.RUnlock()

	stats := make(map[ProtocolType]int)
	for _, clients := range ub.clients {
		for _, client := range clients {
			stats[client.Type]++
		}
	}
	return stats
}

func generateClientID(userID string, protocol ProtocolType) string {
	return userID + "_" + string(protocol) + "_" + time.Now().Format("20060102150405")
}
