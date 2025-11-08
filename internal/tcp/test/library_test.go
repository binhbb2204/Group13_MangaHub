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

func setupLibraryTestDB(t *testing.T) {
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
		VALUES ('manga-1', 'Test Manga 1', 'Author 1', 'ongoing', 100),
		       ('manga-2', 'Test Manga 2', 'Author 2', 'completed', 50)
	`)
	if err != nil {
		t.Fatalf("Failed to insert test manga: %v", err)
	}
}

func authenticateClient(t *testing.T, conn net.Conn) {
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
}

func TestAddToLibrary(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9200")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9200")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	authenticateClient(t, conn)

	addMsg := map[string]interface{}{
		"type": "add_to_library",
		"payload": map[string]interface{}{
			"manga_id": "manga-1",
			"status":   "reading",
		},
	}
	addJSON, _ := json.Marshal(addMsg)
	conn.Write(append(addJSON, '\n'))

	response := make([]byte, 1024)
	n, _ := conn.Read(response)
	if n == 0 {
		t.Fatal("No add response received")
	}

	var count int
	err = database.DB.QueryRow(`SELECT COUNT(*) FROM user_progress WHERE user_id = ? AND manga_id = ?`,
		"test-user-1", "manga-1").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 entry in library, got %d", count)
	}
}

func TestGetLibrary(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	_, err := database.DB.Exec(`
		INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at) 
		VALUES ('test-user-1', 'manga-1', 10, 'reading', datetime('now')),
		       ('test-user-1', 'manga-2', 50, 'completed', datetime('now'))
	`)
	if err != nil {
		t.Fatalf("Failed to insert test progress: %v", err)
	}

	var count int
	database.DB.QueryRow(`SELECT COUNT(*) FROM user_progress WHERE user_id = ?`, "test-user-1").Scan(&count)
	if count != 2 {
		t.Fatalf("Expected 2 progress entries before test, got %d", count)
	}

	time.Sleep(50 * time.Millisecond)

	server := tcp.NewServer("9201")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9201")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	authenticateClient(t, conn)

	getMsg := map[string]interface{}{
		"type":    "get_library",
		"payload": map[string]interface{}{},
	}
	getJSON, _ := json.Marshal(getMsg)
	conn.Write(append(getJSON, '\n'))

	response := make([]byte, 4096)
	n, _ := conn.Read(response)
	if n == 0 {
		t.Fatal("No library response received")
	}

	responseStr := string(response[:n])
	if !contains(responseStr, "manga-1") || !contains(responseStr, "manga-2") {
		t.Errorf("Expected library to contain both manga, got: %s", responseStr)
	}
}

func TestGetProgress(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	_, err := database.DB.Exec(`
		INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at) 
		VALUES ('test-user-1', 'manga-1', 42, 'reading', datetime('now'))
	`)
	if err != nil {
		t.Fatalf("Failed to insert test progress: %v", err)
	}

	server := tcp.NewServer("9202")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9202")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	authenticateClient(t, conn)

	getMsg := map[string]interface{}{
		"type": "get_progress",
		"payload": map[string]interface{}{
			"manga_id": "manga-1",
		},
	}
	getJSON, _ := json.Marshal(getMsg)
	conn.Write(append(getJSON, '\n'))

	response := make([]byte, 1024)
	n, _ := conn.Read(response)
	if n == 0 {
		t.Fatal("No progress response received")
	}

	responseStr := string(response[:n])
	if !contains(responseStr, "42") || !contains(responseStr, "reading") {
		t.Errorf("Expected progress with chapter 42 and status reading, got: %s", responseStr)
	}
}

func TestRemoveFromLibrary(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	_, err := database.DB.Exec(`
		INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at) 
		VALUES ('test-user-1', 'manga-1', 10, 'reading', datetime('now'))
	`)
	if err != nil {
		t.Fatalf("Failed to insert test progress: %v", err)
	}

	server := tcp.NewServer("9203")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9203")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	authenticateClient(t, conn)

	removeMsg := map[string]interface{}{
		"type": "remove_from_library",
		"payload": map[string]interface{}{
			"manga_id": "manga-1",
		},
	}
	removeJSON, _ := json.Marshal(removeMsg)
	conn.Write(append(removeJSON, '\n'))

	response := make([]byte, 1024)
	n, _ := conn.Read(response)
	if n == 0 {
		t.Fatal("No remove response received")
	}

	var count int
	err = database.DB.QueryRow(`SELECT COUNT(*) FROM user_progress WHERE user_id = ? AND manga_id = ?`,
		"test-user-1", "manga-1").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected 0 entries in library after removal, got %d", count)
	}
}

func TestAddToLibraryMangaNotFound(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9204")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9204")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	authenticateClient(t, conn)

	addMsg := map[string]interface{}{
		"type": "add_to_library",
		"payload": map[string]interface{}{
			"manga_id": "non-existent",
			"status":   "reading",
		},
	}
	addJSON, _ := json.Marshal(addMsg)
	conn.Write(append(addJSON, '\n'))

	response := make([]byte, 1024)
	n, _ := conn.Read(response)
	responseStr := string(response[:n])

	if !contains(responseStr, "error") || !contains(responseStr, "Manga not found") {
		t.Errorf("Expected 'Manga not found' error, got: %s", responseStr)
	}
}
