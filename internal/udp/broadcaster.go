package udp

import (
	"net"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

type Broadcaster struct {
	conn          *net.UDPConn
	subMgr        *SubscriberManager
	log           *logger.Logger
	unifiedBridge *bridge.UnifiedBridge
}

func NewBroadcaster(conn *net.UDPConn, subMgr *SubscriberManager, log *logger.Logger) *Broadcaster {
	return &Broadcaster{
		conn:   conn,
		subMgr: subMgr,
		log:    log,
	}
}

func (b *Broadcaster) SetBridge(ub *bridge.UnifiedBridge) {
	b.unifiedBridge = ub
	b.log.Info("udp_broadcaster_bridge_set")
}

func (b *Broadcaster) BroadcastToUser(userID string, event bridge.BroadcastEvent) {
	subscribers := b.subMgr.GetSubscribers(userID, event.EventType)

	if len(subscribers) == 0 {
		b.log.Debug("no_udp_subscribers",
			"user_id", userID,
			"event_type", event.EventType)
		return
	}

	messageBytes := CreateNotificationMessage(userID, event.EventType, event.Data)

	successCount := 0
	failCount := 0

	for _, sub := range subscribers {
		_, err := b.conn.WriteToUDP(messageBytes, sub.Addr)
		if err != nil {
			failCount++
			b.log.Warn("broadcast_failed",
				"user_id", userID,
				"addr", sub.Addr.String(),
				"error", err.Error())
		} else {
			successCount++
		}
	}

	b.log.Info("udp_broadcast_complete",
		"user_id", userID,
		"event_type", event.EventType,
		"success_count", successCount,
		"fail_count", failCount,
		"total_devices", len(subscribers))
}

func (b *Broadcaster) BroadcastUnifiedEvent(userID string, event bridge.UnifiedEvent) {
	subscribers := b.subMgr.GetSubscribers(userID, string(event.Type))

	if len(subscribers) == 0 {
		b.log.Debug("no_udp_subscribers",
			"user_id", userID,
			"event_type", event.Type)
		return
	}

	messageBytes := CreateNotificationMessage(userID, string(event.Type), event.Data)

	successCount := 0
	failCount := 0

	for _, sub := range subscribers {
		_, writeErr := b.conn.WriteToUDP(messageBytes, sub.Addr)
		if writeErr != nil {
			failCount++
			b.log.Warn("broadcast_failed",
				"user_id", userID,
				"addr", sub.Addr.String(),
				"error", writeErr.Error())
		} else {
			successCount++
		}
	}

	b.log.Info("udp_unified_broadcast_complete",
		"user_id", userID,
		"event_type", event.Type,
		"success_count", successCount,
		"fail_count", failCount,
		"total_devices", len(subscribers))
}

func (b *Broadcaster) BroadcastToAll(event bridge.BroadcastEvent) {
	b.log.Info("broadcasting_to_all", "event_type", event.EventType)

	// Gather all subscribers matching this event type across all users
	subs := b.subMgr.GetAllSubscribersForEvent(event.EventType)
	if len(subs) == 0 {
		b.log.Debug("no_udp_global_subscribers", "event_type", event.EventType)
		return
	}

	// Build message once
	messageBytes := CreateNotificationMessage("", event.EventType, event.Data)

	successCount := 0
	failCount := 0
	for _, sub := range subs {
		if _, err := b.conn.WriteToUDP(messageBytes, sub.Addr); err != nil {
			failCount++
			b.log.Warn("broadcast_all_failed", "addr", sub.Addr.String(), "error", err.Error())
		} else {
			successCount++
		}
	}

	b.log.Info("udp_broadcast_all_complete",
		"event_type", event.EventType,
		"success_count", successCount,
		"fail_count", failCount,
		"total_devices", len(subs))
}

func (b *Broadcaster) GetSubscriberCount(userID string) int {
	return len(b.subMgr.GetSubscribers(userID, "all"))
}
