package udp_test

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/udp"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
)

func TestServerStartStop(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19091", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if err := server.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}
}

func TestServerStartWithInvalidPort(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("invalid-port", nil)
	err := server.Start()
	if err == nil {
		server.Stop()
		t.Error("Expected error when starting with invalid port")
	}
}

func TestServerRegister(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19092", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19092,
	})
	if err != nil {
		t.Fatalf("Failed to dial UDP: %v", err)
	}
	defer conn.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, err := utils.GenerateJWT("user1", "testuser", jwtSecret)
	if err != nil {
		t.Fatalf("Failed to generate JWT: %v", err)
	}

	registerMsg := udp.CreateRegisterMessage(token)
	_, err = conn.Write(registerMsg)
	if err != nil {
		t.Fatalf("Failed to send register message: %v", err)
	}

	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	msg, err := udp.ParseMessage(buffer[:n])
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if msg.Type != "success" {
		t.Errorf("Expected success response, got '%s'", msg.Type)
	}

	if server.GetSubscriberCount() != 1 {
		t.Errorf("Expected 1 subscriber, got %d", server.GetSubscriberCount())
	}
}

func TestServerRegisterInvalidToken(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19093", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19093,
	})
	if err != nil {
		t.Fatalf("Failed to dial UDP: %v", err)
	}
	defer conn.Close()

	registerMsg := udp.CreateRegisterMessage("invalid-token")
	conn.Write(registerMsg)

	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	msg, err := udp.ParseMessage(buffer[:n])
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if msg.Type != "error" {
		t.Errorf("Expected error response, got '%s'", msg.Type)
	}
}

func TestServerUnregister(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19094", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19094,
	})
	if err != nil {
		t.Fatalf("Failed to dial UDP: %v", err)
	}
	defer conn.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, _ := utils.GenerateJWT("user1", "testuser", jwtSecret)

	registerMsg := udp.CreateRegisterMessage(token)
	conn.Write(registerMsg)

	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.Read(buffer)

	unregisterMsg := udp.CreateUnregisterMessage()
	conn.Write(unregisterMsg)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read unregister response: %v", err)
	}

	msg, err := udp.ParseMessage(buffer[:n])
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if msg.Type != "success" {
		t.Errorf("Expected success response, got '%s'", msg.Type)
	}
}

func TestServerSubscribe(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19095", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19095,
	})
	if err != nil {
		t.Fatalf("Failed to dial UDP: %v", err)
	}
	defer conn.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, _ := utils.GenerateJWT("user1", "testuser", jwtSecret)

	registerMsg := udp.CreateRegisterMessage(token)
	conn.Write(registerMsg)

	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.Read(buffer)

	subscribeMsg := udp.CreateSubscribeMessage([]string{"progress_update"})
	conn.Write(subscribeMsg)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read subscribe response: %v", err)
	}

	msg, err := udp.ParseMessage(buffer[:n])
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if msg.Type != "success" {
		t.Errorf("Expected success response, got '%s'", msg.Type)
	}
}

func TestServerHeartbeat(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19096", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19096,
	})
	if err != nil {
		t.Fatalf("Failed to dial UDP: %v", err)
	}
	defer conn.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, _ := utils.GenerateJWT("user1", "testuser", jwtSecret)

	registerMsg := udp.CreateRegisterMessage(token)
	conn.Write(registerMsg)

	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.Read(buffer)

	heartbeatMsg := udp.CreateHeartbeatMessage("client1")
	conn.Write(heartbeatMsg)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to read heartbeat response: %v", err)
	}

	msg, err := udp.ParseMessage(buffer[:n])
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if msg.Type != "success" {
		t.Errorf("Expected success response for heartbeat, got '%s'", msg.Type)
	}
}
