package discovery

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

type Announcement struct {
	LocalIP   string            `json:"local_ip"`
	Services  map[string]string `json:"services"`
	Timestamp time.Time         `json:"timestamp"`
}

type Broadcaster struct {
	announcement Announcement
	mu           sync.RWMutex
	stopCh       chan struct{}
}

func NewBroadcaster(localIP string, services map[string]string) *Broadcaster {
	return &Broadcaster{
		announcement: Announcement{
			LocalIP:   localIP,
			Services:  services,
			Timestamp: time.Now(),
		},
		stopCh: make(chan struct{}),
	}
}

func (b *Broadcaster) Start() {
	go b.broadcastLoop()
}

func (b *Broadcaster) Stop() {
	close(b.stopCh)
}

func (b *Broadcaster) GetAnnouncement() Announcement {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.announcement
}

func (b *Broadcaster) broadcastLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.broadcast()
		case <-b.stopCh:
			return
		}
	}
}

func (b *Broadcaster) broadcast() {
	conn, err := net.Dial("udp", "255.255.255.255:9099")
	if err != nil {
		logger.GetLogger().Error("broadcast_dial_failed", "error", err.Error())
		return
	}
	defer conn.Close()

	b.mu.Lock()
	b.announcement.Timestamp = time.Now()
	data, _ := json.Marshal(b.announcement)
	b.mu.Unlock()

	message := fmt.Sprintf("MANGAHUB:%s", string(data))
	_, err = conn.Write([]byte(message))
	if err != nil {
		logger.GetLogger().Error("broadcast_write_failed", "error", err.Error())
	}
}
