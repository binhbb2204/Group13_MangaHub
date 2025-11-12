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

func BenchmarkRegister(b *testing.B) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19400", nil)
	if err := server.Start(); err != nil {
		b.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, _ := utils.GenerateJWT("user1", "testuser", jwtSecret)
	registerMsg := udp.CreateRegisterMessage(token)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 19400,
		})
		if err != nil {
			continue
		}

		conn.Write(registerMsg)

		buffer := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		conn.Read(buffer)
		conn.Close()
	}
}

func BenchmarkHeartbeat(b *testing.B) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19401", nil)
	if err := server.Start(); err != nil {
		b.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19401,
	})
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
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

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		conn.Write(heartbeatMsg)
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		conn.Read(buffer)
	}
}

func TestThroughput(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19402", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19402,
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

	numMessages := 1000
	heartbeatMsg := udp.CreateHeartbeatMessage("client1")

	start := time.Now()
	successCount := 0

	for i := 0; i < numMessages; i++ {
		conn.Write(heartbeatMsg)
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		n, err := conn.Read(buffer)
		if err == nil && n > 0 {
			successCount++
		}
	}

	elapsed := time.Since(start)
	throughput := float64(successCount) / elapsed.Seconds()

	t.Logf("Throughput: %.2f messages/second (%d/%d successful in %v)",
		throughput, successCount, numMessages, elapsed)

	if throughput < 100 {
		t.Errorf("Expected throughput >100 msg/s, got %.2f", throughput)
	}
}

func TestLatency(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19403", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19403,
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

	numSamples := 100
	latencies := make([]time.Duration, 0, numSamples)
	heartbeatMsg := udp.CreateHeartbeatMessage("client1")

	for i := 0; i < numSamples; i++ {
		start := time.Now()
		conn.Write(heartbeatMsg)

		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, err := conn.Read(buffer)
		if err == nil && n > 0 {
			latency := time.Since(start)
			latencies = append(latencies, latency)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if len(latencies) == 0 {
		t.Fatal("No successful latency measurements")
	}

	var sum time.Duration
	for _, lat := range latencies {
		sum += lat
	}
	avgLatency := sum / time.Duration(len(latencies))

	t.Logf("Average latency: %v (%d samples)", avgLatency, len(latencies))

	if avgLatency > 100*time.Millisecond {
		t.Errorf("Expected average latency <100ms, got %v", avgLatency)
	}
}

func TestConcurrentThroughput(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19404", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	numClients := 10
	messagesPerClient := 100

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}

	var wg sync.WaitGroup
	totalSuccess := 0
	var mu sync.Mutex

	start := time.Now()

	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
				IP:   net.ParseIP("127.0.0.1"),
				Port: 19404,
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

			heartbeatMsg := udp.CreateHeartbeatMessage("client1")
			successCount := 0

			for j := 0; j < messagesPerClient; j++ {
				conn.Write(heartbeatMsg)
				conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
				n, err := conn.Read(buffer)
				if err == nil && n > 0 {
					successCount++
				}
			}

			mu.Lock()
			totalSuccess += successCount
			mu.Unlock()
		}(i)
	}

	wg.Wait()
	elapsed := time.Since(start)

	totalMessages := numClients * messagesPerClient
	throughput := float64(totalSuccess) / elapsed.Seconds()

	t.Logf("Concurrent throughput: %.2f msg/s (%d/%d successful, %d clients)",
		throughput, totalSuccess, totalMessages, numClients)

	if throughput < 100 {
		t.Errorf("Expected concurrent throughput >100 msg/s, got %.2f", throughput)
	}
}
