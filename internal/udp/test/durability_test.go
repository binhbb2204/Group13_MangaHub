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

func TestServerRestart(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19300", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if err := server.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to restart server: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19300,
	})
	if err != nil {
		server.Stop()
		t.Fatalf("Failed to connect after restart: %v", err)
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
	if err != nil || n == 0 {
		server.Stop()
		t.Error("Server not functional after restart")
	}

	server.Stop()
}

func TestGracefulShutdown(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19301", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19301,
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
	conn.Write(registerMsg)

	buffer := make([]byte, 1024)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.Read(buffer)

	if err := server.Stop(); err != nil {
		t.Errorf("Graceful shutdown failed: %v", err)
	}
}

func TestMalformedPacketRecovery(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19302", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19302,
	})
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	malformedData := []byte("this is not valid json{{{")
	conn.Write(malformedData)

	time.Sleep(100 * time.Millisecond)

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
	if err != nil || n == 0 {
		t.Error("Server did not recover from malformed packet")
	}
}

func TestMultipleRestartCycles(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19303", nil)

	numCycles := 5
	for i := 0; i < numCycles; i++ {
		if err := server.Start(); err != nil {
			t.Fatalf("Cycle %d: Failed to start server: %v", i, err)
		}

		time.Sleep(100 * time.Millisecond)

		conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 19303,
		})
		if err == nil {
			jwtSecret := os.Getenv("JWT_SECRET")
			if jwtSecret == "" {
				jwtSecret = "your-secret-key-change-this-in-production"
			}
			token, _ := utils.GenerateJWT("user1", "testuser", jwtSecret)
			registerMsg := udp.CreateRegisterMessage(token)
			conn.Write(registerMsg)
			conn.Close()
		}

		if err := server.Stop(); err != nil {
			t.Fatalf("Cycle %d: Failed to stop server: %v", i, err)
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Logf("Completed %d restart cycles successfully", numCycles)
}

func TestRecoveryFromPortConflict(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server1 := udp.NewServer("19304", nil)
	if err := server1.Start(); err != nil {
		t.Fatalf("Failed to start first server: %v", err)
	}
	defer server1.Stop()

	server2 := udp.NewServer("19304", nil)
	err := server2.Start()
	if err == nil {
		server2.Stop()
		t.Error("Expected error when starting server on occupied port")
	}
}

func TestLongRunningStability(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running stability test in short mode")
	}

	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19305", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}

	duration := 30 * time.Second
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeout := time.After(duration)
	requestCount := 0
	successCount := 0

	for {
		select {
		case <-ticker.C:
			conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
				IP:   net.ParseIP("127.0.0.1"),
				Port: 19305,
			})
			if err != nil {
				continue
			}

			token, _ := utils.GenerateJWT("user1", "testuser", jwtSecret)
			registerMsg := udp.CreateRegisterMessage(token)
			conn.Write(registerMsg)

			buffer := make([]byte, 1024)
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			n, err := conn.Read(buffer)
			if err == nil && n > 0 {
				successCount++
			}
			requestCount++
			conn.Close()

		case <-timeout:
			successRate := float64(successCount) / float64(requestCount) * 100
			t.Logf("Stability test: %d/%d requests successful (%.2f%%) over %v",
				successCount, requestCount, successRate, duration)

			if successRate < 95.0 {
				t.Errorf("Expected >95%% success rate, got %.2f%%", successRate)
			}
			return
		}
	}
}
