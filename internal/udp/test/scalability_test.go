package udp_test

import (
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/udp"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
)

func TestConcurrentConnections(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19200", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	numClients := 100
	var wg sync.WaitGroup
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}

	successCount := 0
	var mu sync.Mutex

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
				IP:   net.ParseIP("127.0.0.1"),
				Port: 19200,
			})
			if err != nil {
				return
			}
			defer conn.Close()

			token, _ := utils.GenerateJWT("user"+string(rune(id)), "testuser", jwtSecret)
			registerMsg := udp.CreateRegisterMessage(token)
			conn.Write(registerMsg)

			buffer := make([]byte, 1024)
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, err := conn.Read(buffer)
			if err == nil && n > 0 {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(i)
	}

	wg.Wait()

	if successCount < numClients*90/100 {
		t.Errorf("Expected at least 90%% success rate, got %d/%d", successCount, numClients)
	}

	t.Logf("Successfully registered %d/%d concurrent clients", successCount, numClients)
}

func TestHighFrequencyHeartbeats(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19201", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19201,
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

	numHeartbeats := 100
	successCount := 0

	for i := 0; i < numHeartbeats; i++ {
		heartbeatMsg := udp.CreateHeartbeatMessage("client1")
		conn.Write(heartbeatMsg)

		conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, err := conn.Read(buffer)
		if err == nil && n > 0 {
			successCount++
		}
		time.Sleep(10 * time.Millisecond)
	}

	if successCount < numHeartbeats*90/100 {
		t.Errorf("Expected at least 90%% heartbeat success rate, got %d/%d", successCount, numHeartbeats)
	}

	t.Logf("Successfully processed %d/%d heartbeats", successCount, numHeartbeats)
}

func TestSessionLoad(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19202", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	numSessions := 500
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}

	connections := make([]*net.UDPConn, 0, numSessions)

	for i := 0; i < numSessions; i++ {
		conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 19202,
		})
		if err != nil {
			continue
		}

		token, _ := utils.GenerateJWT("user"+string(rune(i)), "testuser", jwtSecret)
		registerMsg := udp.CreateRegisterMessage(token)
		conn.Write(registerMsg)

		connections = append(connections, conn)
		time.Sleep(5 * time.Millisecond)
	}

	time.Sleep(500 * time.Millisecond)

	actualCount := server.GetSubscriberCount()
	if actualCount < numSessions*80/100 {
		t.Errorf("Expected at least 80%% of %d sessions, got %d", numSessions, actualCount)
	}

	t.Logf("Successfully maintained %d/%d sessions under load", actualCount, numSessions)

	for _, conn := range connections {
		conn.Close()
	}
}

func TestConnectionChurn(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19203", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}

	numCycles := 50
	for i := 0; i < numCycles; i++ {
		conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 19203,
		})
		if err != nil {
			continue
		}

		token, _ := utils.GenerateJWT("user1", "testuser", jwtSecret)
		registerMsg := udp.CreateRegisterMessage(token)
		conn.Write(registerMsg)

		buffer := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		conn.Read(buffer)

		unregisterMsg := udp.CreateUnregisterMessage()
		conn.Write(unregisterMsg)

		conn.Close()
		time.Sleep(50 * time.Millisecond)
	}

	t.Logf("Completed %d connection churn cycles", numCycles)
}

func TestScalabilityMetrics(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19204", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	testCases := []int{10, 50, 100, 200}
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}

	for _, numClients := range testCases {
		t.Run("clients_"+string(rune(numClients)), func(t *testing.T) {
			start := time.Now()
			var wg sync.WaitGroup

			for i := 0; i < numClients; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()

					conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
						IP:   net.ParseIP("127.0.0.1"),
						Port: 19204,
					})
					if err != nil {
						return
					}
					defer conn.Close()

					token, _ := utils.GenerateJWT("user"+string(rune(id)), "testuser", jwtSecret)
					registerMsg := udp.CreateRegisterMessage(token)
					conn.Write(registerMsg)

					buffer := make([]byte, 1024)
					conn.SetReadDeadline(time.Now().Add(2 * time.Second))
					conn.Read(buffer)
				}(i)
			}

			wg.Wait()
			elapsed := time.Since(start)

			t.Logf("Registered %d clients in %v (avg: %v per client)",
				numClients, elapsed, elapsed/time.Duration(numClients))
		})
	}
}
