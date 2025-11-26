package test

import (
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/websocket"
)

func TestManagerRegisterClient(t *testing.T) {
	manager := websocket.NewManager()
	go manager.Run()

	manager.BroadcastMessage([]byte("test"))

	time.Sleep(50 * time.Millisecond)

	users := manager.GetActiveUsers()
	if len(users) != 0 {
		t.Errorf("Expected 0 users before registration, got %d", len(users))
	}
}

func TestManagerGetClient(t *testing.T) {
	manager := websocket.NewManager()
	go manager.Run()

	users := manager.GetActiveUsers()
	if len(users) != 0 {
		t.Errorf("Expected 0 active users, got %d", len(users))
	}

	_, found := manager.GetClient("non-existent-user")
	if found {
		t.Error("Expected to not find non-existent user")
	}
}

func TestManagerBroadcast(t *testing.T) {
	manager := websocket.NewManager()
	go manager.Run()

	testMsg := []byte("test message")
	manager.BroadcastMessage(testMsg)

	time.Sleep(50 * time.Millisecond)
}

func TestManagerSendToUser(t *testing.T) {
	manager := websocket.NewManager()
	go manager.Run()

	sent := manager.SendToUser("non-existent-user", []byte("test"))
	if sent {
		t.Error("Expected send to fail for non-existent user")
	}
}

func TestClientUpdateActivity(t *testing.T) {
	client := &websocket.Client{
		ID:         "test-user-1",
		Username:   "testuser",
		Send:       make(chan []byte, 256),
		LastActive: time.Now().Add(-1 * time.Hour),
	}

	oldTime := client.GetLastActive()
	time.Sleep(10 * time.Millisecond)
	client.UpdateActivity()
	newTime := client.GetLastActive()

	if !newTime.After(oldTime) {
		t.Error("Expected LastActive to be updated")
	}
}
