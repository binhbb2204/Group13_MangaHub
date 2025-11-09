package metrics_test

import (
	"encoding/json"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/user"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/metrics"
	"github.com/gin-gonic/gin"
)

type mockConn struct {
	closed bool
}

func (m *mockConn) Read(p []byte) (int, error)         { return 0, nil }
func (m *mockConn) Write(p []byte) (int, error)        { return len(p), nil }
func (m *mockConn) Close() error                       { m.closed = true; return nil }
func (m *mockConn) LocalAddr() net.Addr                { return mockAddr{} }
func (m *mockConn) RemoteAddr() net.Addr               { return mockAddr{} }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

type mockAddr struct{}

func (a mockAddr) Network() string { return "tcp" }
func (a mockAddr) String() string  { return "127.0.0.1:0" }

func setupMetricsTest(t *testing.T) (*bridge.Bridge, func()) {
	metrics.Reset()

	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	if err := database.InitDatabase(dbPath); err != nil {
		t.Fatalf("init db: %v", err)
	}

	database.DB.Exec(`INSERT INTO users (id, username, email, password_hash) VALUES ('user1','user1','user1@example.com','hash1')`)
	database.DB.Exec(`INSERT INTO manga (id, title, author, status, total_chapters) VALUES ('manga1','Manga 1','Author','ongoing',100)`)
	database.DB.Exec(`INSERT INTO user_progress (user_id, manga_id, current_chapter, status, updated_at) VALUES ('user1','manga1',0,'reading', datetime('now'))`)

	logger.Init(logger.INFO, false, nil)
	br := bridge.NewBridge(logger.GetLogger())
	br.Start()

	cleanup := func() {
		br.Stop()
		database.Close()
	}

	return br, cleanup
}

func TestMetrics_InitialState(t *testing.T) {
	metrics.Reset()

	metricsHandler := metrics.NewHandler()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/metrics", metricsHandler.Metrics)

	req := httptest.NewRequest("GET", "/metrics", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != 200 {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if result["broadcasts_total"].(float64) != 0 {
		t.Fatalf("expected broadcasts_total=0, got %v", result["broadcasts_total"])
	}
	if result["broadcast_fails_total"].(float64) != 0 {
		t.Fatalf("expected broadcast_fails_total=0, got %v", result["broadcast_fails_total"])
	}
}

func TestMetrics_AfterSuccessfulBroadcast(t *testing.T) {
	br, cleanup := setupMetricsTest(t)
	defer cleanup()

	conn := &mockConn{}
	br.RegisterTCPClient(conn, "user1")

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
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != 200 {
		t.Fatalf("unexpected HTTP status: %d", resp.Code)
	}

	time.Sleep(100 * time.Millisecond)

	broadcasts := metrics.GetBroadcasts()
	if broadcasts < 1 {
		t.Fatalf("expected broadcasts >= 1, got %d", broadcasts)
	}

	activeConns := metrics.GetActiveConnections()
	if activeConns != 1 {
		t.Fatalf("expected active_connections=1, got %d", activeConns)
	}
}

func TestMetrics_Endpoint(t *testing.T) {
	br, cleanup := setupMetricsTest(t)
	defer cleanup()

	conn := &mockConn{}
	br.RegisterTCPClient(conn, "user1")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	userHandler := user.NewHandler(br)
	metricsHandler := metrics.NewHandler()

	router.PUT("/progress", func(c *gin.Context) {
		c.Set("user_id", "user1")
		userHandler.UpdateProgress(c)
	})
	router.GET("/metrics", metricsHandler.Metrics)

	reqBody := `{"manga_id":"manga1","current_chapter":10,"status":"reading"}`
	req := httptest.NewRequest("PUT", "/progress", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != 200 {
		t.Fatalf("unexpected HTTP status: %d", resp.Code)
	}

	time.Sleep(100 * time.Millisecond)

	metricsReq := httptest.NewRequest("GET", "/metrics", nil)
	metricsResp := httptest.NewRecorder()
	router.ServeHTTP(metricsResp, metricsReq)

	if metricsResp.Code != 200 {
		t.Fatalf("expected 200, got %d", metricsResp.Code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(metricsResp.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if result["broadcasts_total"].(float64) < 1 {
		t.Fatalf("expected broadcasts_total >= 1, got %v", result["broadcasts_total"])
	}
	if result["active_connections"].(float64) != 1 {
		t.Fatalf("expected active_connections=1, got %v", result["active_connections"])
	}
}
