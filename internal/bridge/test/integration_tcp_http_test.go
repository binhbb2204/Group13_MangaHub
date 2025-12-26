package bridge_test

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/manga"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/user"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

type mockAddr struct{ s string }

func (a mockAddr) Network() string { return "tcp" }
func (a mockAddr) String() string  { return a.s }

type bufConn struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (c *bufConn) Read(p []byte) (int, error) { return 0, nil }
func (c *bufConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(p)
}
func (c *bufConn) Close() error                       { return nil }
func (c *bufConn) LocalAddr() net.Addr                { return mockAddr{"127.0.0.1:0"} }
func (c *bufConn) RemoteAddr() net.Addr               { return mockAddr{"127.0.0.1:0"} }
func (c *bufConn) SetDeadline(t time.Time) error      { return nil }
func (c *bufConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *bufConn) SetWriteDeadline(t time.Time) error { return nil }
func (c *bufConn) GetString() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.String()
}

func setupIntegrationEnv(t *testing.T) (*bridge.Bridge, func()) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	if err := database.InitDatabase(dbPath); err != nil {
		t.Fatalf("init db: %v", err)
	}
	database.DB.Exec(`INSERT INTO users (id, username, email, password_hash) VALUES ('userA','userA','userA@example.com','hashA')`)
	database.DB.Exec(`INSERT INTO users (id, username, email, password_hash) VALUES ('userB','userB','userB@example.com','hashB')`)
	database.DB.Exec(`INSERT INTO manga (id, title, author, status, total_chapters) VALUES ('mangaX','Manga X','Author','ongoing',150)`)
	database.DB.Exec(`INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at) VALUES ('userA','mangaX',0,'reading', datetime('now'))`)
	logger.Init(logger.INFO, false, nil)
	br := bridge.NewBridge(logger.GetLogger())
	br.Start()
	cleanup := func() {
		br.Stop()
		database.Close()
	}
	return br, cleanup
}

func TestIntegration_LibraryAddBroadcast(t *testing.T) {
	br, cleanup := setupIntegrationEnv(t)
	defer cleanup()
	userAConn1 := &bufConn{}
	userAConn2 := &bufConn{}
	userBConn := &bufConn{}
	br.RegisterTCPClient(userAConn1, "userA")
	br.RegisterTCPClient(userAConn2, "userA")
	br.RegisterTCPClient(userBConn, "userB")
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Create mock external source for testing
	mockSource := manga.NewMockExternalSource()
	userHandler := user.NewHandlerWithSource(br, mockSource)

	router.POST("/library", func(c *gin.Context) {
		c.Set("user_id", "userA")
		userHandler.AddToLibrary(c)
	})
	reqBody := `{"manga_id":"mangaX","status":"reading"}`
	req := httptest.NewRequest("POST", "/library", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != 200 {
		t.Fatalf("unexpected HTTP status: %d", resp.Code)
	}
	time.Sleep(150 * time.Millisecond)
	line1 := userAConn1.GetString()
	line2 := userAConn2.GetString()
	lineB := userBConn.GetString()
	if line1 == "" || line2 == "" {
		t.Fatalf("expected event for userA connections")
	}
	if lineB != "" {
		t.Fatalf("userB received unexpected event: %s", lineB)
	}
	var evt bridge.Event
	if err := json.Unmarshal([]byte(strings.TrimSpace(line1)), &evt); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if evt.Type != bridge.EventTypeLibraryUpdate || evt.UserID != "userA" {
		t.Fatalf("event mismatch: %+v", evt)
	}
}

func TestIntegration_ProgressUpdateEndToEnd(t *testing.T) {
	br, cleanup := setupIntegrationEnv(t)
	defer cleanup()
	tcpConn := &bufConn{}
	br.RegisterTCPClient(tcpConn, "userA")
	gin.SetMode(gin.TestMode)
	router := gin.New()
	userHandler := user.NewHandler(br)
	router.PUT("/progress", func(c *gin.Context) {
		c.Set("user_id", "userA")
		userHandler.UpdateProgress(c)
	})
	reqBody := `{"manga_id":"mangaX","current_chapter":42,"status":"reading"}`
	req := httptest.NewRequest("PUT", "/progress", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != 200 {
		t.Fatalf("unexpected HTTP status: %d", resp.Code)
	}
	time.Sleep(150 * time.Millisecond)
	line := tcpConn.GetString()
	if line == "" {
		t.Fatalf("no progress_update event received")
	}
	var evt bridge.Event
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &evt); err != nil {
		t.Fatalf("unmarshal event: %v", err)
	}
	if evt.Type != bridge.EventTypeProgressUpdate {
		t.Fatalf("expected progress_update, got %s", evt.Type)
	}
	if evt.UserID != "userA" {
		t.Fatalf("wrong user in event: %s", evt.UserID)
	}
	var chapter int
	err := database.DB.QueryRow(`SELECT current_chapter FROM user_progress WHERE user_id='userA' AND manga_id='mangaX'`).Scan(&chapter)
	if err != nil {
		t.Fatalf("db query: %v", err)
	}
	if chapter != 42 {
		t.Fatalf("expected chapter 42, got %d", chapter)
	}
}

func TestIntegration_ErrorIsolation(t *testing.T) {
	br, cleanup := setupIntegrationEnv(t)
	defer cleanup()
	tcpConn := &bufConn{}
	br.RegisterTCPClient(tcpConn, "userA")
	gin.SetMode(gin.TestMode)
	router := gin.New()
	userHandler := user.NewHandler(br)
	router.PUT("/progress", func(c *gin.Context) {
		c.Set("user_id", "userA")
		userHandler.UpdateProgress(c)
	})
	reqBody := `{"manga_id":"doesNotExist","current_chapter":5,"status":"reading"}`
	req := httptest.NewRequest("PUT", "/progress", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code == 200 {
		t.Fatalf("expected non-200 for invalid manga")
	}
	time.Sleep(100 * time.Millisecond)
	if got := tcpConn.GetString(); got != "" {
		t.Fatalf("unexpected event received after failed operation: %s", got)
	}
}

func TestIntegration_MultiEventBroadcast(t *testing.T) {
	br, cleanup := setupIntegrationEnv(t)
	defer cleanup()
	tcpConn := &bufConn{}
	br.RegisterTCPClient(tcpConn, "userA")
	gin.SetMode(gin.TestMode)
	router := gin.New()
	userHandler := user.NewHandler(br)
	router.PUT("/progress", func(c *gin.Context) {
		c.Set("user_id", "userA")
		userHandler.UpdateProgress(c)
	})
	body1 := `{"manga_id":"mangaX","current_chapter":1,"status":"reading"}`
	req1 := httptest.NewRequest("PUT", "/progress", strings.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	resp1 := httptest.NewRecorder()
	router.ServeHTTP(resp1, req1)
	if resp1.Code != 200 {
		t.Fatalf("unexpected HTTP status #1: %d", resp1.Code)
	}
	body2 := `{"manga_id":"mangaX","current_chapter":2,"status":"reading"}`
	req2 := httptest.NewRequest("PUT", "/progress", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	resp2 := httptest.NewRecorder()
	router.ServeHTTP(resp2, req2)
	if resp2.Code != 200 {
		t.Fatalf("unexpected HTTP status #2: %d", resp2.Code)
	}
	time.Sleep(200 * time.Millisecond)
	raw := strings.TrimSpace(tcpConn.GetString())
	lines := strings.Split(raw, "\n")
	var events []bridge.Event
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var e bridge.Event
		if err := json.Unmarshal([]byte(ln), &e); err == nil {
			events = append(events, e)
		}
	}
	if len(events) < 2 {
		t.Fatalf("expected >=2 events, got %d: %v", len(events), lines)
	}
	if events[0].Type != bridge.EventTypeProgressUpdate || events[1].Type != bridge.EventTypeProgressUpdate {
		t.Fatalf("unexpected event types: %+v", events)
	}
	if v, ok := events[1].Data["chapter_id"].(float64); !ok || int(v) != 2 {
		t.Fatalf("expected last chapter 2, got %+v", events[1].Data["chapter_id"])
	}
}

func TestIntegration_Unauthorized_NoBroadcast(t *testing.T) {
	br, cleanup := setupIntegrationEnv(t)
	defer cleanup()
	tcpConn := &bufConn{}
	br.RegisterTCPClient(tcpConn, "userA")
	gin.SetMode(gin.TestMode)
	router := gin.New()
	userHandler := user.NewHandler(br)
	router.PUT("/progress", func(c *gin.Context) {
		userHandler.UpdateProgress(c)
	})
	req := httptest.NewRequest("PUT", "/progress", strings.NewReader(`{"manga_id":"mangaX","current_chapter":10}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.Code)
	}
	time.Sleep(100 * time.Millisecond)
	if got := strings.TrimSpace(tcpConn.GetString()); got != "" {
		t.Fatalf("expected no events, got: %s", got)
	}
}
