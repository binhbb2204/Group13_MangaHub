package integration_test

import (
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
)

func TestCrossProtocolEventPropagation(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29001")
	env.WaitForBridgeReady()

	event := bridge.NewUnifiedEvent(
		bridge.EventProgressUpdate,
		"test-user-1",
		bridge.ProtocolTCP,
		map[string]interface{}{
			"manga_id": "test-manga-1",
			"chapter":  5,
			"status":   "reading",
		},
	)

	env.Bridge.BroadcastEvent(event)

	time.Sleep(200 * time.Millisecond)

	if env.Bridge.GetActiveUserCount() == 0 {
		t.Log("No active users - event broadcasted successfully without registered clients")
	}
}

func TestMultipleProtocolsBroadcast(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29002")
	env.StartTCPServer(t, "29003")
	env.WaitForBridgeReady()

	testCases := []struct {
		name      string
		eventType bridge.EventType
		userID    string
		data      map[string]interface{}
	}{
		{
			name:      "progress_update",
			eventType: bridge.EventProgressUpdate,
			userID:    "user1",
			data: map[string]interface{}{
				"manga_id": "manga1",
				"chapter":  10,
			},
		},
		{
			name:      "library_update",
			eventType: bridge.EventLibraryUpdate,
			userID:    "user2",
			data: map[string]interface{}{
				"manga_id": "manga2",
				"action":   "added",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			event := bridge.NewUnifiedEvent(
				tc.eventType,
				tc.userID,
				bridge.ProtocolWebSocket,
				tc.data,
			)

			env.Bridge.BroadcastEvent(event)
			time.Sleep(100 * time.Millisecond)
		})
	}
}

func TestBridgeStatistics(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29004")
	env.StartTCPServer(t, "29005")
	env.WaitForBridgeReady()

	stats := env.Bridge.GetProtocolStats()

	if stats == nil {
		t.Error("Expected protocol stats, got nil")
	}

	totalConnections := env.Bridge.GetTotalConnectionCount()
	if totalConnections < 0 {
		t.Errorf("Invalid total connection count: %d", totalConnections)
	}

	t.Logf("Protocol stats: %v, Total connections: %d", stats, totalConnections)
}
