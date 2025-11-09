package tcp_test

import (
	"encoding/json"
	"net"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/tcp"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
)

func TestReliabilityInvalidAuthToken(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9800", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9800")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	invalidTokens := []string{
		"invalid-token",
		"",
		"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.invalid.signature",
		"malformed-jwt-token",
	}

	for _, token := range invalidTokens {
		authMsg := map[string]interface{}{
			"type":    "auth",
			"payload": map[string]string{"token": token},
		}
		authJSON, _ := json.Marshal(authMsg)
		conn.Write(append(authJSON, '\n'))

		response := make([]byte, 1024)
		n, err := conn.Read(response)

		if err != nil {
			t.Errorf("Connection failed for token: %s", token)
			continue
		}

		var resp tcp.Message
		if err := json.Unmarshal(response[:n], &resp); err != nil {
			t.Errorf("Invalid response format for token: %s", token)
			continue
		}

		if resp.Type != "error" {
			t.Errorf("Expected error response for invalid token: %s, got: %s", token, resp.Type)
		}
	}

	t.Log("Reliability: All invalid auth tokens handled correctly")
}

func TestReliabilityMalformedMessages(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9801", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9801")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	malformedMessages := []string{
		"{invalid json\n",
		"{}\n",
		"{\"type\":\"unknown\"}\n",
		"not json at all\n",
		"{\"type\":\"ping\",\"payload\":\"wrong-type\"}\n",
		"{\"type\":123}\n",
	}

	for _, msg := range malformedMessages {
		conn.Write([]byte(msg))

		response := make([]byte, 1024)
		n, err := conn.Read(response)

		if err != nil {
			continue
		}

		var resp tcp.Message
		if json.Unmarshal(response[:n], &resp) == nil {
			if resp.Type != "error" && resp.Type != "pong" {
				t.Errorf("Expected error response for malformed message: %s, got: %s", msg, resp.Type)
			}
		}
	}

	t.Log("Reliability: All malformed messages handled gracefully")
}

func TestReliabilityUnauthenticatedOperations(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9802", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9802")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	protectedOps := []map[string]interface{}{
		{
			"type": "sync_progress",
			"payload": map[string]interface{}{
				"manga_id":        "manga-1",
				"current_chapter": 5,
				"status":          "reading",
			},
		},
		{
			"type":    "get_library",
			"payload": map[string]interface{}{},
		},
		{
			"type": "get_progress",
			"payload": map[string]interface{}{
				"manga_id": "manga-1",
			},
		},
		{
			"type": "add_to_library",
			"payload": map[string]interface{}{
				"manga_id": "manga-1",
			},
		},
		{
			"type": "remove_from_library",
			"payload": map[string]interface{}{
				"manga_id": "manga-1",
			},
		},
	}

	for _, op := range protectedOps {
		opJSON, _ := json.Marshal(op)
		conn.Write(append(opJSON, '\n'))

		response := make([]byte, 1024)
		n, err := conn.Read(response)

		if err != nil {
			t.Errorf("Connection failed for operation: %s", op["type"])
			continue
		}

		var resp tcp.Message
		if err := json.Unmarshal(response[:n], &resp); err == nil {
			if resp.Type != "error" {
				t.Errorf("Expected error for unauthenticated operation: %s, got: %s", op["type"], resp.Type)
			}
		}
	}

	t.Log("Reliability: All unauthenticated operations rejected")
}

func TestReliabilityInvalidDatabaseOperations(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9803", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9803")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, _ := utils.GenerateJWT("test-user-1", "testuser", jwtSecret)

	authMsg := map[string]interface{}{
		"type":    "auth",
		"payload": map[string]string{"token": token},
	}
	authJSON, _ := json.Marshal(authMsg)
	conn.Write(append(authJSON, '\n'))

	response := make([]byte, 1024)
	conn.Read(response)

	invalidOps := []map[string]interface{}{
		{
			"type": "sync_progress",
			"payload": map[string]interface{}{
				"manga_id":        "non-existent-manga",
				"current_chapter": 5,
				"status":          "reading",
			},
		},
		{
			"type": "sync_progress",
			"payload": map[string]interface{}{
				"manga_id":        "manga-1",
				"current_chapter": -1,
				"status":          "reading",
			},
		},
		{
			"type": "get_progress",
			"payload": map[string]interface{}{
				"manga_id": "",
			},
		},
	}

	for _, op := range invalidOps {
		opJSON, _ := json.Marshal(op)
		conn.Write(append(opJSON, '\n'))

		n, err := conn.Read(response)

		if err != nil {
			t.Errorf("Connection failed for invalid operation: %v", op)
			continue
		}

		var resp tcp.Message
		if err := json.Unmarshal(response[:n], &resp); err == nil {
			if resp.Type != "error" {
				t.Errorf("Expected error for invalid database operation: %v, got: %s", op, resp.Type)
			}
		}
	}

	t.Log("Reliability: All invalid database operations rejected")
}

func TestReliabilityConcurrentUserUpdates(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	database.DB.Exec(`INSERT INTO manga (id, title, author, status, total_chapters) 
	                   VALUES ('manga-concurrent', 'Test Manga', 'Author', 'ongoing', 100)`)

	server := tcp.NewServer("9804", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	numConcurrent := 10
	var wg sync.WaitGroup
	wg.Add(numConcurrent)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}

	for i := 0; i < numConcurrent; i++ {
		go func(chapter int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", "localhost:9804")
			if err != nil {
				return
			}
			defer conn.Close()

			token, _ := utils.GenerateJWT("test-user-1", "testuser", jwtSecret)
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
					"manga_id":        "manga-concurrent",
					"current_chapter": chapter,
					"status":          "reading",
				},
			}
			syncJSON, _ := json.Marshal(syncMsg)
			conn.Write(append(syncJSON, '\n'))
			conn.Read(response)
		}(i + 1)
	}

	wg.Wait()

	var finalChapter int
	database.DB.QueryRow(`SELECT current_chapter FROM user_progress 
	                      WHERE user_id='test-user-1' AND manga_id='manga-concurrent'`).
		Scan(&finalChapter)

	if finalChapter < 1 || finalChapter > numConcurrent {
		t.Errorf("Invalid final chapter state: %d (expected 1-%d)", finalChapter, numConcurrent)
	}

	t.Logf("Reliability: Concurrent updates handled, final state: chapter %d", finalChapter)
}

func TestReliabilityConnectionStability(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9805", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9805")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	for i := 0; i < 100; i++ {
		pingMsg := map[string]interface{}{
			"type":    "ping",
			"payload": map[string]interface{}{},
		}
		pingJSON, _ := json.Marshal(pingMsg)
		conn.Write(append(pingJSON, '\n'))

		response := make([]byte, 1024)
		n, err := conn.Read(response)

		if err != nil || n == 0 {
			t.Fatalf("Connection became unstable after %d messages", i+1)
		}

		var resp tcp.Message
		if err := json.Unmarshal(response[:n], &resp); err != nil {
			t.Fatalf("Invalid response after %d messages", i+1)
		}

		if resp.Type != "pong" {
			t.Fatalf("Unexpected response type after %d messages: %s", i+1, resp.Type)
		}
	}

	t.Log("Reliability: Connection stable for 100 consecutive messages")
}

func TestReliabilityLargePayloadHandling(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9806", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9806")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	largeString := strings.Repeat("A", 10000)

	authMsg := map[string]interface{}{
		"type": "auth",
		"payload": map[string]string{
			"token": largeString,
		},
	}
	authJSON, _ := json.Marshal(authMsg)
	conn.Write(append(authJSON, '\n'))

	response := make([]byte, 16384)
	n, err := conn.Read(response)

	if err != nil {
		t.Fatal("Server crashed on large payload")
	}

	var resp tcp.Message
	if err := json.Unmarshal(response[:n], &resp); err != nil {
		t.Fatal("Invalid response for large payload")
	}

	if resp.Type != "error" {
		t.Errorf("Expected error response for large invalid token, got: %s", resp.Type)
	}

	t.Log("Reliability: Large payload handled gracefully")
}

func TestReliabilityRapidConnectionCycles(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9807", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	numCycles := 50
	successCount := 0

	for i := 0; i < numCycles; i++ {
		conn, err := net.Dial("tcp", "localhost:9807")
		if err != nil {
			continue
		}

		pingMsg := map[string]interface{}{
			"type":    "ping",
			"payload": map[string]interface{}{},
		}
		pingJSON, _ := json.Marshal(pingMsg)
		conn.Write(append(pingJSON, '\n'))

		response := make([]byte, 1024)
		n, err := conn.Read(response)
		conn.Close()

		if err == nil && n > 0 {
			successCount++
		}

		time.Sleep(10 * time.Millisecond)
	}

	successRate := float64(successCount) / float64(numCycles) * 100

	if successRate < 90 {
		t.Errorf("Rapid connection success rate too low: %.2f%% (expected >90%%)", successRate)
	}

	t.Logf("Reliability: Rapid connection cycles - %.2f%% success rate (%d/%d)",
		successRate, successCount, numCycles)
}
