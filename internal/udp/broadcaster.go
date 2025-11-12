package udp

import (
	"net"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

type Broadcaster struct {
	conn   *net.UDPConn
	subMgr *SubscriberManager
	log    *logger.Logger
}

func NewBroadcaster(conn *net.UDPConn, subMgr *SubscriberManager, log *logger.Logger) *Broadcaster {
	return &Broadcaster{
		conn:   conn,
		subMgr: subMgr,
		log:    log,
	}
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

func (b *Broadcaster) BroadcastToAll(event bridge.BroadcastEvent) {
	b.log.Info("broadcasting_to_all", "event_type", event.EventType)
}
