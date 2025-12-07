package websocket

import (
	"encoding/json"
	"sync"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

type WSBroadcaster struct {
	manager      *Manager
	bridge       *bridge.UnifiedBridge
	logger       *logger.Logger
	connMap      map[string]*Client
	connMapMutex sync.RWMutex
}

func NewWSBroadcaster(manager *Manager, log *logger.Logger) *WSBroadcaster {
	return &WSBroadcaster{
		manager: manager,
		logger:  log,
		connMap: make(map[string]*Client),
	}
}

func (wb *WSBroadcaster) SetBridge(bridge *bridge.UnifiedBridge) {
	wb.bridge = bridge
	wb.logger.Info("ws_broadcaster_bridge_set")
}

func (wb *WSBroadcaster) RegisterConnection(connID string, client *Client) {
	wb.connMapMutex.Lock()
	defer wb.connMapMutex.Unlock()
	wb.connMap[connID] = client
	wb.logger.Debug("ws_connection_registered", "conn_id", connID, "user_id", client.ID)
}

func (wb *WSBroadcaster) UnregisterConnection(connID string) {
	wb.connMapMutex.Lock()
	defer wb.connMapMutex.Unlock()
	delete(wb.connMap, connID)
	wb.logger.Debug("ws_connection_unregistered", "conn_id", connID)
}

func (wb *WSBroadcaster) SendToConnection(connID string, event bridge.UnifiedEvent) error {
	wb.connMapMutex.RLock()
	client, ok := wb.connMap[connID]
	wb.connMapMutex.RUnlock()

	if !ok {
		wb.logger.Warn("ws_connection_not_found", "conn_id", connID)
		return nil
	}

	messageBytes, err := json.Marshal(event)
	if err != nil {
		wb.logger.Error("failed_to_marshal_event", "error", err.Error())
		return err
	}

	select {
	case client.Send <- messageBytes:
		wb.logger.Debug("ws_event_sent", "conn_id", connID, "event_type", event.Type)
	default:
		wb.logger.Warn("ws_send_channel_full", "conn_id", connID)
	}

	return nil
}

func (wb *WSBroadcaster) BroadcastToUser(userID string, event bridge.UnifiedEvent) {
	client, ok := wb.manager.GetClient(userID)
	if !ok {
		wb.logger.Debug("ws_user_not_connected", "user_id", userID)
		return
	}

	messageBytes, err := json.Marshal(event)
	if err != nil {
		wb.logger.Error("failed_to_marshal_event", "error", err.Error())
		return
	}

	select {
	case client.Send <- messageBytes:
		wb.logger.Debug("ws_event_broadcast", "user_id", userID, "event_type", event.Type)
	default:
		wb.logger.Warn("ws_send_channel_full", "user_id", userID)
	}
}

func (wb *WSBroadcaster) GetActiveConnections(userID string) []string {
	wb.connMapMutex.RLock()
	defer wb.connMapMutex.RUnlock()

	connections := make([]string, 0)
	for connID, client := range wb.connMap {
		if client.ID == userID {
			connections = append(connections, connID)
		}
	}
	return connections
}

func (wb *WSBroadcaster) CloseConnection(connID string) error {
	wb.connMapMutex.RLock()
	client, ok := wb.connMap[connID]
	wb.connMapMutex.RUnlock()

	if !ok {
		return nil
	}

	client.Conn.Close()
	wb.UnregisterConnection(connID)
	wb.logger.Info("ws_connection_closed", "conn_id", connID)
	return nil
}

func (wb *WSBroadcaster) NotifyProgressUpdate(userID string, data map[string]interface{}) {
	event := bridge.NewUnifiedEvent(
		bridge.EventProgressUpdate,
		userID,
		bridge.ProtocolWebSocket,
		data,
	)
	wb.BroadcastToUser(userID, event)
}

func (wb *WSBroadcaster) NotifyLibraryUpdate(userID string, data map[string]interface{}) {
	event := bridge.NewUnifiedEvent(
		bridge.EventLibraryUpdate,
		userID,
		bridge.ProtocolWebSocket,
		data,
	)
	wb.BroadcastToUser(userID, event)
}
