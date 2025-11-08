package tcp_test

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/tcp"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/utils"
)

func TestInteroperabilityJSONProtocol(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9600")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9600")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	messages := []string{
		`{"type":"ping","payload":{}}`,
		`{"type":"ping", "payload": {}}`,
		`{"type": "ping", "payload": {}}`,
	}

	for i, msg := range messages {
		conn.Write([]byte(msg + "\n"))
		response := make([]byte, 1024)
		n, err := conn.Read(response)
		if err != nil || n == 0 {
			t.Errorf("Message %d failed: %s", i+1, msg)
			continue
		}

		var respMsg map[string]interface{}
		if err := json.Unmarshal(response[:n], &respMsg); err != nil {
			t.Errorf("Response %d not valid JSON: %v", i+1, err)
		}
	}

	t.Logf("Interoperability: JSON protocol variations handled correctly")
}

func TestInteroperabilityMessageFormatCompliance(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9601")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9601")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	testCases := []struct {
		name     string
		message  string
		expectOK bool
	}{
		{"Valid ping", `{"type":"ping","payload":{}}`, true},
		{"Missing payload", `{"type":"ping"}`, true},
		{"Missing type", `{"payload":{}}`, false},
		{"Invalid JSON", `{invalid json}`, false},
		{"Empty message", ``, false},
		{"Extra fields", `{"type":"ping","payload":{},"extra":"field"}`, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.message != "" {
				conn.Write([]byte(tc.message + "\n"))
				response := make([]byte, 1024)
				n, err := conn.Read(response)

				if tc.expectOK {
					if err != nil || n == 0 {
						t.Errorf("Expected success but got error: %v", err)
					}
				} else {
					if err == nil && n > 0 {
						respStr := string(response[:n])
						if !strings.Contains(respStr, "error") {
							t.Errorf("Expected error response but got: %s", respStr)
						}
					}
				}
			}
		})
	}

	t.Logf("Interoperability: Message format compliance validated")
}

func TestInteroperabilityNewlineDelimiter(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9602")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9602")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	messages := []string{
		`{"type":"ping","payload":{}}`,
		`{"type":"ping","payload":{}}`,
		`{"type":"ping","payload":{}}`,
	}

	for _, msg := range messages {
		conn.Write([]byte(msg + "\n"))
	}

	reader := bufio.NewReader(conn)
	receivedCount := 0
	for i := 0; i < len(messages); i++ {
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("Failed to read response %d: %v", i+1, err)
		}
		if len(line) > 0 {
			receivedCount++
		}
	}

	if receivedCount != len(messages) {
		t.Errorf("Expected %d responses, got %d", len(messages), receivedCount)
	}

	t.Logf("Interoperability: Newline delimiter handled correctly for %d messages", receivedCount)
}

func TestInteroperabilityUTF8Encoding(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	_, err := database.DB.Exec(`
		INSERT INTO manga (id, title, author, status, total_chapters) 
		VALUES ('manga-utf8', 'テストマンガ 📚', '作者名', 'ongoing', 50)
	`)
	if err != nil {
		t.Fatalf("Failed to insert UTF-8 test data: %v", err)
	}

	server := tcp.NewServer("9603")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9603")
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

	response := make([]byte, 4096)
	conn.Read(response)

	addMsg := map[string]interface{}{
		"type": "add_to_library",
		"payload": map[string]interface{}{
			"manga_id": "manga-utf8",
			"status":   "reading",
		},
	}
	addJSON, _ := json.Marshal(addMsg)
	conn.Write(append(addJSON, '\n'))
	conn.Read(response)

	getMsg := map[string]interface{}{
		"type":    "get_library",
		"payload": map[string]interface{}{},
	}
	getJSON, _ := json.Marshal(getMsg)
	conn.Write(append(getJSON, '\n'))

	n, _ := conn.Read(response)
	responseStr := string(response[:n])

	if !strings.Contains(responseStr, "テストマンガ") {
		t.Errorf("UTF-8 characters not preserved: %s", responseStr)
	}

	t.Logf("Interoperability: UTF-8 encoding handled correctly")
}

func TestInteroperabilityMultipleMessageTypes(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9604")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9604")
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "your-secret-key-change-this-in-production"
	}
	token, _ := utils.GenerateJWT("test-user-1", "testuser", jwtSecret)

	messages := []map[string]interface{}{
		{"type": "ping", "payload": map[string]interface{}{}},
		{"type": "auth", "payload": map[string]string{"token": token}},
		{"type": "add_to_library", "payload": map[string]interface{}{"manga_id": "manga-1", "status": "reading"}},
		{"type": "sync_progress", "payload": map[string]interface{}{"user_id": "test-user-1", "manga_id": "manga-1", "current_chapter": 10, "status": "reading"}},
		{"type": "get_progress", "payload": map[string]interface{}{"manga_id": "manga-1"}},
		{"type": "get_library", "payload": map[string]interface{}{}},
		{"type": "remove_from_library", "payload": map[string]interface{}{"manga_id": "manga-1"}},
	}

	response := make([]byte, 4096)
	successCount := 0

	for _, msg := range messages {
		msgJSON, _ := json.Marshal(msg)
		conn.Write(append(msgJSON, '\n'))
		n, err := conn.Read(response)
		if err == nil && n > 0 {
			successCount++
		}
		time.Sleep(10 * time.Millisecond)
	}

	if successCount != len(messages) {
		t.Errorf("Not all message types handled: %d/%d succeeded", successCount, len(messages))
	}

	t.Logf("Interoperability: All %d message types processed successfully", successCount)
}

func TestInteroperabilityResponseFormat(t *testing.T) {
	setupLibraryTestDB(t)
	defer database.Close()

	server := tcp.NewServer("9605")
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()
	time.Sleep(100 * time.Millisecond)

	conn, err := net.Dial("tcp", "localhost:9605")
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
		t.Fatal("No response received")
	}

	var respMsg struct {
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
	}

	if err := json.Unmarshal(response[:n], &respMsg); err != nil {
		t.Fatalf("Response not valid JSON: %v", err)
	}

	if respMsg.Type != "pong" {
		t.Errorf("Expected type 'pong', got '%s'", respMsg.Type)
	}

	if !strings.HasSuffix(string(response[:n]), "\n") {
		t.Error("Response not terminated with newline")
	}

	t.Logf("Interoperability: Response format complies with protocol specification")
}
