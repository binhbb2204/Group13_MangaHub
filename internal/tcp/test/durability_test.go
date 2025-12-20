package tcp_test

import (
	"encoding/json"
	"net"
	"os"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/tcp"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
)

func TestDurabilityDatabasePersistence(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("0", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", server.Address())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, _ := utils.GenerateJWT("test-user-1", "testuser", "user", jwtSecret)

	authMsg := map[string]interface{}{
		"type":    "auth",
		"payload": map[string]string{"token": token},
	}
	authJSON, _ := json.Marshal(authMsg)
	conn.Write(append(authJSON, '\n'))

	response := make([]byte, 1024)
	conn.Read(response)

	syncMsg := map[string]interface{}{
		"type": "sync_progress",
		"payload": map[string]interface{}{
			"user_id":         "test-user-1",
			"manga_id":        "manga-1",
			"current_chapter": 99,
			"status":          "reading",
		},
	}
	syncJSON, _ := json.Marshal(syncMsg)
	conn.Write(append(syncJSON, '\n'))
	conn.Read(response)

	conn.Close()
	time.Sleep(100 * time.Millisecond)

	var chapter int
	var status string
	err = database.DB.QueryRow(`SELECT current_chapter, status FROM user_progress WHERE user_id = ? AND manga_id = ?`,
		"test-user-1", "manga-1").Scan(&chapter, &status)
	if err != nil {
		t.Fatalf("Data not persisted to database: %v", err)
	}

	if chapter != 99 {
		t.Errorf("Expected chapter 99, got %d", chapter)
	}
	if status != "reading" {
		t.Errorf("Expected status 'reading', got '%s'", status)
	}

	t.Logf("Durability: Data persisted correctly (chapter=%d, status=%s)", chapter, status)
}

func TestDurabilityDataConsistency(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("0", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", server.Address())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, _ := utils.GenerateJWT("test-user-1", "testuser", "user", jwtSecret)

	authMsg := map[string]interface{}{
		"type":    "auth",
		"payload": map[string]string{"token": token},
	}
	authJSON, _ := json.Marshal(authMsg)
	conn.Write(append(authJSON, '\n'))

	response := make([]byte, 1024)
	conn.Read(response)

	for i := 1; i <= 5; i++ {
		syncMsg := map[string]interface{}{
			"type": "sync_progress",
			"payload": map[string]interface{}{
				"user_id":         "test-user-1",
				"manga_id":        "manga-1",
				"current_chapter": i * 10,
				"status":          "reading",
			},
		}
		syncJSON, _ := json.Marshal(syncMsg)
		conn.Write(append(syncJSON, '\n'))
		conn.Read(response)
		time.Sleep(10 * time.Millisecond)
	}

	var chapter int
	err = database.DB.QueryRow(`SELECT current_chapter FROM user_progress WHERE user_id = ? AND manga_id = ?`,
		"test-user-1", "manga-1").Scan(&chapter)
	if err != nil {
		t.Fatalf("Failed to query final state: %v", err)
	}

	if chapter != 50 {
		t.Errorf("Data consistency violation: expected final chapter 50, got %d", chapter)
	}

	t.Logf("Durability: Data consistency maintained across %d updates (final chapter=%d)", 5, chapter)
}

func TestDurabilityServerRestart(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("0", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	_, port, _ := net.SplitHostPort(server.Address())
	address := "localhost:" + port

	conn, err := net.Dial("tcp", address)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, _ := utils.GenerateJWT("test-user-1", "testuser", "user", jwtSecret)

	authMsg := map[string]interface{}{
		"type":    "auth",
		"payload": map[string]string{"token": token},
	}
	authJSON, _ := json.Marshal(authMsg)
	conn.Write(append(authJSON, '\n'))

	response := make([]byte, 1024)
	conn.Read(response)

	syncMsg := map[string]interface{}{
		"type": "sync_progress",
		"payload": map[string]interface{}{
			"user_id":         "test-user-1",
			"manga_id":        "manga-1",
			"current_chapter": 75,
			"status":          "completed",
		},
	}
	syncJSON, _ := json.Marshal(syncMsg)
	conn.Write(append(syncJSON, '\n'))
	conn.Read(response)
	conn.Close()

	server.Stop()
	time.Sleep(200 * time.Millisecond)

	// Restart server on the SAME port to simulate restart
	// Note: In a real dynamic port scenario, we can't easily guarantee the same port.
	// However, for the purpose of testing persistence, the port doesn't matter as long as the DB is the same.
	// But the test logic below tries to reconnect to "localhost:9402".
	// We should use a new dynamic port for the restarted server and connect to that.

	server = tcp.NewServer("0", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to restart server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn2, err := net.Dial("tcp", server.Address())
	if err != nil {
		t.Fatalf("Failed to reconnect after restart: %v", err)
	}
	defer conn2.Close()

	token2, _ := utils.GenerateJWT("test-user-1", "testuser", "user", jwtSecret)
	authMsg2 := map[string]interface{}{
		"type":    "auth",
		"payload": map[string]string{"token": token2},
	}
	authJSON2, _ := json.Marshal(authMsg2)
	conn2.Write(append(authJSON2, '\n'))
	conn2.Read(response)

	getMsg := map[string]interface{}{
		"type": "get_progress",
		"payload": map[string]interface{}{
			"manga_id": "manga-1",
		},
	}
	getJSON, _ := json.Marshal(getMsg)
	conn2.Write(append(getJSON, '\n'))

	n, _ := conn2.Read(response)
	responseStr := string(response[:n])

	if !contains(responseStr, "75") || !contains(responseStr, "completed") {
		t.Errorf("Data not preserved after server restart: %s", responseStr)
	}

	t.Logf("Durability: Data survived server restart")
}

func TestDurabilityTransactionRollback(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("0", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", server.Address())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, _ := utils.GenerateJWT("test-user-1", "testuser", "user", jwtSecret)

	authMsg := map[string]interface{}{
		"type":    "auth",
		"payload": map[string]string{"token": token},
	}
	authJSON, _ := json.Marshal(authMsg)
	conn.Write(append(authJSON, '\n'))

	response := make([]byte, 1024)
	conn.Read(response)

	syncMsg := map[string]interface{}{
		"type": "sync_progress",
		"payload": map[string]interface{}{
			"user_id":         "test-user-1",
			"manga_id":        "non-existent-manga",
			"current_chapter": 10,
			"status":          "reading",
		},
	}
	syncJSON, _ := json.Marshal(syncMsg)
	conn.Write(append(syncJSON, '\n'))
	n, _ := conn.Read(response)

	responseStr := string(response[:n])
	if !contains(responseStr, "error") {
		t.Errorf("Expected error for invalid manga, got: %s", responseStr)
	}

	var count int
	database.DB.QueryRow(`SELECT COUNT(*) FROM user_progress WHERE user_id = ? AND manga_id = ?`,
		"test-user-1", "non-existent-manga").Scan(&count)

	if count != 0 {
		t.Errorf("Invalid data was persisted (count=%d), transaction not rolled back", count)
	}

	t.Logf("Durability: Invalid operations correctly rejected, no data corruption")
}
