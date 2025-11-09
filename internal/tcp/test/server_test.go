package tcp_test

import (
	"bufio"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/tcp"
)

func TestNewServer(t *testing.T) {
	server := tcp.NewServer("9999", nil)
	if server == nil {
		t.Fatal("NewServer returned nil")
	}
	if server.Port != "9999" {
		t.Errorf("Expected port 9999, got %s", server.Port)
	}
}

func TestServerStartStop(t *testing.T) {
	server := tcp.NewServer("9091", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	if err := server.Stop(); err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}
}

func TestServerAcceptConnection(t *testing.T) {
	server := tcp.NewServer("9092", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)
	conn, err := net.Dial("tcp", "localhost:9092")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()
	time.Sleep(100 * time.Millisecond)
	if server.GetClientCount() != 1 {
		t.Errorf("Expected 1 client, got %d", server.GetClientCount())
	}
}

func TestMultipleConcurrentConnections(t *testing.T) {
	server := tcp.NewServer("9093", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)
	numClients := 10
	clients := make([]net.Conn, numClients)
	for i := 0; i < numClients; i++ {
		conn, err := net.Dial("tcp", "localhost:9093")
		if err != nil {
			t.Fatalf("Failed to connect client %d: %v", i, err)
		}
		clients[i] = conn
		defer conn.Close()
	}
	time.Sleep(200 * time.Millisecond)
	count := server.GetClientCount()
	if count != numClients {
		t.Errorf("Expected %d clients, got %d", numClients, count)
	}
	for i := 0; i < numClients/2; i++ {
		clients[i].Close()
	}
	time.Sleep(200 * time.Millisecond)
	expectedCount := numClients - (numClients / 2)
	count = server.GetClientCount()
	if count != expectedCount {
		t.Errorf("Expected %d clients after closing half, got %d", expectedCount, count)
	}
}

func TestEchoFunctionality(t *testing.T) {
	server := tcp.NewServer("9094", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)
	conn, err := net.Dial("tcp", "localhost:9094")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	testMessage := `{"type":"ping","payload":{}}`
	_, err = conn.Write([]byte(testMessage + "\n"))
	if err != nil {
		t.Fatalf("Failed to write to server: %v", err)
	}

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("Failed to read from server: %v", err)
	}

	if !strings.Contains(response, `"type":"pong"`) {
		t.Errorf("Expected pong response, got '%s'", response)
	}
}

func TestClientDisconnection(t *testing.T) {
	server := tcp.NewServer("9095", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)
	conn, err := net.Dial("tcp", "localhost:9095")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	if server.GetClientCount() != 1 {
		t.Errorf("Expected 1 client before disconnect, got %d", server.GetClientCount())
	}
	conn.Close()
	time.Sleep(200 * time.Millisecond)
	if server.GetClientCount() != 0 {
		t.Errorf("Expected 0 clients after disconnect, got %d", server.GetClientCount())
	}
}

func TestServerStopWithActiveConnections(t *testing.T) {
	server := tcp.NewServer("9096", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	time.Sleep(100 * time.Millisecond)
	numClients := 5
	clients := make([]net.Conn, numClients)
	for i := 0; i < numClients; i++ {
		conn, err := net.Dial("tcp", "localhost:9096")
		if err != nil {
			t.Fatalf("Failed to connect client %d: %v", i, err)
		}
		clients[i] = conn
		defer conn.Close()
	}
	time.Sleep(100 * time.Millisecond)
	if server.GetClientCount() != numClients {
		t.Errorf("Expected %d clients, got %d", numClients, server.GetClientCount())
	}
	if err := server.Stop(); err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}
	if server.GetClientCount() != 0 {
		t.Errorf("Expected 0 clients after server stop, got %d", server.GetClientCount())
	}
}

func TestPortAlreadyInUse(t *testing.T) {
	server1 := tcp.NewServer("9097", nil)
	if err := server1.Start(); err != nil {
		t.Fatalf("Failed to start first server: %v", err)
	}
	defer server1.Stop()
	time.Sleep(100 * time.Millisecond)
	server2 := tcp.NewServer("9097", nil)
	err := server2.Start()
	if err == nil {
		server2.Stop()
		t.Error("Second server should fail to start on same port")
	}
}

func BenchmarkServerAcceptConnections(b *testing.B) {
	server := tcp.NewServer("9098", nil)
	if err := server.Start(); err != nil {
		b.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		conn, err := net.Dial("tcp", "localhost:9098")
		if err != nil {
			b.Fatalf("Failed to connect: %v", err)
		}
		conn.Close()
	}
}
