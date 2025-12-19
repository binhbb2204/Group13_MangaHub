package bridge

import "time"

type GRPCBroadcaster interface {
	BroadcastToUser(userID string, event UnifiedEvent)
	SendToStream(streamID string, event UnifiedEvent) error
	GetActiveStreams(userID string) []string
}

type WebSocketBroadcaster interface {
	SendToConnection(connID string, event UnifiedEvent) error
	BroadcastToUser(userID string, event UnifiedEvent)
	GetActiveConnections(userID string) []string
	CloseConnection(connID string) error
}

type UDPBroadcaster interface {
	BroadcastUnifiedEvent(userID string, event UnifiedEvent)
	GetSubscriberCount(userID string) int
}

type EventMetadata struct {
	RequestID   string    `json:"request_id,omitempty"`
	Priority    int       `json:"priority"`
	TTL         int       `json:"ttl"`
	Timestamp   time.Time `json:"timestamp"`
	Source      string    `json:"source"`
	Correlation string    `json:"correlation_id,omitempty"`
}

const (
	EventProgressUpdate     EventType = "progress_update"
	EventLibraryUpdate      EventType = "library_update"
	EventUserMessage        EventType = "user_message"
	EventChapterCompleted   EventType = "chapter_completed"
	EventMangaStarted       EventType = "manga_started"
	EventLibraryAdd         EventType = "library_add"
	EventLibraryRemove      EventType = "library_remove"
	EventStatusChange       EventType = "status_change"
	EventSyncRequest        EventType = "sync_request"
	EventSyncComplete       EventType = "sync_complete"
	EventConflictDetected   EventType = "conflict_detected"
	EventDeviceConnected    EventType = "device_connected"
	EventDeviceDisconnected EventType = "device_disconnected"
	EventSessionExpired     EventType = "session_expired"
	EventHealthCheck        EventType = "health_check"
	EventMetricsUpdate      EventType = "metrics_update"
	EventChapterRelease     EventType = "chapter_release"
)

type UnifiedEvent struct {
	ID          string                 `json:"id"`
	Type        EventType              `json:"type"`
	UserID      string                 `json:"user_id"`
	SourceProto ProtocolType           `json:"source_protocol"`
	Timestamp   time.Time              `json:"timestamp"`
	Data        map[string]interface{} `json:"data"`
	Metadata    EventMetadata          `json:"metadata"`
}

func NewUnifiedEvent(eventType EventType, userID string, sourceProto ProtocolType, data map[string]interface{}) UnifiedEvent {
	return UnifiedEvent{
		ID:          generateEventID(),
		Type:        eventType,
		UserID:      userID,
		SourceProto: sourceProto,
		Timestamp:   time.Now(),
		Data:        data,
		Metadata: EventMetadata{
			Priority:  0,
			TTL:       60,
			Timestamp: time.Now(),
			Source:    string(sourceProto),
		},
	}
}

func generateEventID() string {
	return time.Now().Format("20060102150405") + "_" + randomString(8)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[i%len(letters)]
	}
	return string(b)
}
