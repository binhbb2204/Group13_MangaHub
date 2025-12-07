package integration_test

import (
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
)

func TestUserReadingProgressJourney(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29400")
	env.WaitForBridgeReady()

	userID := "journey-user-1"
	mangaID := "journey-manga-1"

	database.DB.Exec(`INSERT INTO users (id, username, password_hash) VALUES (?, ?, ?)`, userID, "journeyuser", "hash")
	database.DB.Exec(`INSERT INTO manga (id, title, author) VALUES (?, ?, ?)`, mangaID, "Journey Manga", "Author")

	steps := []struct {
		name    string
		chapter int
		status  string
	}{
		{"start_reading", 1, "reading"},
		{"continue_reading", 5, "reading"},
		{"progress_update", 10, "reading"},
		{"complete_chapter", 15, "reading"},
		{"finish_manga", 20, "completed"},
	}

	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			event := bridge.NewUnifiedEvent(
				bridge.EventProgressUpdate,
				userID,
				bridge.ProtocolWebSocket,
				map[string]interface{}{
					"manga_id": mangaID,
					"chapter":  step.chapter,
					"status":   step.status,
				},
			)

			env.Bridge.BroadcastEvent(event)
			time.Sleep(100 * time.Millisecond)

			var currentChapter int
			err := database.DB.QueryRow(`
				SELECT current_chapter FROM user_progress 
				WHERE user_id = ? AND manga_id = ?
			`, userID, mangaID).Scan(&currentChapter)

			if err == nil {
				t.Logf("Step '%s': chapter %d recorded in database", step.name, currentChapter)
			}
		})
	}

	t.Log("User reading journey completed successfully")
}

func TestLibraryManagementJourney(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29401")
	env.WaitForBridgeReady()

	userID := "library-user"

	actions := []struct {
		name    string
		mangaID string
		action  string
	}{
		{"add_manga_1", "lib-manga-1", "added"},
		{"add_manga_2", "lib-manga-2", "added"},
		{"add_manga_3", "lib-manga-3", "added"},
		{"remove_manga_1", "lib-manga-1", "removed"},
		{"update_manga_2", "lib-manga-2", "updated"},
	}

	for _, action := range actions {
		t.Run(action.name, func(t *testing.T) {
			event := bridge.NewUnifiedEvent(
				bridge.EventLibraryUpdate,
				userID,
				bridge.ProtocolTCP,
				map[string]interface{}{
					"manga_id": action.mangaID,
					"action":   action.action,
				},
			)

			env.Bridge.BroadcastEvent(event)
			time.Sleep(50 * time.Millisecond)

			t.Logf("Library action '%s' processed", action.name)
		})
	}

	t.Log("Library management journey completed")
}

func TestMultiDeviceSyncJourney(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29402")
	env.StartTCPServer(t, "29403")
	env.WaitForBridgeReady()

	userID := "sync-user"
	mangaID := "sync-manga"

	database.DB.Exec(`INSERT INTO users (id, username, password_hash) VALUES (?, ?, ?)`, userID, "syncuser", "hash")
	database.DB.Exec(`INSERT INTO manga (id, title, author) VALUES (?, ?, ?)`, mangaID, "Sync Manga", "Author")

	scenarios := []struct {
		device   string
		protocol bridge.ProtocolType
		chapter  int
	}{
		{"phone", bridge.ProtocolWebSocket, 5},
		{"tablet", bridge.ProtocolTCP, 8},
		{"desktop", bridge.ProtocolGRPC, 12},
		{"phone", bridge.ProtocolWebSocket, 15},
	}

	for i, scenario := range scenarios {
		t.Run(scenario.device+"_update_"+string(rune(i)), func(t *testing.T) {
			event := bridge.NewUnifiedEvent(
				bridge.EventProgressUpdate,
				userID,
				scenario.protocol,
				map[string]interface{}{
					"manga_id":    mangaID,
					"chapter":     scenario.chapter,
					"device_name": scenario.device,
					"status":      "reading",
				},
			)

			env.Bridge.BroadcastEvent(event)
			time.Sleep(100 * time.Millisecond)

			t.Logf("Device '%s' updated to chapter %d via %s", scenario.device, scenario.chapter, scenario.protocol)
		})
	}

	t.Log("Multi-device sync journey completed")
}

func TestCompleteUserSession(t *testing.T) {
	env := SetupTestEnvironment(t)
	defer env.Cleanup()

	env.StartUDPServer(t, "29404")
	env.WaitForBridgeReady()

	userID := "session-user"

	database.DB.Exec(`INSERT INTO users (id, username, password_hash) VALUES (?, ?, ?)`, userID, "sessionuser", "hash")

	t.Run("login", func(t *testing.T) {
		event := bridge.NewUnifiedEvent(
			bridge.EventDeviceConnected,
			userID,
			bridge.ProtocolWebSocket,
			map[string]interface{}{
				"device_type": "mobile",
			},
		)
		env.Bridge.BroadcastEvent(event)
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("browse_library", func(t *testing.T) {
		for i := 1; i <= 3; i++ {
			mangaID := "browse-manga-" + string(rune(i))
			database.DB.Exec(`INSERT INTO manga (id, title, author) VALUES (?, ?, ?)`, mangaID, "Manga "+string(rune(i)), "Author")

			event := bridge.NewUnifiedEvent(
				bridge.EventLibraryUpdate,
				userID,
				bridge.ProtocolWebSocket,
				map[string]interface{}{
					"manga_id": mangaID,
					"action":   "viewed",
				},
			)
			env.Bridge.BroadcastEvent(event)
		}
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("read_manga", func(t *testing.T) {
		event := bridge.NewUnifiedEvent(
			bridge.EventProgressUpdate,
			userID,
			bridge.ProtocolWebSocket,
			map[string]interface{}{
				"manga_id": "browse-manga-1",
				"chapter":  5,
				"status":   "reading",
			},
		)
		env.Bridge.BroadcastEvent(event)
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("logout", func(t *testing.T) {
		event := bridge.NewUnifiedEvent(
			bridge.EventDeviceDisconnected,
			userID,
			bridge.ProtocolWebSocket,
			map[string]interface{}{
				"device_type": "mobile",
			},
		)
		env.Bridge.BroadcastEvent(event)
		time.Sleep(50 * time.Millisecond)
	})

	t.Log("Complete user session journey completed")
}
