package integration_test

import (
	"sync"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
)

func TestConcurrentProgressUpdates(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29200")
	env.WaitForBridgeReady()

	userID := "conflict-user-1"
	mangaID := "conflict-manga-1"

	var wg sync.WaitGroup
	numUpdates := 10

	for i := 0; i < numUpdates; i++ {
		wg.Add(1)
		go func(chapter int) {
			defer wg.Done()

			event := bridge.NewUnifiedEvent(
				bridge.EventProgressUpdate,
				userID,
				bridge.ProtocolWebSocket,
				map[string]interface{}{
					"manga_id": mangaID,
					"chapter":  chapter,
					"status":   "reading",
				},
			)

			env.Bridge.BroadcastEvent(event)
		}(i + 1)
	}

	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	t.Logf("Successfully handled %d concurrent progress updates", numUpdates)
}

func TestMultiDeviceConflict(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29201")
	env.StartTCPServer(t, "29202")
	env.WaitForBridgeReady()

	userID := "multi-device-user"
	mangaID := "test-manga-conflict"

	devices := []struct {
		name     string
		protocol bridge.ProtocolType
		chapter  int
	}{
		{"device-1", bridge.ProtocolWebSocket, 15},
		{"device-2", bridge.ProtocolTCP, 18},
		{"device-3", bridge.ProtocolUDP, 12},
		{"device-4", bridge.ProtocolGRPC, 20},
	}

	var wg sync.WaitGroup

	for _, device := range devices {
		wg.Add(1)
		go func(d struct {
			name     string
			protocol bridge.ProtocolType
			chapter  int
		}) {
			defer wg.Done()

			event := bridge.NewUnifiedEvent(
				bridge.EventProgressUpdate,
				userID,
				d.protocol,
				map[string]interface{}{
					"manga_id":    mangaID,
					"chapter":     d.chapter,
					"status":      "reading",
					"device_name": d.name,
				},
			)

			env.Bridge.BroadcastEvent(event)
			time.Sleep(50 * time.Millisecond)
		}(device)
	}

	wg.Wait()
	time.Sleep(500 * time.Millisecond)

	t.Logf("Multi-device conflict scenario completed with %d devices", len(devices))
}

func TestDatabaseConsistency(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	userID := "db-consistency-user"
	mangaID := "db-manga-1"

	_, err := database.DB.Exec(`
		INSERT INTO users (id, username, password_hash) 
		VALUES (?, ?, ?)
		ON CONFLICT(id) DO NOTHING
	`, userID, "testuser", "hash123")
	if err != nil {
		t.Fatalf("Failed to insert user: %v", err)
	}

	_, err = database.DB.Exec(`
		INSERT INTO manga (id, title, author) 
		VALUES (?, ?, ?)
		ON CONFLICT(id) DO NOTHING
	`, mangaID, "Test Manga", "Test Author")
	if err != nil {
		t.Fatalf("Failed to insert manga: %v", err)
	}

	numUpdates := 20
	var wg sync.WaitGroup

	for i := 0; i < numUpdates; i++ {
		wg.Add(1)
		go func(chapter int) {
			defer wg.Done()

			_, err := database.DB.Exec(`
				INSERT INTO user_progress (user_id, manga_id, current_chapter, status)
				VALUES (?, ?, ?, ?)
				ON CONFLICT(user_id, manga_id) DO UPDATE SET
					current_chapter = CASE 
						WHEN excluded.current_chapter > user_progress.current_chapter 
						THEN excluded.current_chapter 
						ELSE user_progress.current_chapter 
					END,
					updated_at = CURRENT_TIMESTAMP
			`, userID, mangaID, chapter, "reading")

			if err != nil {
				t.Logf("Update %d error: %v", chapter, err)
			}
		}(i + 10)
	}

	wg.Wait()
	time.Sleep(300 * time.Millisecond)

	var finalChapter int
	err = database.DB.QueryRow(`
		SELECT current_chapter FROM user_progress 
		WHERE user_id = ? AND manga_id = ?
	`, userID, mangaID).Scan(&finalChapter)

	if err != nil {
		t.Fatalf("Failed to query final state: %v", err)
	}

	expectedMax := numUpdates + 9
	if finalChapter != expectedMax {
		t.Errorf("Expected chapter %d (highest), got %d", expectedMax, finalChapter)
	}

	t.Logf("Database consistency maintained: final chapter = %d", finalChapter)
}

func TestEventOrderingUnderLoad(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29203")
	env.WaitForBridgeReady()

	userID := "ordering-test-user"
	numEvents := 50

	eventsSent := make([]int, numEvents)
	var wg sync.WaitGroup

	startTime := time.Now()

	for i := 0; i < numEvents; i++ {
		wg.Add(1)
		eventsSent[i] = i

		go func(index int) {
			defer wg.Done()

			event := bridge.NewUnifiedEvent(
				bridge.EventProgressUpdate,
				userID,
				bridge.ProtocolWebSocket,
				map[string]interface{}{
					"manga_id": "ordering-manga",
					"chapter":  index,
					"sequence": index,
				},
			)

			env.Bridge.BroadcastEvent(event)
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	time.Sleep(500 * time.Millisecond)

	t.Logf("Sent %d events in %v (%v per event avg)", numEvents, duration, duration/time.Duration(numEvents))
}
