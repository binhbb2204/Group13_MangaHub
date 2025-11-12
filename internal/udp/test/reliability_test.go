package udp_test

import (
	"net"
	"os"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/udp"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
)

func TestMessageDelivery(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19500", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19500,
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

	numMessages := 100
	successCount := 0

	for i := 0; i < numMessages; i++ {
		registerMsg := udp.CreateRegisterMessage(token)
		conn.Write(registerMsg)

		buffer := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		n, err := conn.Read(buffer)
		if err == nil && n > 0 {
			successCount++
		}
		time.Sleep(10 * time.Millisecond)
	}

	deliveryRate := float64(successCount) / float64(numMessages) * 100
	t.Logf("Message delivery rate: %.2f%% (%d/%d)", deliveryRate, successCount, numMessages)

	if deliveryRate < 90.0 {
		t.Errorf("Expected delivery rate >90%%, got %.2f%%", deliveryRate)
	}
}

func TestSessionPersistence(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19501", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19501,
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

	if server.GetSubscriberCount() != 1 {
		t.Error("Session not created")
	}

	time.Sleep(1 * time.Second)

	heartbeatMsg := udp.CreateHeartbeatMessage("client1")
	conn.Write(heartbeatMsg)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buffer)
	if err != nil || n == 0 {
		t.Error("Session not persistent across heartbeats")
	}

	if server.GetSubscriberCount() != 1 {
		t.Error("Session count changed unexpectedly")
	}
}

func TestReconnectionBehavior(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	server := udp.NewServer("19502", nil)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, _ := utils.GenerateJWT("user1", "testuser", jwtSecret)

	for i := 0; i < 5; i++ {
		conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 19502,
		})
		if err != nil {
			t.Fatalf("Reconnection %d failed: %v", i, err)
		}

		registerMsg := udp.CreateRegisterMessage(token)
		conn.Write(registerMsg)

		buffer := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, err := conn.Read(buffer)
		if err != nil || n == 0 {
			conn.Close()
			t.Errorf("Reconnection %d: Failed to receive response", i)
			continue
		}

		conn.Close()
		time.Sleep(200 * time.Millisecond)
	}

	t.Log("Reconnection behavior tested successfully")
}

func TestNotificationBroadcast(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	br := bridge.NewBridge(logger.GetLogger())
	br.Start()
	defer br.Stop()

	server := udp.NewServer("19503", br)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 19503,
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

	subscribeMsg := udp.CreateSubscribeMessage([]string{"progress_update"})
	conn.Write(subscribeMsg)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	conn.Read(buffer)

	time.Sleep(100 * time.Millisecond)

	br.NotifyProgressUpdate(bridge.ProgressUpdateEvent{
		UserID:       "user1",
		MangaID:      "manga-1",
		ChapterID:    42,
		Status:       "reading",
		LastReadDate: time.Now(),
	})

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to receive notification: %v", err)
	}

	if n == 0 {
		t.Error("Received empty notification")
	}

	msg, err := udp.ParseMessage(buffer[:n])
	if err != nil {
		t.Fatalf("Failed to parse notification: %v", err)
	}

	if msg.Type != "notification" {
		t.Errorf("Expected notification type, got '%s'", msg.Type)
	}

	if msg.EventType != "progress_update" {
		t.Errorf("Expected progress_update event, got '%s'", msg.EventType)
	}
}

func TestMultiDeviceNotification(t *testing.T) {
	logger.Init(logger.ERROR, false, nil)

	br := bridge.NewBridge(logger.GetLogger())
	br.Start()
	defer br.Stop()

	server := udp.NewServer("19504", br)
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	time.Sleep(100 * time.Millisecond)

	numDevices := 3
	connections := make([]*net.UDPConn, numDevices)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, _ := utils.GenerateJWT("user1", "testuser", jwtSecret)

	for i := 0; i < numDevices; i++ {
		conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
			IP:   net.ParseIP("127.0.0.1"),
			Port: 19504,
		})
		if err != nil {
			t.Fatalf("Device %d failed to connect: %v", i, err)
		}
		connections[i] = conn

		registerMsg := udp.CreateRegisterMessage(token)
		conn.Write(registerMsg)

		buffer := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.Read(buffer)
	}

	time.Sleep(100 * time.Millisecond)

	br.NotifyLibraryUpdate(bridge.LibraryUpdateEvent{
		UserID:  "user1",
		MangaID: "manga-1",
		Action:  "added",
	})

	successCount := 0
	for i, conn := range connections {
		buffer := make([]byte, 1024)
		conn.SetReadDeadline(time.Now().Add(3 * time.Second))
		n, err := conn.Read(buffer)
		if err == nil && n > 0 {
			msg, err := udp.ParseMessage(buffer[:n])
			if err == nil && msg.Type == "notification" {
				successCount++
			}
		}
		conn.Close()

		if err != nil {
			t.Logf("Device %d did not receive notification", i)
		}
	}

	t.Logf("Multi-device notification: %d/%d devices received notification", successCount, numDevices)

	if successCount < numDevices {
		t.Errorf("Expected all %d devices to receive notification, got %d", numDevices, successCount)
	}
}
