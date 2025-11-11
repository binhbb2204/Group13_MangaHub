package tcp

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

type HeartbeatConfig struct {
	Interval time.Duration
	Timeout  time.Duration
}

func DefaultHeartbeatConfig() HeartbeatConfig {
	return HeartbeatConfig{
		Interval: 30 * time.Second,
		Timeout:  90 * time.Second,
	}
}

type HeartbeatManager struct {
	config         HeartbeatConfig
	lastHeartbeats map[string]time.Time
	rttMeasures    map[string]time.Duration
	mu             sync.RWMutex
	log            *logger.Logger
	stopChan       chan struct{}
	stopped        bool
	stopMu         sync.Mutex
}

func NewHeartbeatManager(config HeartbeatConfig) *HeartbeatManager {
	return &HeartbeatManager{
		config:         config,
		lastHeartbeats: make(map[string]time.Time),
		rttMeasures:    make(map[string]time.Duration),
		log:            logger.WithContext("component", "heartbeat_manager"),
		stopChan:       make(chan struct{}),
	}
}

func (hm *HeartbeatManager) Start() {
	hm.log.Info("heartbeat_manager_started",
		"interval", hm.config.Interval.String(),
		"timeout", hm.config.Timeout.String())
	go hm.cleanupLoop()
}

func (hm *HeartbeatManager) Stop() {
	hm.stopMu.Lock()
	defer hm.stopMu.Unlock()

	if hm.stopped {
		return
	}

	hm.log.Info("heartbeat_manager_stopping")
	hm.stopped = true
	close(hm.stopChan)
}

func (hm *HeartbeatManager) RegisterClient(clientID string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.lastHeartbeats[clientID] = time.Now()
	hm.log.Debug("client_registered_for_heartbeat", "client_id", clientID)
}

func (hm *HeartbeatManager) UnregisterClient(clientID string) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	delete(hm.lastHeartbeats, clientID)
	delete(hm.rttMeasures, clientID)
	hm.log.Debug("client_unregistered_from_heartbeat", "client_id", clientID)
}

func (hm *HeartbeatManager) RecordHeartbeat(clientID string, rtt time.Duration) {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	hm.lastHeartbeats[clientID] = time.Now()
	hm.rttMeasures[clientID] = rtt
	hm.log.Debug("heartbeat_recorded",
		"client_id", clientID,
		"rtt", rtt.String())
}

func (hm *HeartbeatManager) GetLastHeartbeat(clientID string) (time.Time, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	t, exists := hm.lastHeartbeats[clientID]
	return t, exists
}

func (hm *HeartbeatManager) GetRTT(clientID string) (time.Duration, bool) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	rtt, exists := hm.rttMeasures[clientID]
	return rtt, exists
}

func (hm *HeartbeatManager) IsAlive(clientID string) bool {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	lastHeartbeat, exists := hm.lastHeartbeats[clientID]
	if !exists {
		return false
	}
	return time.Since(lastHeartbeat) < hm.config.Timeout
}

func (hm *HeartbeatManager) GetNetworkQuality(clientID string) string {
	rtt, exists := hm.GetRTT(clientID)
	if !exists {
		return "Unknown"
	}
	switch {
	case rtt < 50*time.Millisecond:
		return "Excellent"
	case rtt < 100*time.Millisecond:
		return "Good"
	case rtt < 200*time.Millisecond:
		return "Fair"
	case rtt < 500*time.Millisecond:
		return "Poor"
	default:
		return "Very Poor"
	}
}

func (hm *HeartbeatManager) cleanupLoop() {
	ticker := time.NewTicker(hm.config.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			hm.cleanupStaleConnections()
		case <-hm.stopChan:
			return
		}
	}
}

func (hm *HeartbeatManager) cleanupStaleConnections() {
	hm.mu.Lock()
	defer hm.mu.Unlock()
	now := time.Now()
	staleClients := make([]string, 0)
	for clientID, lastHeartbeat := range hm.lastHeartbeats {
		if now.Sub(lastHeartbeat) > hm.config.Timeout {
			staleClients = append(staleClients, clientID)
		}
	}
	for _, clientID := range staleClients {
		delete(hm.lastHeartbeats, clientID)
		delete(hm.rttMeasures, clientID)
		hm.log.Warn("heartbeat_timeout",
			"client_id", clientID,
			"timeout", hm.config.Timeout.String())
	}
	if len(staleClients) > 0 {
		hm.log.Info("stale_connections_cleaned",
			"count", len(staleClients))
	}
}

func StartHeartbeatForConnection(
	ctx context.Context,
	conn net.Conn,
	clientID string,
	interval time.Duration,
	hm *HeartbeatManager,
	log *logger.Logger,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	log.Debug("heartbeat_loop_started",
		"client_id", clientID,
		"interval", interval.String())
	for {
		select {
		case <-ctx.Done():
			log.Debug("heartbeat_loop_stopped", "client_id", clientID)
			return
		case <-ticker.C:
			sentAt := time.Now()
			_, err := conn.Write(CreateHeartbeatMessage())
			if err != nil {
				log.Warn("heartbeat_send_failed",
					"client_id", clientID,
					"error", err.Error())
				return
			}
			rtt := time.Since(sentAt)
			hm.RecordHeartbeat(clientID, rtt)
		}
	}
}
