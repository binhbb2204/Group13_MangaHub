package bridge_test

import (
	"os"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

func TestUnifiedBridgeCreation(t *testing.T) {
	log := logger.New(logger.DEBUG, false, os.Stdout)
	ub := bridge.NewUnifiedBridge(log)

	if ub == nil {
		t.Fatal("expected unified bridge to be created")
	}

	if ub.GetActiveUserCount() != 0 {
		t.Errorf("expected 0 active users, got %d", ub.GetActiveUserCount())
	}
}

func TestRegisterProtocolClient(t *testing.T) {
	log := logger.New(logger.DEBUG, false, os.Stdout)
	ub := bridge.NewUnifiedBridge(log)
	ub.Start()
	defer ub.Stop()

	userID := "test_user_1"
	clientID := ub.RegisterProtocolClient(nil, userID, bridge.ProtocolTCP)

	if clientID == "" {
		t.Fatal("expected client ID to be returned")
	}

	if ub.GetActiveUserCount() != 1 {
		t.Errorf("expected 1 active user, got %d", ub.GetActiveUserCount())
	}

	if ub.GetTotalConnectionCount() != 1 {
		t.Errorf("expected 1 total connection, got %d", ub.GetTotalConnectionCount())
	}
}

func TestUnregisterProtocolClient(t *testing.T) {
	log := logger.New(logger.DEBUG, false, os.Stdout)
	ub := bridge.NewUnifiedBridge(log)
	ub.Start()
	defer ub.Stop()

	userID := "test_user_1"
	clientID := ub.RegisterProtocolClient(nil, userID, bridge.ProtocolTCP)

	ub.UnregisterProtocolClient(clientID, userID)

	if ub.GetActiveUserCount() != 0 {
		t.Errorf("expected 0 active users after unregister, got %d", ub.GetActiveUserCount())
	}
}

func TestMultipleProtocolClients(t *testing.T) {
	log := logger.New(logger.DEBUG, false, os.Stdout)
	ub := bridge.NewUnifiedBridge(log)
	ub.Start()
	defer ub.Stop()

	userID := "test_user_1"

	tcpID := ub.RegisterProtocolClient(nil, userID, bridge.ProtocolTCP)
	wsID := ub.RegisterProtocolClient(nil, userID, bridge.ProtocolWebSocket)
	grpcID := ub.RegisterProtocolClient(nil, userID, bridge.ProtocolGRPC)

	if ub.GetTotalConnectionCount() != 3 {
		t.Errorf("expected 3 connections, got %d", ub.GetTotalConnectionCount())
	}

	stats := ub.GetProtocolStats()
	if stats[bridge.ProtocolTCP] != 1 {
		t.Errorf("expected 1 TCP connection, got %d", stats[bridge.ProtocolTCP])
	}
	if stats[bridge.ProtocolWebSocket] != 1 {
		t.Errorf("expected 1 WebSocket connection, got %d", stats[bridge.ProtocolWebSocket])
	}
	if stats[bridge.ProtocolGRPC] != 1 {
		t.Errorf("expected 1 gRPC connection, got %d", stats[bridge.ProtocolGRPC])
	}

	ub.UnregisterProtocolClient(tcpID, userID)
	ub.UnregisterProtocolClient(wsID, userID)
	ub.UnregisterProtocolClient(grpcID, userID)

	if ub.GetActiveUserCount() != 0 {
		t.Errorf("expected 0 users after cleanup, got %d", ub.GetActiveUserCount())
	}
}

func TestBroadcastEvent(t *testing.T) {
	log := logger.New(logger.DEBUG, false, os.Stdout)
	ub := bridge.NewUnifiedBridge(log)
	ub.Start()
	defer ub.Stop()

	userID := "test_user_1"
	event := bridge.NewUnifiedEvent(
		bridge.EventProgressUpdate,
		userID,
		bridge.ProtocolTCP,
		map[string]interface{}{
			"manga_id": "123",
			"chapter":  45,
		},
	)

	ub.BroadcastEvent(event)
	time.Sleep(100 * time.Millisecond)
}

func TestMultipleUsers(t *testing.T) {
	log := logger.New(logger.DEBUG, false, os.Stdout)
	ub := bridge.NewUnifiedBridge(log)
	ub.Start()
	defer ub.Stop()

	user1 := "user_1"
	user2 := "user_2"
	user3 := "user_3"

	ub.RegisterProtocolClient(nil, user1, bridge.ProtocolTCP)
	ub.RegisterProtocolClient(nil, user2, bridge.ProtocolWebSocket)
	ub.RegisterProtocolClient(nil, user3, bridge.ProtocolGRPC)

	if ub.GetActiveUserCount() != 3 {
		t.Errorf("expected 3 active users, got %d", ub.GetActiveUserCount())
	}

	ub.RegisterProtocolClient(nil, user1, bridge.ProtocolWebSocket)

	if ub.GetTotalConnectionCount() != 4 {
		t.Errorf("expected 4 total connections, got %d", ub.GetTotalConnectionCount())
	}
}
