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

func setupTestDB(t *testing.T) string {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	if err := database.InitDatabase(dbPath); err != nil {
		t.Fatalf("Failed to init test database: %v", err)
	}

	_, err := database.DB.Exec(`
		INSERT INTO users (id, username, email, password_hash) 
		VALUES ('test-user-1', 'testuser', 'test@example.com', 'hash123')
	`)
	if err != nil {
		t.Fatalf("Failed to insert test user: %v", err)
	}

	_, err = database.DB.Exec(`
		INSERT INTO manga (id, title, author, status, total_chapters) 
		VALUES ('manga-1', 'Test Manga', 'Author', 'ongoing', 100)
	`)
	if err != nil {
		t.Fatalf("Failed to insert test manga: %v", err)
	}

	return dbPath
}

func TestSyncProgressWithDatabase(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9100", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9100")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, err := utils.GenerateJWT("test-user-1", "testuser", jwtSecret)
	if err != nil {
		t.Fatalf("Failed to generate JWT: %v", err)
	}

	authMsg := map[string]interface{}{
		"type":    "auth",
		"payload": map[string]string{"token": token},
	}
	authJSON, _ := json.Marshal(authMsg)
	conn.Write(append(authJSON, '\n'))

	response := make([]byte, 1024)
	n, _ := conn.Read(response)
	if n == 0 {
		t.Fatal("No auth response received")
	}

	syncMsg := map[string]interface{}{
		"type": "sync_progress",
		"payload": map[string]interface{}{
			"user_id":         "test-user-1",
			"manga_id":        "manga-1",
			"current_chapter": 42,
			"status":          "reading",
		},
	}
	syncJSON, _ := json.Marshal(syncMsg)
	conn.Write(append(syncJSON, '\n'))

	n, _ = conn.Read(response)
	if n == 0 {
		t.Fatal("No sync response received")
	}

	var progress struct {
		CurrentChapter int
		Status         string
	}
	err = database.DB.QueryRow(
		`SELECT current_chapter, status FROM user_progress WHERE user_id = ? AND manga_id = ?`,
		"test-user-1", "manga-1",
	).Scan(&progress.CurrentChapter, &progress.Status)

	if err != nil {
		t.Fatalf("Failed to query synced progress: %v", err)
	}

	if progress.CurrentChapter != 42 {
		t.Errorf("Expected chapter 42, got %d", progress.CurrentChapter)
	}
	if progress.Status != "reading" {
		t.Errorf("Expected status 'reading', got '%s'", progress.Status)
	}
}

func TestSyncProgressMangaNotFound(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9101", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9101")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, err := utils.GenerateJWT("test-user-1", "testuser", jwtSecret)
	if err != nil {
		t.Fatalf("Failed to generate JWT: %v", err)
	}

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
			"manga_id":        "non-existent",
			"current_chapter": 10,
			"status":          "reading",
		},
	}
	syncJSON, _ := json.Marshal(syncMsg)
	conn.Write(append(syncJSON, '\n'))

	n, _ := conn.Read(response)
	responseStr := string(response[:n])

	if !contains(responseStr, "error") || !contains(responseStr, "Manga not found") {
		t.Errorf("Expected 'Manga not found' error, got: %s", responseStr)
	}
}

func TestSyncProgressInvalidStatus(t *testing.T) {
	setupTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9102", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9102")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, err := utils.GenerateJWT("test-user-1", "testuser", jwtSecret)
	if err != nil {
		t.Fatalf("Failed to generate JWT: %v", err)
	}

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
			"current_chapter": 10,
			"status":          "invalid_status",
		},
	}
	syncJSON, _ := json.Marshal(syncMsg)
	conn.Write(append(syncJSON, '\n'))

	n, _ := conn.Read(response)
	responseStr := string(response[:n])

	if !contains(responseStr, "error") || !contains(responseStr, "Invalid status") {
		t.Errorf("Expected 'Invalid status' error, got: %s", responseStr)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || (len(s) >= len(substr) && searchSubstring(s, substr)))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
