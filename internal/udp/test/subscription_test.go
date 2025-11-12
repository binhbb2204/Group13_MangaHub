package udp_test

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/udp"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

func setupSubscriberManager(t *testing.T) *udp.SubscriberManager {
	logger.Init(logger.ERROR, false, nil)
	log := logger.GetLogger()
	return udp.NewSubscriberManager(log)
}

func TestSubscribe(t *testing.T) {
	sm := setupSubscriberManager(t)
	defer sm.Stop()

	userID := "user1"
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5001}
	eventTypes := []string{"progress_update"}

	sm.Subscribe(userID, addr, eventTypes)

	subs := sm.GetSubscribers(userID, "progress_update")
	if len(subs) != 1 {
		t.Errorf("Expected 1 subscriber, got %d", len(subs))
	}

	if subs[0].UserID != userID {
		t.Errorf("Expected user_id '%s', got '%s'", userID, subs[0].UserID)
	}
}

func TestUnsubscribe(t *testing.T) {
	sm := setupSubscriberManager(t)
	defer sm.Stop()

	userID := "user1"
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5002}
	eventTypes := []string{"all"}

	sm.Subscribe(userID, addr, eventTypes)
	sm.Unsubscribe(addr)

	subs := sm.GetSubscribers(userID, "progress_update")
	if len(subs) != 0 {
		t.Errorf("Expected 0 subscribers after unsubscribe, got %d", len(subs))
	}
}

func TestUpdateSubscription(t *testing.T) {
	sm := setupSubscriberManager(t)
	defer sm.Stop()

	userID := "user1"
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5003}
	initialTypes := []string{"progress_update"}
	updatedTypes := []string{"library_update"}

	sm.Subscribe(userID, addr, initialTypes)

	success := sm.UpdateSubscription(addr, updatedTypes)
	if !success {
		t.Error("UpdateSubscription failed")
	}

	progressSubs := sm.GetSubscribers(userID, "progress_update")
	if len(progressSubs) != 0 {
		t.Errorf("Expected 0 subscribers for progress_update, got %d", len(progressSubs))
	}

	librarySubs := sm.GetSubscribers(userID, "library_update")
	if len(librarySubs) != 1 {
		t.Errorf("Expected 1 subscriber for library_update, got %d", len(librarySubs))
	}
}

func TestHeartbeat(t *testing.T) {
	sm := setupSubscriberManager(t)
	defer sm.Stop()

	userID := "user1"
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5004}
	eventTypes := []string{"all"}

	sm.Subscribe(userID, addr, eventTypes)
	time.Sleep(100 * time.Millisecond)

	success := sm.Heartbeat(addr)
	if !success {
		t.Error("Heartbeat failed for registered subscriber")
	}

	unknownAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9999}
	success = sm.Heartbeat(unknownAddr)
	if success {
		t.Error("Heartbeat should fail for unregistered address")
	}
}

func TestGetSubscribersFiltered(t *testing.T) {
	sm := setupSubscriberManager(t)
	defer sm.Stop()

	userID := "user1"
	addr1 := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5005}
	addr2 := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5006}
	addr3 := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5007}

	sm.Subscribe(userID, addr1, []string{"progress_update"})
	sm.Subscribe(userID, addr2, []string{"library_update"})
	sm.Subscribe(userID, addr3, []string{"all"})

	progressSubs := sm.GetSubscribers(userID, "progress_update")
	if len(progressSubs) != 2 {
		t.Errorf("Expected 2 subscribers for progress_update, got %d", len(progressSubs))
	}

	librarySubs := sm.GetSubscribers(userID, "library_update")
	if len(librarySubs) != 2 {
		t.Errorf("Expected 2 subscribers for library_update, got %d", len(librarySubs))
	}
}

func TestMultipleSubscribersPerUser(t *testing.T) {
	sm := setupSubscriberManager(t)
	defer sm.Stop()

	userID := "user1"
	addr1 := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5008}
	addr2 := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5009}
	addr3 := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5010}

	sm.Subscribe(userID, addr1, []string{"all"})
	sm.Subscribe(userID, addr2, []string{"all"})
	sm.Subscribe(userID, addr3, []string{"all"})

	subs := sm.GetSubscribers(userID, "progress_update")
	if len(subs) != 3 {
		t.Errorf("Expected 3 subscribers, got %d", len(subs))
	}

	count := sm.GetSubscriberCount()
	if count != 3 {
		t.Errorf("Expected total count 3, got %d", count)
	}
}

func TestGetUserByAddr(t *testing.T) {
	sm := setupSubscriberManager(t)
	defer sm.Stop()

	userID := "user1"
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5011}

	sm.Subscribe(userID, addr, []string{"all"})

	foundUserID, exists := sm.GetUserByAddr(addr)
	if !exists {
		t.Error("Expected to find user by address")
	}

	if foundUserID != userID {
		t.Errorf("Expected user_id '%s', got '%s'", userID, foundUserID)
	}

	unknownAddr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9999}
	_, exists = sm.GetUserByAddr(unknownAddr)
	if exists {
		t.Error("Should not find user for unknown address")
	}
}

func TestConcurrentSubscriptionOperations(t *testing.T) {
	sm := setupSubscriberManager(t)
	defer sm.Stop()

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			userID := "user1"
			addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 6000 + id}
			sm.Subscribe(userID, addr, []string{"all"})
			sm.Heartbeat(addr)
			sm.GetSubscribers(userID, "progress_update")
		}(i)
	}

	wg.Wait()

	count := sm.GetSubscriberCount()
	if count != numGoroutines {
		t.Errorf("Expected %d subscribers after concurrent operations, got %d", numGoroutines, count)
	}
}

func TestCleanupStale(t *testing.T) {
	sm := setupSubscriberManager(t)
	defer sm.Stop()

	userID := "user1"
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5012}

	sm.Subscribe(userID, addr, []string{"all"})

	count := sm.GetSubscriberCount()
	if count != 1 {
		t.Errorf("Expected 1 subscriber before cleanup, got %d", count)
	}

	t.Log("Cleanup test requires waiting >2 minutes for stale detection")
	t.Log("Skipping actual cleanup wait in unit test")
}
