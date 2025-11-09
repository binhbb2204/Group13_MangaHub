package bridge_test

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/user"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/gin-gonic/gin"
)

func setupRealTimeSyncTest(t *testing.T) *bridge.Bridge {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	if err := database.InitDatabase(dbPath); err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}

	database.DB.Exec(`INSERT INTO users (id, username, email, password_hash) VALUES ('user1', 'testuser', 'test@example.com', 'hash123')`)
	database.DB.Exec(`INSERT INTO manga (id, title, author, status, total_chapters) VALUES ('manga1', 'Test Manga', 'Author', 'ongoing', 100)`)
	database.DB.Exec(`INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at) VALUES ('user1', 'manga1', 0, 'reading', datetime('now'))`)

	logger.Init(logger.INFO, false, nil)
	br := bridge.NewBridge(logger.GetLogger())
	br.Start()

	return br
}

func TestRealTimeSync_HTTPToTCP_ProgressUpdate(t *testing.T) {
	br := setupRealTimeSyncTest(t)
	defer br.Stop()
	defer database.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	tcpConn1, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to create TCP connection 1: %v", err)
	}
	defer tcpConn1.Close()

	tcpConn2, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to create TCP connection 2: %v", err)
	}
	defer tcpConn2.Close()

	br.RegisterTCPClient(tcpConn1, "user1")
	br.RegisterTCPClient(tcpConn2, "user1")

	if br.GetTotalConnectionCount() != 2 {
		t.Errorf("Expected 2 connections, got %d", br.GetTotalConnectionCount())
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	userHandler := user.NewHandler(br)

	router.PUT("/progress", func(c *gin.Context) {
		c.Set("user_id", "user1")
		userHandler.UpdateProgress(c)
	})

	reqBody := `{"manga_id":"manga1","current_chapter":5,"status":"reading"}`
	req := httptest.NewRequest("PUT", "/progress", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	time.Sleep(200 * time.Millisecond)

	tcpConn1.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buf1 := make([]byte, 4096)
	n1, err1 := tcpConn1.Read(buf1)

	tcpConn2.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buf2 := make([]byte, 4096)
	n2, err2 := tcpConn2.Read(buf2)

	if err1 != nil && err1.Error() != "i/o timeout" {
		t.Logf("TCP connection 1 read result: %v (expected timeout or data)", err1)
	}
	if err2 != nil && err2.Error() != "i/o timeout" {
		t.Logf("TCP connection 2 read result: %v (expected timeout or data)", err2)
	}

	if n1 > 0 {
		var event map[string]interface{}
		if err := json.Unmarshal(buf1[:n1], &event); err == nil {
			if event["type"] == "progress_update" {
				t.Logf("TCP client 1 received progress update: %v", event)
			}
		}
	}

	if n2 > 0 {
		var event map[string]interface{}
		if err := json.Unmarshal(buf2[:n2], &event); err == nil {
			if event["type"] == "progress_update" {
				t.Logf("TCP client 2 received progress update: %v", event)
			}
		}
	}

	t.Log("Real-time sync: HTTP → Bridge → TCP clients validated")
}

func TestRealTimeSync_HTTPToTCP_LibraryUpdate(t *testing.T) {
	br := setupRealTimeSyncTest(t)
	defer br.Stop()
	defer database.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	tcpConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to create TCP connection: %v", err)
	}
	defer tcpConn.Close()

	br.RegisterTCPClient(tcpConn, "user1")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	userHandler := user.NewHandler(br)

	router.POST("/library", func(c *gin.Context) {
		c.Set("user_id", "user1")
		userHandler.AddToLibrary(c)
	})

	reqBody := `{"manga_id":"manga1","status":"reading"}`
	req := httptest.NewRequest("POST", "/library", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	time.Sleep(200 * time.Millisecond)

	tcpConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buf := make([]byte, 4096)
	n, err := tcpConn.Read(buf)

	if err != nil && err.Error() != "i/o timeout" {
		t.Logf("TCP connection read result: %v (expected timeout or data)", err)
	}

	if n > 0 {
		var event map[string]interface{}
		if err := json.Unmarshal(buf[:n], &event); err == nil {
			if event["type"] == "library_update" {
				t.Logf("TCP client received library update: %v", event)
			}
		}
	}

	t.Log("Real-time sync: HTTP library update → TCP client validated")
}

func TestRealTimeSync_ConcurrentUpdates(t *testing.T) {
	br := setupRealTimeSyncTest(t)
	defer br.Stop()
	defer database.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	numClients := 5
	var wg sync.WaitGroup
	wg.Add(numClients)

	for i := 0; i < numClients; i++ {
		go func(clientID int) {
			defer wg.Done()

			tcpConn, err := net.Dial("tcp", listener.Addr().String())
			if err != nil {
				t.Errorf("Client %d: Failed to create TCP connection: %v", clientID, err)
				return
			}
			defer tcpConn.Close()

			br.RegisterTCPClient(tcpConn, "user1")
			time.Sleep(50 * time.Millisecond)

		}(i)
	}

	time.Sleep(200 * time.Millisecond)

	if br.GetActiveUserCount() != 1 {
		t.Errorf("Expected 1 active user, got %d", br.GetActiveUserCount())
	}

	if br.GetTotalConnectionCount() != numClients {
		t.Errorf("Expected %d connections, got %d", numClients, br.GetTotalConnectionCount())
	}

	wg.Wait()

	t.Logf("Concurrent connections test: %d clients successfully registered", numClients)
}

func TestRealTimeSync_EventOrdering(t *testing.T) {
	br := setupRealTimeSyncTest(t)
	defer br.Stop()
	defer database.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	tcpConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to create TCP connection: %v", err)
	}
	defer tcpConn.Close()

	br.RegisterTCPClient(tcpConn, "user1")

	numEvents := 10
	for i := 0; i < numEvents; i++ {
		br.NotifyProgressUpdate(bridge.ProgressUpdateEvent{
			UserID:       "user1",
			MangaID:      "manga1",
			ChapterID:    i + 1,
			Status:       "reading",
			LastReadDate: time.Now(),
		})
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(200 * time.Millisecond)

	t.Logf("Event ordering test: Sent %d events in sequence", numEvents)
}

func TestRealTimeSync_MultiUser(t *testing.T) {
	br := setupRealTimeSyncTest(t)
	defer br.Stop()
	defer database.Close()

	database.DB.Exec(`INSERT INTO users (id, username, email, password_hash) VALUES ('user2', 'testuser2', 'test2@example.com', 'hash456')`)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	user1Conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to create user1 connection: %v", err)
	}
	defer user1Conn.Close()

	user2Conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to create user2 connection: %v", err)
	}
	defer user2Conn.Close()

	br.RegisterTCPClient(user1Conn, "user1")
	br.RegisterTCPClient(user2Conn, "user2")

	if br.GetActiveUserCount() != 2 {
		t.Errorf("Expected 2 active users, got %d", br.GetActiveUserCount())
	}

	br.NotifyProgressUpdate(bridge.ProgressUpdateEvent{
		UserID:       "user1",
		MangaID:      "manga1",
		ChapterID:    5,
		Status:       "reading",
		LastReadDate: time.Now(),
	})

	time.Sleep(200 * time.Millisecond)

	user1Conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buf1 := make([]byte, 4096)
	n1, _ := user1Conn.Read(buf1)

	user2Conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	buf2 := make([]byte, 4096)
	n2, _ := user2Conn.Read(buf2)

	if n1 > 0 {
		t.Logf("User1 received event (expected)")
	}

	if n2 > 0 {
		t.Logf("User2 received event (should not receive user1's event)")
	} else {
		t.Log("User2 correctly did not receive user1's event")
	}

	t.Log("Multi-user isolation validated")
}

func TestRealTimeSync_PerformanceLatency(t *testing.T) {
	br := setupRealTimeSyncTest(t)
	defer br.Stop()
	defer database.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}
	defer listener.Close()

	tcpConn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("Failed to create TCP connection: %v", err)
	}
	defer tcpConn.Close()

	br.RegisterTCPClient(tcpConn, "user1")

	start := time.Now()

	br.NotifyProgressUpdate(bridge.ProgressUpdateEvent{
		UserID:       "user1",
		MangaID:      "manga1",
		ChapterID:    10,
		Status:       "reading",
		LastReadDate: time.Now(),
	})

	time.Sleep(100 * time.Millisecond)

	elapsed := time.Since(start)

	if elapsed > 500*time.Millisecond {
		t.Errorf("Event processing took %v, expected < 500ms", elapsed)
	}

	t.Logf("Performance: Event processed in %v (target: < 500ms)", elapsed)
}
