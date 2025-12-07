package bridge

import (
	"sync"
	"time"
)

type ConnectionHealth struct {
	mu            sync.RWMutex
	healthChecks  map[string]*ClientHealth
	checkInterval time.Duration
	timeout       time.Duration
	stopChan      chan struct{}
	logger        interface{ Info(string, ...interface{}) }
}

type ClientHealth struct {
	LastSeen     time.Time
	MissedChecks int
	IsHealthy    bool
	mu           sync.RWMutex
}

func NewConnectionHealth(checkInterval, timeout time.Duration, logger interface{ Info(string, ...interface{}) }) *ConnectionHealth {
	return &ConnectionHealth{
		healthChecks:  make(map[string]*ClientHealth),
		checkInterval: checkInterval,
		timeout:       timeout,
		stopChan:      make(chan struct{}),
		logger:        logger,
	}
}

func (ch *ConnectionHealth) Start() {
	go ch.runHealthChecks()
}

func (ch *ConnectionHealth) Stop() {
	close(ch.stopChan)
}

func (ch *ConnectionHealth) RegisterClient(clientID string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()

	ch.healthChecks[clientID] = &ClientHealth{
		LastSeen:  time.Now(),
		IsHealthy: true,
	}
}

func (ch *ConnectionHealth) UnregisterClient(clientID string) {
	ch.mu.Lock()
	defer ch.mu.Unlock()
	delete(ch.healthChecks, clientID)
}

func (ch *ConnectionHealth) UpdateActivity(clientID string) {
	ch.mu.RLock()
	health, exists := ch.healthChecks[clientID]
	ch.mu.RUnlock()

	if exists {
		health.mu.Lock()
		health.LastSeen = time.Now()
		health.MissedChecks = 0
		health.IsHealthy = true
		health.mu.Unlock()
	}
}

func (ch *ConnectionHealth) runHealthChecks() {
	ticker := time.NewTicker(ch.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ch.performHealthCheck()
		case <-ch.stopChan:
			return
		}
	}
}

func (ch *ConnectionHealth) performHealthCheck() {
	ch.mu.RLock()
	clients := make(map[string]*ClientHealth, len(ch.healthChecks))
	for id, health := range ch.healthChecks {
		clients[id] = health
	}
	ch.mu.RUnlock()

	now := time.Now()
	unhealthyClients := []string{}

	for clientID, health := range clients {
		health.mu.Lock()
		if now.Sub(health.LastSeen) > ch.timeout {
			health.MissedChecks++
			if health.MissedChecks >= 3 {
				health.IsHealthy = false
				unhealthyClients = append(unhealthyClients, clientID)
			}
		}
		health.mu.Unlock()
	}

	if len(unhealthyClients) > 0 && ch.logger != nil {
		ch.logger.Info("unhealthy_clients_detected", "count", len(unhealthyClients))
	}
}

func (ch *ConnectionHealth) IsClientHealthy(clientID string) bool {
	ch.mu.RLock()
	health, exists := ch.healthChecks[clientID]
	ch.mu.RUnlock()

	if !exists {
		return false
	}

	health.mu.RLock()
	defer health.mu.RUnlock()
	return health.IsHealthy
}

func (ch *ConnectionHealth) GetHealthyClientCount() int {
	ch.mu.RLock()
	defer ch.mu.RUnlock()

	count := 0
	for _, health := range ch.healthChecks {
		health.mu.RLock()
		if health.IsHealthy {
			count++
		}
		health.mu.RUnlock()
	}
	return count
}
