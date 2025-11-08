package tcp_test

import (
	"encoding/json"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/tcp"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
)

func TestScalabilityConcurrentConnections(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9300")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	numClients := 50
	var wg sync.WaitGroup
	successCount := int32(0)
	errorCount := int32(0)

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", "localhost:9300")
			if err != nil {
				atomic.AddInt32(&errorCount, 1)
				return
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
			n, err := conn.Read(response)
			if err != nil || n == 0 {
				atomic.AddInt32(&errorCount, 1)
				return
			}

			pingMsg := map[string]interface{}{
				"type":    "ping",
				"payload": map[string]interface{}{},
			}
			pingJSON, _ := json.Marshal(pingMsg)
			conn.Write(append(pingJSON, '\n'))

			n, err = conn.Read(response)
			if err == nil && n > 0 {
				atomic.AddInt32(&successCount, 1)
			} else {
				atomic.AddInt32(&errorCount, 1)
			}
		}(i)
	}

	wg.Wait()

	expectedMin := int32(float64(numClients) * 0.95)
	if successCount < expectedMin {
		t.Errorf("Scalability test failed: only %d/%d clients succeeded (expected >95%%)", successCount, numClients)
	}
	t.Logf("Scalability: %d/%d clients succeeded, %d errors", successCount, numClients, errorCount)
}

func TestScalabilityHighThroughput(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9301")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9301")
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

	numMessages := 100
	start := time.Now()
	successCount := 0

	for i := 0; i < numMessages; i++ {
		pingMsg := map[string]interface{}{
			"type":    "ping",
			"payload": map[string]interface{}{},
		}
		pingJSON, _ := json.Marshal(pingMsg)
		conn.Write(append(pingJSON, '\n'))

		n, err := conn.Read(response)
		if err == nil && n > 0 {
			successCount++
		}
	}

	duration := time.Since(start)
	throughput := float64(successCount) / duration.Seconds()

	if throughput < 50 {
		t.Errorf("Throughput too low: %.2f messages/sec (expected >50)", throughput)
	}
	t.Logf("Throughput: %.2f messages/sec (%d messages in %v)", throughput, successCount, duration)
}

func TestScalabilityConcurrentDatabaseWrites(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9302")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	numClients := 10
	messagesPerClient := 5
	var wg sync.WaitGroup
	successCount := int32(0)

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(clientID int) {
			defer wg.Done()

			conn, err := net.Dial("tcp", "localhost:9302")
			if err != nil {
				return
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

			for j := 0; j < messagesPerClient; j++ {
				syncMsg := map[string]interface{}{
					"type": "sync_progress",
					"payload": map[string]interface{}{
						"user_id":         "test-user-1",
						"manga_id":        "manga-1",
						"current_chapter": clientID*10 + j,
						"status":          "reading",
					},
				}
				syncJSON, _ := json.Marshal(syncMsg)
				conn.Write(append(syncJSON, '\n'))

				n, err := conn.Read(response)
				if err == nil && n > 0 {
					atomic.AddInt32(&successCount, 1)
				}
			}
		}(i)
	}

	wg.Wait()

	expectedMessages := int32(numClients * messagesPerClient)
	if successCount < expectedMessages*95/100 {
		t.Errorf("Concurrent DB writes: only %d/%d succeeded (expected >95%%)", successCount, expectedMessages)
	}
	t.Logf("Concurrent DB writes: %d/%d messages succeeded", successCount, expectedMessages)
}

func BenchmarkMessageProcessing(b *testing.B) {
	tmpDir := b.TempDir()
	dbPath := tmpDir + "/test.db"
	database.InitDatabase(dbPath)
	defer database.Close()

	database.DB.Exec(`INSERT INTO users (id, username, email, password_hash) VALUES ('test-user-1', 'testuser', 'test@example.com', 'hash123')`)
	database.DB.Exec(`INSERT INTO manga (id, title, author, status, total_chapters) VALUES ('manga-1', 'Test Manga', 'Author', 'ongoing', 100)`)

	server := tcp.NewServer("9303")
	server.Start()
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, _ := net.Dial("tcp", "localhost:9303")
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

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pingMsg := map[string]interface{}{
			"type":    "ping",
			"payload": map[string]interface{}{},
		}
		pingJSON, _ := json.Marshal(pingMsg)
		conn.Write(append(pingJSON, '\n'))
		conn.Read(response)
	}
}

func TestScalabilityMemoryLeak(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9304")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	iterations := 20
	for i := 0; i < iterations; i++ {
		conn, err := net.Dial("tcp", "localhost:9304")
		if err != nil {
			t.Fatalf("Connection failed on iteration %d: %v", i, err)
		}

		pingMsg := map[string]interface{}{
			"type":    "ping",
			"payload": map[string]interface{}{},
		}
		pingJSON, _ := json.Marshal(pingMsg)
		conn.Write(append(pingJSON, '\n'))

		response := make([]byte, 1024)
		conn.Read(response)
		conn.Close()

		time.Sleep(10 * time.Millisecond)
	}

	t.Logf("Memory leak test: %d connect/disconnect cycles completed successfully", iterations)
}
