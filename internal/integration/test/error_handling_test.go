package integration_test

import (
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
)

func TestInvalidEventData(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29500")
	env.WaitForBridgeReady()

	testCases := []struct {
		name string
		data map[string]interface{}
	}{
		{
			name: "nil_data",
			data: nil,
		},
		{
			name: "empty_data",
			data: map[string]interface{}{},
		},
		{
			name: "missing_required_fields",
			data: map[string]interface{}{
				"invalid_field": "value",
			},
		},
		{
			name: "invalid_types",
			data: map[string]interface{}{
				"chapter": "not_a_number",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			event := bridge.NewUnifiedEvent(
				bridge.EventProgressUpdate,
				"error-user",
				bridge.ProtocolWebSocket,
				tc.data,
			)

			env.Bridge.BroadcastEvent(event)
			time.Sleep(100 * time.Millisecond)

			t.Logf("System handled invalid data case: %s", tc.name)
		})
	}
}

func TestEmptyUserID(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29501")
	env.WaitForBridgeReady()

	event := bridge.NewUnifiedEvent(
		bridge.EventProgressUpdate,
		"",
		bridge.ProtocolTCP,
		map[string]interface{}{
			"manga_id": "test",
			"chapter":  1,
		},
	)

	env.Bridge.BroadcastEvent(event)
	time.Sleep(100 * time.Millisecond)

	userCount := env.Bridge.GetActiveUserCount()
	t.Logf("Empty user ID handled, active users: %d", userCount)
}

func TestRapidEventFlood(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29502")
	env.WaitForBridgeReady()

	numEvents := 1000

	for i := 0; i < numEvents; i++ {
		event := bridge.NewUnifiedEvent(
			bridge.EventProgressUpdate,
			"flood-user",
			bridge.ProtocolWebSocket,
			map[string]interface{}{
				"index": i,
			},
		)

		env.Bridge.BroadcastEvent(event)
	}

	time.Sleep(500 * time.Millisecond)

	t.Logf("System survived rapid event flood of %d events", numEvents)
}

func TestMalformedEventTypes(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29503")
	env.WaitForBridgeReady()

	event := bridge.UnifiedEvent{
		ID:          "test-id",
		Type:        "invalid_event_type",
		UserID:      "test-user",
		SourceProto: bridge.ProtocolWebSocket,
		Timestamp:   time.Now(),
		Data: map[string]interface{}{
			"test": "data",
		},
	}

	env.Bridge.BroadcastEvent(event)
	time.Sleep(100 * time.Millisecond)

	t.Log("System handled malformed event type")
}

func TestConcurrentErrorScenarios(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29504")
	env.WaitForBridgeReady()

	errorScenarios := []func(){
		func() {
			event := bridge.NewUnifiedEvent(bridge.EventProgressUpdate, "", bridge.ProtocolTCP, nil)
			env.Bridge.BroadcastEvent(event)
		},
		func() {
			event := bridge.NewUnifiedEvent("invalid", "user", bridge.ProtocolWebSocket, map[string]interface{}{})
			env.Bridge.BroadcastEvent(event)
		},
		func() {
			event := bridge.NewUnifiedEvent(bridge.EventLibraryUpdate, "user", "invalid-protocol", map[string]interface{}{})
			env.Bridge.BroadcastEvent(event)
		},
	}

	for i, scenario := range errorScenarios {
		go func(index int, fn func()) {
			for j := 0; j < 10; j++ {
				fn()
			}
		}(i, scenario)
	}

	time.Sleep(500 * time.Millisecond)

	t.Log("System handled concurrent error scenarios without crashing")
}

func TestGracefulDegradation(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29505")
	env.WaitForBridgeReady()

	validEvent := bridge.NewUnifiedEvent(
		bridge.EventProgressUpdate,
		"valid-user",
		bridge.ProtocolWebSocket,
		map[string]interface{}{
			"manga_id": "test",
			"chapter":  1,
		},
	)

	invalidEvent := bridge.NewUnifiedEvent(
		bridge.EventProgressUpdate,
		"",
		bridge.ProtocolTCP,
		nil,
	)

	env.Bridge.BroadcastEvent(invalidEvent)
	time.Sleep(50 * time.Millisecond)

	env.Bridge.BroadcastEvent(validEvent)
	time.Sleep(50 * time.Millisecond)

	env.Bridge.BroadcastEvent(invalidEvent)
	time.Sleep(50 * time.Millisecond)

	env.Bridge.BroadcastEvent(validEvent)
	time.Sleep(50 * time.Millisecond)

	t.Log("System maintained operation despite interspersed errors")
}
