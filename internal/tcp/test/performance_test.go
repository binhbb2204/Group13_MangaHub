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

func TestPerformanceMessageLatency(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9700", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9700")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	numMessages := 50
	var totalLatency time.Duration

	for i := 0; i < numMessages; i++ {
		pingMsg := map[string]interface{}{
			"type":    "ping",
			"payload": map[string]interface{}{},
		}
		pingJSON, _ := json.Marshal(pingMsg)

		start := time.Now()
		conn.Write(append(pingJSON, '\n'))

		response := make([]byte, 1024)
		n, err := conn.Read(response)
		latency := time.Since(start)

		if err != nil || n == 0 {
			t.Errorf("Message %d failed", i+1)
			continue
		}

		totalLatency += latency
	}

	avgLatency := totalLatency / time.Duration(numMessages)

	if avgLatency > 50*time.Millisecond {
		t.Errorf("Average latency too high: %v (expected <50ms)", avgLatency)
	}

	t.Logf("Performance: Average message latency: %v", avgLatency)
}

func TestPerformanceDatabaseQuerySpeed(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	for i := 1; i <= 10; i++ {
		database.DB.Exec(`INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at) 
		                   VALUES ('test-user-1', ?, ?, 'reading', datetime('now'))`,
			"manga-"+string(rune(i)), i*10)
	}

	server := tcp.NewServer("9701", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9701")
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

	response := make([]byte, 8192)
	conn.Read(response)

	getLibMsg := map[string]interface{}{
		"type":    "get_library",
		"payload": map[string]interface{}{},
	}
	getLibJSON, _ := json.Marshal(getLibMsg)

	start := time.Now()
	conn.Write(append(getLibJSON, '\n'))
	n, err := conn.Read(response)
	queryTime := time.Since(start)

	if err != nil || n == 0 {
		t.Fatal("Failed to get library")
	}

	if queryTime > 100*time.Millisecond {
		t.Errorf("Database query too slow: %v (expected <100ms)", queryTime)
	}

	t.Logf("Performance: Database query time: %v", queryTime)
}

func TestPerformanceConcurrentOperationsThroughput(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9702", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	numClients := 10
	messagesPerClient := 20
	totalMessages := numClients * messagesPerClient

	start := time.Now()

	doneChan := make(chan bool, numClients)
	for i := 0; i < numClients; i++ {
		go func() {
			conn, err := net.Dial("tcp", "localhost:9702")
			if err != nil {
				doneChan <- false
				return
			}
			defer conn.Close()

			response := make([]byte, 1024)
			for j := 0; j < messagesPerClient; j++ {
				pingMsg := map[string]interface{}{
					"type":    "ping",
					"payload": map[string]interface{}{},
				}
				pingJSON, _ := json.Marshal(pingMsg)
				conn.Write(append(pingJSON, '\n'))
				conn.Read(response)
			}
			doneChan <- true
		}()
	}

	for i := 0; i < numClients; i++ {
		<-doneChan
	}

	duration := time.Since(start)
	throughput := float64(totalMessages) / duration.Seconds()

	if throughput < 100 {
		t.Errorf("Concurrent throughput too low: %.2f msg/sec (expected >100)", throughput)
	}

	t.Logf("Performance: Concurrent throughput: %.2f messages/sec (%d messages in %v)",
		throughput, totalMessages, duration)
}

func TestPerformanceAuthenticationSpeed(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9703", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	numAuths := 20
	var totalAuthTime time.Duration

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}

	for i := 0; i < numAuths; i++ {
		conn, err := net.Dial("tcp", "localhost:9703")
		if err != nil {
			t.Fatalf("Failed to connect: %v", err)
		}

		token, _ := utils.GenerateJWT("test-user-1", "testuser", jwtSecret)
		authMsg := map[string]interface{}{
			"type":    "auth",
			"payload": map[string]string{"token": token},
		}
		authJSON, _ := json.Marshal(authMsg)

		start := time.Now()
		conn.Write(append(authJSON, '\n'))

		response := make([]byte, 1024)
		n, err := conn.Read(response)
		authTime := time.Since(start)

		if err != nil || n == 0 {
			t.Errorf("Auth %d failed", i+1)
		}

		totalAuthTime += authTime
		conn.Close()
	}

	avgAuthTime := totalAuthTime / time.Duration(numAuths)

	if avgAuthTime > 50*time.Millisecond {
		t.Errorf("Average auth time too high: %v (expected <50ms)", avgAuthTime)
	}

	t.Logf("Performance: Average authentication time: %v", avgAuthTime)
}

func TestPerformanceLargeLibraryRetrieval(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	for i := 1; i <= 50; i++ {
		database.DB.Exec(`INSERT INTO manga (id, title, author, status, total_chapters) 
		                   VALUES (?, ?, 'Author', 'ongoing', 100)`,
			"manga-large-"+string(rune(i)), "Manga Title "+string(rune(i)))
		database.DB.Exec(`INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at) 
		                   VALUES ('test-user-1', ?, ?, 'reading', datetime('now'))`,
			"manga-large-"+string(rune(i)), i)
	}

	server := tcp.NewServer("9704", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9704")
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

	response := make([]byte, 65536)
	conn.Read(response)

	getLibMsg := map[string]interface{}{
		"type":    "get_library",
		"payload": map[string]interface{}{},
	}
	getLibJSON, _ := json.Marshal(getLibMsg)

	start := time.Now()
	conn.Write(append(getLibJSON, '\n'))
	n, err := conn.Read(response)
	retrievalTime := time.Since(start)

	if err != nil || n == 0 {
		t.Fatal("Failed to retrieve large library")
	}

	if retrievalTime > 200*time.Millisecond {
		t.Errorf("Large library retrieval too slow: %v (expected <200ms)", retrievalTime)
	}

	t.Logf("Performance: Retrieved 50+ items in %v", retrievalTime)
}
