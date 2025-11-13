package test

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/udp"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
)

func startTestUDPServer(t *testing.T, port int) (*udp.Server, func()) {
	server := udp.NewServer(fmt.Sprintf("%d", port), nil)

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start test UDP server: %v", err)
	}

	cleanup := func() {
		server.Stop()
	}

	return server, cleanup
}

func generateTestToken(t *testing.T) string {
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, err := utils.GenerateJWT("test-user-123", "testuser", jwtSecret)
	if err != nil {
		t.Fatalf("Failed to generate JWT: %v", err)
	}
	return token
}

func TestNotifySubscribe(t *testing.T) {
	port := 19200
	_, cleanup := startTestUDPServer(t, port)
	defer cleanup()

	time.Sleep(100 * time.Millisecond)

	tests := []struct {
		name       string
		eventTypes []string
		wantErr    bool
	}{
		{
			name:       "valid subscription",
			eventTypes: []string{"progress_update", "library_update"},
			wantErr:    false,
		},
		{
			name:       "single event type",
			eventTypes: []string{"progress_update"},
			wantErr:    false,
		},
		{
			name:       "empty event types",
			eventTypes: []string{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := generateTestToken(t)

			conn, err := net.Dial("udp", fmt.Sprintf("localhost:%d", port))
			if err != nil {
				t.Fatalf("Failed to connect: %v", err)
			}
			defer conn.Close()

			registerMsg := udp.CreateRegisterMessage(token)
			if _, err := conn.Write(registerMsg); err != nil {
				t.Fatalf("Failed to send register message: %v", err)
			}

			buffer := make([]byte, 4096)
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, err := conn.Read(buffer)
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("Failed to receive response: %v", err)
				}
				return
			}

			var response udp.Message
			if err := json.Unmarshal(buffer[:n], &response); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if response.Type == "error" && !tt.wantErr {
				t.Errorf("Unexpected error response: %s", response.Type)
			}

			types := tt.eventTypes
			if len(types) == 0 {
				types = []string{"progress_update", "library_update"}
			}

			subscribeMsg := udp.CreateSubscribeMessage(types)
			if _, err := conn.Write(subscribeMsg); err != nil {
				t.Fatalf("Failed to send subscribe message: %v", err)
			}

			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, err = conn.Read(buffer)
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("Failed to receive subscribe response: %v", err)
				}
				return
			}

			if err := json.Unmarshal(buffer[:n], &response); err != nil {
				t.Fatalf("Failed to parse subscribe response: %v", err)
			}

			if response.Type == "error" && !tt.wantErr {
				t.Errorf("Subscription failed unexpectedly")
			}
		})
	}
}

func TestNotifyUnsubscribe(t *testing.T) {
	port := 19201
	srv, cleanup := startTestUDPServer(t, port)
	defer cleanup()

	time.Sleep(100 * time.Millisecond)

	token := generateTestToken(t)

	conn, err := net.Dial("udp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	registerMsg := udp.CreateRegisterMessage(token)
	if _, err := conn.Write(registerMsg); err != nil {
		t.Fatalf("Failed to send register message: %v", err)
	}

	buffer := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to receive response: %v", err)
	}

	var response udp.Message
	if err := json.Unmarshal(buffer[:n], &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Type == "error" {
		t.Fatalf("Registration failed: %s", response.Type)
	}

	subscribeMsg := udp.CreateSubscribeMessage([]string{"progress_update"})
	if _, err := conn.Write(subscribeMsg); err != nil {
		t.Fatalf("Failed to send subscribe message: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err = conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to receive subscribe response: %v", err)
	}

	if srv.GetSubscriberCount() != 1 {
		t.Errorf("Expected 1 subscriber, got %d", srv.GetSubscriberCount())
	}

	unregisterMsg := udp.CreateUnregisterMessage()
	if _, err := conn.Write(unregisterMsg); err != nil {
		t.Fatalf("Failed to send unregister message: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if srv.GetSubscriberCount() != 0 {
		t.Errorf("Expected 0 subscribers after unsubscribe, got %d", srv.GetSubscriberCount())
	}
}

func TestNotifyTest(t *testing.T) {
	port := 19202
	_, cleanup := startTestUDPServer(t, port)
	defer cleanup()

	time.Sleep(100 * time.Millisecond)

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "valid test",
			wantErr: false,
		},
		{
			name:    "another valid test",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := generateTestToken(t)
			conn, err := net.Dial("udp", fmt.Sprintf("localhost:%d", port))
			if err != nil {
				t.Fatalf("Failed to connect: %v", err)
			}
			defer conn.Close()

			registerMsg := udp.CreateRegisterMessage(token)
			if _, err := conn.Write(registerMsg); err != nil {
				t.Fatalf("Failed to send test message: %v", err)
			}

			buffer := make([]byte, 4096)
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, err := conn.Read(buffer)
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("No response from server: %v", err)
				}
				return
			}

			var response udp.Message
			if err := json.Unmarshal(buffer[:n], &response); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if response.Type == "error" && !tt.wantErr {
				t.Errorf("Test failed unexpectedly")
			}

			if response.Type != "success" && response.Type != "error" {
				t.Errorf("Expected success or error response, got: %s", response.Type)
			}
		})
	}
}

func TestNotifyHeartbeat(t *testing.T) {
	port := 19203
	_, cleanup := startTestUDPServer(t, port)
	defer cleanup()

	time.Sleep(100 * time.Millisecond)

	token := generateTestToken(t)

	conn, err := net.Dial("udp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	registerMsg := udp.CreateRegisterMessage(token)
	if _, err := conn.Write(registerMsg); err != nil {
		t.Fatalf("Failed to send register message: %v", err)
	}

	buffer := make([]byte, 4096)
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := conn.Read(buffer)
	if err != nil {
		t.Fatalf("Failed to receive response: %v", err)
	}

	var response udp.Message
	if err := json.Unmarshal(buffer[:n], &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response.Type == "error" {
		t.Fatalf("Registration failed")
	}

	for i := 0; i < 3; i++ {
		heartbeatMsg := udp.CreateHeartbeatMessage("client-id")
		if _, err := conn.Write(heartbeatMsg); err != nil {
			t.Fatalf("Failed to send heartbeat message: %v", err)
		}

		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		n, err = conn.Read(buffer)
		if err != nil {
			t.Fatalf("Failed to receive heartbeat response: %v", err)
		}

		if err := json.Unmarshal(buffer[:n], &response); err != nil {
			t.Fatalf("Failed to parse heartbeat response: %v", err)
		}

		if response.Type == "error" {
			t.Errorf("Heartbeat failed on iteration %d", i)
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func TestNotifyConnectivity(t *testing.T) {
	port := 19204
	_, cleanup := startTestUDPServer(t, port)
	defer cleanup()

	time.Sleep(100 * time.Millisecond)

	tests := []struct {
		name    string
		address string
		wantErr bool
	}{
		{
			name:    "localhost connection",
			address: fmt.Sprintf("localhost:%d", port),
			wantErr: false,
		},
		{
			name:    "127.0.0.1 connection",
			address: fmt.Sprintf("127.0.0.1:%d", port),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := generateTestToken(t)

			conn, err := net.Dial("udp", tt.address)
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("Failed to connect: %v", err)
				}
				return
			}
			defer conn.Close()

			registerMsg := udp.CreateRegisterMessage(token)
			if _, err := conn.Write(registerMsg); err != nil {
				t.Fatalf("Failed to send message: %v", err)
			}

			buffer := make([]byte, 4096)
			conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			n, err := conn.Read(buffer)
			if err != nil {
				if !tt.wantErr {
					t.Fatalf("Failed to receive response: %v", err)
				}
				return
			}

			var response udp.Message
			if err := json.Unmarshal(buffer[:n], &response); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			if response.Type == "error" && !tt.wantErr {
				t.Errorf("Connection test failed unexpectedly")
			}
		})
	}
}
