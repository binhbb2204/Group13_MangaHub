package udp

import (
	"net"
	"sync"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

type Subscriber struct {
	UserID       string
	Addr         *net.UDPAddr
	EventTypes   []string
	RegisteredAt time.Time
	LastSeen     time.Time
}

type SubscriberManager struct {
	subscribers map[string][]*Subscriber
	addrToUser  map[string]string
	mu          sync.RWMutex
	log         *logger.Logger
	stopChan    chan struct{}
	stopped     bool
}

func NewSubscriberManager(log *logger.Logger) *SubscriberManager {
	return &SubscriberManager{
		subscribers: make(map[string][]*Subscriber),
		addrToUser:  make(map[string]string),
		log:         log,
		stopChan:    make(chan struct{}),
	}
}

func (sm *SubscriberManager) Subscribe(userID string, addr *net.UDPAddr, eventTypes []string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	addrKey := addr.String()
	now := time.Now()

	sub := &Subscriber{
		UserID:       userID,
		Addr:         addr,
		EventTypes:   eventTypes,
		RegisteredAt: now,
		LastSeen:     now,
	}

	sm.subscribers[userID] = append(sm.subscribers[userID], sub)
	sm.addrToUser[addrKey] = userID

	sm.log.Debug("subscriber_registered",
		"user_id", userID,
		"addr", addrKey,
		"event_types", eventTypes)
}

func (sm *SubscriberManager) Unsubscribe(addr *net.UDPAddr) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	addrKey := addr.String()
	userID, exists := sm.addrToUser[addrKey]
	if !exists {
		return
	}

	subs := sm.subscribers[userID]
	filtered := []*Subscriber{}
	for _, sub := range subs {
		if sub.Addr.String() != addrKey {
			filtered = append(filtered, sub)
		}
	}

	if len(filtered) > 0 {
		sm.subscribers[userID] = filtered
	} else {
		delete(sm.subscribers, userID)
	}

	delete(sm.addrToUser, addrKey)

	sm.log.Debug("subscriber_unregistered",
		"user_id", userID,
		"addr", addrKey)
}

func (sm *SubscriberManager) UpdateSubscription(addr *net.UDPAddr, eventTypes []string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	addrKey := addr.String()
	userID, exists := sm.addrToUser[addrKey]
	if !exists {
		return false
	}

	for _, sub := range sm.subscribers[userID] {
		if sub.Addr.String() == addrKey {
			sub.EventTypes = eventTypes
			sub.LastSeen = time.Now()
			sm.log.Debug("subscription_updated",
				"user_id", userID,
				"addr", addrKey,
				"event_types", eventTypes)
			return true
		}
	}

	return false
}

func (sm *SubscriberManager) Heartbeat(addr *net.UDPAddr) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	addrKey := addr.String()
	userID, exists := sm.addrToUser[addrKey]
	if !exists {
		return false
	}

	for _, sub := range sm.subscribers[userID] {
		if sub.Addr.String() == addrKey {
			sub.LastSeen = time.Now()
			return true
		}
	}

	return false
}

func (sm *SubscriberManager) GetSubscribers(userID, eventType string) []*Subscriber {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	subs := sm.subscribers[userID]
	filtered := []*Subscriber{}

	for _, sub := range subs {
		if sm.matchesEventType(sub, eventType) {
			filtered = append(filtered, sub)
		}
	}

	return filtered
}

func (sm *SubscriberManager) GetUserByAddr(addr *net.UDPAddr) (string, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	userID, exists := sm.addrToUser[addr.String()]
	return userID, exists
}

func (sm *SubscriberManager) GetSubscriberCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	total := 0
	for _, subs := range sm.subscribers {
		total += len(subs)
	}
	return total
}

func (sm *SubscriberManager) StartCleanup() {
	go sm.cleanupLoop()
}

func (sm *SubscriberManager) Stop() {
	if !sm.stopped {
		sm.stopped = true
		close(sm.stopChan)
	}
}

func (sm *SubscriberManager) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.cleanupStale()
		case <-sm.stopChan:
			return
		}
	}
}

func (sm *SubscriberManager) cleanupStale() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	timeout := 2 * time.Minute

	for userID, subs := range sm.subscribers {
		filtered := []*Subscriber{}
		for _, sub := range subs {
			if now.Sub(sub.LastSeen) <= timeout {
				filtered = append(filtered, sub)
			} else {
				delete(sm.addrToUser, sub.Addr.String())
				sm.log.Info("removed_stale_subscriber",
					"user_id", sub.UserID,
					"addr", sub.Addr.String(),
					"inactive_duration", now.Sub(sub.LastSeen).String())
			}
		}

		if len(filtered) > 0 {
			sm.subscribers[userID] = filtered
		} else {
			delete(sm.subscribers, userID)
		}
	}
}

func (sm *SubscriberManager) matchesEventType(sub *Subscriber, eventType string) bool {
	for _, et := range sub.EventTypes {
		if et == "all" || et == eventType {
			return true
		}
	}
	return false
}
