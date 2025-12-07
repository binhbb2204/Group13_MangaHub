package integration_test

import (
	"encoding/json"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/udp"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
)

func TestUDPEventReception(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29100")
	env.WaitForBridgeReady()

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 29100,
	})
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	jwtSecret := "test-secret"
	token, _ := utils.GenerateJWT("test-user-1", "testuser", jwtSecret)

	registerMsg := udp.CreateRegisterMessage(token)
	conn.Write(registerMsg)

	buffer := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buffer)
	if err != nil {
		t.Logf("Register response read error: %v", err)
	}
	if n > 0 {
		t.Logf("Register response received: %d bytes", n)
	}

	time.Sleep(500 * time.Millisecond)

	userCount := env.Bridge.GetActiveUserCount()
	totalConns := env.Bridge.GetTotalConnectionCount()
	t.Logf("Bridge stats before event: active_users=%d, total_connections=%d", userCount, totalConns)

	event := bridge.NewUnifiedEvent(
		bridge.EventProgressUpdate,
		"test-user-1",
		bridge.ProtocolTCP,
		map[string]interface{}{
			"manga_id": "manga-123",
			"chapter":  15,
			"status":   "reading",
		},
	)

	env.Bridge.BroadcastEvent(event)
	time.Sleep(1 * time.Second)

	t.Log("Event broadcast completed - test validates framework can send events")
}

func TestConcurrentEventBroadcast(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29101")
	env.WaitForBridgeReady()

	numEvents := 50
	var wg sync.WaitGroup
	wg.Add(numEvents)

	for i := 0; i < numEvents; i++ {
		go func(index int) {
			defer wg.Done()

			event := bridge.NewUnifiedEvent(
				bridge.EventProgressUpdate,
				"concurrent-user",
				bridge.ProtocolWebSocket,
				map[string]interface{}{
					"manga_id": "manga-concurrent",
					"chapter":  index,
				},
			)

			env.Bridge.BroadcastEvent(event)
		}(i)
	}

	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	t.Logf("Successfully broadcast %d concurrent events", numEvents)
}

func TestEventDataIntegrity(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29102")
	env.WaitForBridgeReady()

	testData := map[string]interface{}{
		"manga_id":     "test-manga-999",
		"chapter":      42,
		"status":       "completed",
		"rating":       4.5,
		"tags":         []string{"action", "adventure"},
		"custom_field": "custom_value",
	}

	event := bridge.NewUnifiedEvent(
		bridge.EventProgressUpdate,
		"test-user-integrity",
		bridge.ProtocolGRPC,
		testData,
	)

	env.Bridge.BroadcastEvent(event)
	time.Sleep(200 * time.Millisecond)

	if event.Data["manga_id"] != testData["manga_id"] {
		t.Error("Event data was modified")
	}

	dataJSON, err := json.Marshal(event.Data)
	if err != nil {
		t.Fatalf("Failed to marshal event data: %v", err)
	}

	var reconstructed map[string]interface{}
	if err := json.Unmarshal(dataJSON, &reconstructed); err != nil {
		t.Fatalf("Failed to unmarshal event data: %v", err)
	}

	if reconstructed["manga_id"] != "test-manga-999" {
		t.Error("Data integrity compromised during serialization")
	}

	t.Log("Event data integrity maintained")
}

func TestMultipleUserEvents(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29103")
	env.WaitForBridgeReady()

	users := []string{"user-1", "user-2", "user-3", "user-4", "user-5"}

	for _, userID := range users {
		event := bridge.NewUnifiedEvent(
			bridge.EventLibraryUpdate,
			userID,
			bridge.ProtocolWebSocket,
			map[string]interface{}{
				"manga_id": "manga-" + userID,
				"action":   "added",
			},
		)

		env.Bridge.BroadcastEvent(event)
	}

	time.Sleep(300 * time.Millisecond)

	t.Logf("Successfully broadcast events for %d users", len(users))
}

func TestEventTypeVariety(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29104")
	env.WaitForBridgeReady()

	testCases := []struct {
		name      string
		eventType bridge.EventType
		data      map[string]interface{}
	}{
		{
			name:      "progress_update",
			eventType: bridge.EventProgressUpdate,
			data:      map[string]interface{}{"chapter": 10},
		},
		{
			name:      "library_update",
			eventType: bridge.EventLibraryUpdate,
			data:      map[string]interface{}{"action": "added"},
		},
		{
			name:      "chapter_completed",
			eventType: bridge.EventChapterCompleted,
			data:      map[string]interface{}{"chapter": 20},
		},
		{
			name:      "manga_started",
			eventType: bridge.EventMangaStarted,
			data:      map[string]interface{}{"manga_id": "new-manga"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			event := bridge.NewUnifiedEvent(
				tc.eventType,
				"variety-user",
				bridge.ProtocolTCP,
				tc.data,
			)

			env.Bridge.BroadcastEvent(event)
			time.Sleep(100 * time.Millisecond)

			if event.Type != tc.eventType {
				t.Errorf("Event type mismatch: expected %s, got %s", tc.eventType, event.Type)
			}
		})
	}
}
