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

func TestEndToEndConnectivity(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19100", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19100,
	})
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, _ := utils.GenerateJWT("user1", "testuser", jwtSecret)

	registerMsg := udp.CreateRegisterMessage(token)
	_, err = conn.Write(registerMsg)
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to receive response: %v", err)
	}

	msg, err := udp.ParseMessage(buffer[:n])
	if err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if msg.Type != "success" {
		t.Errorf("End-to-end connectivity failed: got %s", msg.Type)
	}
}

func TestIPv4Connectivity(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19101", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp4", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19101,
	})
	if err != nil {
		t.Fatalf("Failed to connect via IPv4: %v", err)
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
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to receive IPv4 response: %v", err)
	}

	if n == 0 {
		t.Error("Received empty response on IPv4")
	}
}

func TestLocalhostConnectivity(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19102", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	addresses := []string{
		"127.0.0.1:19102",
		"localhost:19102",
	}

	for _, addr := range addresses {
		t.Run(addr, func(t *testing.T) {
			udpAddr, err := net.ResolveUDPAddr("udp", addr)
			if err != nil {
				t.Skipf("Failed to resolve %s: %v", addr, err)
				return
			}

			conn, err := net.DialUDP("udp", nil, udpAddr)
			if err != nil {
				t.Fatalf("Failed to connect to %s: %v", addr, err)
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
			n, err := conn.Read(buffer)
			if err != nil {
				t.Fatalf("Failed to receive response from %s: %v", addr, err)
			}

			if n == 0 {
				t.Errorf("Received empty response from %s", addr)
			}
		})
	}
}

func TestMultipleClientsConnectivity(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19103", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	numClients := 10
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}

	for i := 0; i < numClients; i++ {
		conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 19103,
		})
		if err != nil {
			t.Fatalf("Client %d failed to connect: %v", i, err)
		}

		token, _ := utils.GenerateJWT("user"+string(rune(i)), "testuser", jwtSecret)
		registerMsg := udp.CreateRegisterMessage(token)
		conn.Write(registerMsg)

		buffer := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, err := conn.Read(buffer)
		if err != nil {
			conn.Close()
			t.Fatalf("Client %d failed to receive response: %v", i, err)
		}

		if n == 0 {
			conn.Close()
			t.Errorf("Client %d received empty response", i)
		}

		conn.Close()
	}

	if server.GetSubscriberCount() != numClients {
		t.Errorf("Expected %d subscribers, got %d", numClients, server.GetSubscriberCount())
	}
}

func TestConnectionTimeout(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19104", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19104,
	})
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, err = conn.Read(buffer)
	if err == nil {
		t.Error("Expected timeout error when no message sent")
	}
}
