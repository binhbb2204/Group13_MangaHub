package grpc

import (
	"sync"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

type StreamConnection struct {
	UserID string
	Stream interface{}
	Active bool
}

type GRPCBroadcaster struct {
	logger  *logger.Logger
	bridge  *bridge.UnifiedBridge
	streams map[string][]*StreamConnection
	mu      sync.RWMutex
}

func NewGRPCBroadcaster(log *logger.Logger) *GRPCBroadcaster {
	return &GRPCBroadcaster{
		logger:  log,
		streams: make(map[string][]*StreamConnection),
	}
}

func (gb *GRPCBroadcaster) SetBridge(b *bridge.UnifiedBridge) {
	gb.bridge = b
	gb.logger.Info("grpc_broadcaster_bridge_set")
}

func (gb *GRPCBroadcaster) RegisterStream(streamID string, userID string, stream interface{}) {
	gb.mu.Lock()
	defer gb.mu.Unlock()

	conn := &StreamConnection{
		UserID: userID,
		Stream: stream,
		Active: true,
	}

	gb.streams[streamID] = append(gb.streams[streamID], conn)
	gb.logger.Info("grpc_stream_registered", "stream_id", streamID, "user_id", userID)
}

func (gb *GRPCBroadcaster) UnregisterStream(streamID string) {
	gb.mu.Lock()
	defer gb.mu.Unlock()

	delete(gb.streams, streamID)
	gb.logger.Info("grpc_stream_unregistered", "stream_id", streamID)
}

func (gb *GRPCBroadcaster) BroadcastToUser(userID string, event bridge.UnifiedEvent) {
	gb.mu.RLock()
	defer gb.mu.RUnlock()

	count := 0
	for _, connections := range gb.streams {
		for _, conn := range connections {
			if conn.UserID == userID && conn.Active {
				count++
				gb.logger.Debug("grpc_event_queued", "user_id", userID, "event_type", event.Type)
			}
		}
	}

	if count == 0 {
		gb.logger.Debug("grpc_no_active_streams", "user_id", userID)
	}
}

func (gb *GRPCBroadcaster) SendToStream(streamID string, event bridge.UnifiedEvent) error {
	gb.mu.RLock()
	connections, ok := gb.streams[streamID]
	gb.mu.RUnlock()

	if !ok || len(connections) == 0 {
		gb.logger.Warn("grpc_stream_not_found", "stream_id", streamID)
		return nil
	}

	gb.logger.Debug("grpc_event_sent", "stream_id", streamID, "event_type", event.Type)
	return nil
}

func (gb *GRPCBroadcaster) GetActiveStreams(userID string) []string {
	gb.mu.RLock()
	defer gb.mu.RUnlock()

	streams := make([]string, 0)
	for streamID, connections := range gb.streams {
		for _, conn := range connections {
			if conn.UserID == userID && conn.Active {
				streams = append(streams, streamID)
				break
			}
		}
	}
	return streams
}

func (gb *GRPCBroadcaster) GetStreamCount() int {
	gb.mu.RLock()
	defer gb.mu.RUnlock()
	return len(gb.streams)
}

func (gb *GRPCBroadcaster) NotifyProgressUpdate(userID string, data map[string]interface{}) {
	event := bridge.NewUnifiedEvent(
		bridge.EventProgressUpdate,
		userID,
		bridge.ProtocolGRPC,
		data,
	)
	gb.BroadcastToUser(userID, event)
}

func (gb *GRPCBroadcaster) NotifyLibraryUpdate(userID string, data map[string]interface{}) {
	event := bridge.NewUnifiedEvent(
		bridge.EventLibraryUpdate,
		userID,
		bridge.ProtocolGRPC,
		data,
	)
	gb.BroadcastToUser(userID, event)
}
