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

func TestConnectivityGracefulDisconnection(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9500", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9500")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

	pingMsg := map[string]interface{}{
		"type":    "ping",
		"payload": map[string]interface{}{},
	}
	pingJSON, _ := json.Marshal(pingMsg)
	conn.Write(append(pingJSON, '\n'))

	response := make([]byte, 1024)
	n, err := conn.Read(response)
	if err != nil || n == 0 {
		t.Fatalf("Failed to receive response: %v", err)
	}

	conn.Close()
	time.Sleep(100 * time.Millisecond)

	conn2, err := net.Dial("tcp", "localhost:9500")
	if err != nil {
		t.Fatalf("Failed to reconnect after graceful close: %v", err)
	}
	defer conn2.Close()

	conn2.Write(append(pingJSON, '\n'))
	n, err = conn2.Read(response)
	if err != nil || n == 0 {
		t.Fatalf("Server not accepting new connections after client disconnect")
	}

	t.Logf("Connectivity: Graceful disconnection and reconnection successful")
}

func TestConnectivityMultipleSequentialConnections(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9501", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	numConnections := 10
	for i := 0; i < numConnections; i++ {
		conn, err := net.Dial("tcp", "localhost:9501")
		if err != nil {
			t.Fatalf("Connection %d failed: %v", i+1, err)
		}

		pingMsg := map[string]interface{}{
			"type":    "ping",
			"payload": map[string]interface{}{},
		}
		pingJSON, _ := json.Marshal(pingMsg)
		conn.Write(append(pingJSON, '\n'))

		response := make([]byte, 1024)
		n, err := conn.Read(response)
		if err != nil || n == 0 {
			t.Fatalf("Connection %d failed to receive response", i+1)
		}

		conn.Close()
		time.Sleep(50 * time.Millisecond)
	}

	t.Logf("Connectivity: %d sequential connections successful", numConnections)
}

func TestConnectivityConnectionTimeout(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9502", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialTimeout("tcp", "localhost:9502", 2*time.Second)
	if err != nil {
		t.Fatalf("Failed to connect with timeout: %v", err)
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(1 * time.Second))

	pingMsg := map[string]interface{}{
		"type":    "ping",
		"payload": map[string]interface{}{},
	}
	pingJSON, _ := json.Marshal(pingMsg)
	conn.Write(append(pingJSON, '\n'))

	response := make([]byte, 1024)
	n, err := conn.Read(response)
	if err != nil {
		t.Fatalf("Failed to read with deadline: %v", err)
	}
	if n == 0 {
		t.Fatal("No response received before deadline")
	}

	t.Logf("Connectivity: Connection timeout handling works correctly")
}

func TestConnectivityAbruptDisconnection(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9503", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9503")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}

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

	if tcpConn, ok := conn.(*net.TCPConn); ok {
		tcpConn.SetLinger(0)
	}
	conn.Close()

	time.Sleep(200 * time.Millisecond)

	conn2, err := net.Dial("tcp", "localhost:9503")
	if err != nil {
		t.Fatalf("Server crashed after abrupt disconnection: %v", err)
	}
	defer conn2.Close()

	pingMsg := map[string]interface{}{
		"type":    "ping",
		"payload": map[string]interface{}{},
	}
	pingJSON, _ := json.Marshal(pingMsg)
	conn2.Write(append(pingJSON, '\n'))

	n, err := conn2.Read(response)
	if err != nil || n == 0 {
		t.Fatal("Server not accepting connections after abrupt client disconnect")
	}

	t.Logf("Connectivity: Server recovered from abrupt disconnection")
}

func TestConnectivityConcurrentConnectionsStability(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9504", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	numConnections := 20
	connections := make([]net.Conn, numConnections)

	for i := 0; i < numConnections; i++ {
		conn, err := net.Dial("tcp", "localhost:9504")
		if err != nil {
			t.Fatalf("Failed to establish connection %d: %v", i+1, err)
		}
		connections[i] = conn
	}

	for i, conn := range connections {
		pingMsg := map[string]interface{}{
			"type":    "ping",
			"payload": map[string]interface{}{},
		}
		pingJSON, _ := json.Marshal(pingMsg)
		conn.Write(append(pingJSON, '\n'))

		response := make([]byte, 1024)
		n, err := conn.Read(response)
		if err != nil || n == 0 {
			t.Errorf("Connection %d failed to communicate", i+1)
		}
		conn.Close()
	}

	t.Logf("Connectivity: All %d concurrent connections stable", numConnections)
}

func TestConnectivityServerShutdownGraceful(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9505", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9505")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	pingMsg := map[string]interface{}{
		"type":    "ping",
		"payload": map[string]interface{}{},
	}
	pingJSON, _ := json.Marshal(pingMsg)
	conn.Write(append(pingJSON, '\n'))

	response := make([]byte, 1024)
	n, err := conn.Read(response)
	if err != nil || n == 0 {
		t.Fatal("Failed to communicate before shutdown")
	}

	shutdownComplete := make(chan bool)
	go func() {
		server.Stop()
		shutdownComplete <- true
	}()

	select {
	case <-shutdownComplete:
		t.Logf("Connectivity: Server shutdown gracefully")
	case <-time.After(3 * time.Second):
		t.Fatal("Server shutdown timed out")
	}
}
