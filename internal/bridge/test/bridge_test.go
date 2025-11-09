package bridge_test

import (
	"net"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

func TestNewBridge(t *testing.T) {
	logger.Init(logger.INFO, false, nil)
	br := bridge.NewBridge(logger.GetLogger())
	if br == nil {
		t.Fatal("NewBridge returned nil")
	}
}

func TestBridgeStartStop(t *testing.T) {
	logger.Init(logger.INFO, false, nil)
	br := bridge.NewBridge(logger.GetLogger())

	br.Start()
	time.Sleep(100 * time.Millisecond)

	if br.GetActiveUserCount() != 0 {
		t.Errorf("Expected 0 active users, got %d", br.GetActiveUserCount())
	}

	br.Stop()
	time.Sleep(100 * time.Millisecond)
}

func TestBridgeRegisterUnregister(t *testing.T) {
	logger.Init(logger.INFO, false, nil)
	br := bridge.NewBridge(logger.GetLogger())
	br.Start()
	defer br.Stop()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	conn1, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to create connection 1: %v", err)
	}
	defer conn1.Close()

	conn2, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to create connection 2: %v", err)
	}
	defer conn2.Close()

	br.RegisterTCPClient(conn1, "user1")
	if br.GetActiveUserCount() != 1 {
		t.Errorf("Expected 1 active user after first registration, got %d", br.GetActiveUserCount())
	}
	if br.GetTotalConnectionCount() != 1 {
		t.Errorf("Expected 1 total connection after first registration, got %d", br.GetTotalConnectionCount())
	}

	br.RegisterTCPClient(conn2, "user1")
	if br.GetActiveUserCount() != 1 {
		t.Errorf("Expected 1 active user after second registration (same user), got %d", br.GetActiveUserCount())
	}
	if br.GetTotalConnectionCount() != 2 {
		t.Errorf("Expected 2 total connections after second registration, got %d", br.GetTotalConnectionCount())
	}

	br.UnregisterTCPClient(conn1, "user1")
	if br.GetActiveUserCount() != 1 {
		t.Errorf("Expected 1 active user after first unregister, got %d", br.GetActiveUserCount())
	}
	if br.GetTotalConnectionCount() != 1 {
		t.Errorf("Expected 1 total connection after first unregister, got %d", br.GetTotalConnectionCount())
	}

	br.UnregisterTCPClient(conn2, "user1")
	if br.GetActiveUserCount() != 0 {
		t.Errorf("Expected 0 active users after all unregister, got %d", br.GetActiveUserCount())
	}
	if br.GetTotalConnectionCount() != 0 {
		t.Errorf("Expected 0 total connections after all unregister, got %d", br.GetTotalConnectionCount())
	}
}

func TestBridgeMultipleUsers(t *testing.T) {
	logger.Init(logger.INFO, false, nil)
	br := bridge.NewBridge(logger.GetLogger())
	br.Start()
	defer br.Stop()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	conn1, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to create connection 1: %v", err)
	}
	defer conn1.Close()

	conn2, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to create connection 2: %v", err)
	}
	defer conn2.Close()

	br.RegisterTCPClient(conn1, "user1")
	br.RegisterTCPClient(conn2, "user2")

	if br.GetActiveUserCount() != 2 {
		t.Errorf("Expected 2 active users, got %d", br.GetActiveUserCount())
	}
	if br.GetTotalConnectionCount() != 2 {
		t.Errorf("Expected 2 total connections, got %d", br.GetTotalConnectionCount())
	}

	br.UnregisterTCPClient(conn1, "user1")
	br.UnregisterTCPClient(conn2, "user2")

	if br.GetActiveUserCount() != 0 {
		t.Errorf("Expected 0 active users after cleanup, got %d", br.GetActiveUserCount())
	}
}

func TestBridgeNotifications(t *testing.T) {
	logger.Init(logger.INFO, false, nil)
	br := bridge.NewBridge(logger.GetLogger())
	br.Start()
	defer br.Stop()

	progressEvent := bridge.ProgressUpdateEvent{
		UserID:       "user1",
		MangaID:      "manga1",
		ChapterID:    5,
		Status:       "reading",
		LastReadDate: time.Now(),
	}

	br.NotifyProgressUpdate(progressEvent)
	time.Sleep(100 * time.Millisecond)

	libraryEvent := bridge.LibraryUpdateEvent{
		UserID:  "user1",
		MangaID: "manga2",
		Action:  "added",
	}

	br.NotifyLibraryUpdate(libraryEvent)
	time.Sleep(100 * time.Millisecond)
}
