package bridge_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/user"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
)

type concurrentMockAddr struct{ s string }

func (a concurrentMockAddr) Network() string { return "tcp" }
func (a concurrentMockAddr) String() string  { return a.s }

type concurrentBufConn struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func (c *concurrentBufConn) Read(p []byte) (int, error) { return 0, nil }
func (c *concurrentBufConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.buf.Write(p)
}
func (c *concurrentBufConn) Close() error                       { return nil }
func (c *concurrentBufConn) LocalAddr() net.Addr                { return concurrentMockAddr{"127.0.0.1:0"} }
func (c *concurrentBufConn) RemoteAddr() net.Addr               { return concurrentMockAddr{"127.0.0.1:0"} }
func (c *concurrentBufConn) SetDeadline(t time.Time) error      { return nil }
func (c *concurrentBufConn) SetReadDeadline(t time.Time) error  { return nil }
func (c *concurrentBufConn) SetWriteDeadline(t time.Time) error { return nil }

func setupConcurrentTest(t *testing.T) (*bridge.Bridge, func()) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	if err := database.InitDatabase(dbPath); err != nil {
		t.Fatalf("init db: %v", err)
	}

	database.DB.Exec(`INSERT INTO users (id, username, email, password_hash) VALUES ('userA','userA','userA@example.com','hashA')`)
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

func TestConcurrency_ParallelProgressUpdates(t *testing.T) {
	br, cleanup := setupConcurrentTest(t)
	defer cleanup()

	tcpConn := &concurrentBufConn{}
	br.RegisterTCPClient(tcpConn, "userA")

	gin.SetMode(gin.TestMode)
	router := gin.New()
	userHandler := user.NewHandler(br)

	router.PUT("/progress", func(c *gin.Context) {
		c.Set("user_id", "userA")
		userHandler.UpdateProgress(c)
	})

	numUpdates := 20
	var wg sync.WaitGroup
	wg.Add(numUpdates)

	for i := 1; i <= numUpdates; i++ {
		go func(chapter int) {
			defer wg.Done()
			reqBody := fmt.Sprintf(`{"manga_id":"mangaX","current_chapter":%d,"status":"reading"}`, chapter)
			req := httptest.NewRequest("PUT", "/progress", strings.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			if resp.Code != 200 {
				t.Errorf("unexpected HTTP status for chapter %d: %d", chapter, resp.Code)
			}
		}(i)
	}

	wg.Wait()
	time.Sleep(300 * time.Millisecond)

	var finalChapter int
	err := database.DB.QueryRow(`SELECT current_chapter FROM user_progress WHERE user_id='userA' AND manga_id='mangaX'`).Scan(&finalChapter)
	if err != nil {
		t.Fatalf("db query: %v", err)
	}

	if finalChapter < 1 || finalChapter > numUpdates {
		t.Fatalf("expected final chapter between 1 and %d, got %d", numUpdates, finalChapter)
	}

	tcpConn.mu.Lock()
	raw := strings.TrimSpace(tcpConn.buf.String())
	tcpConn.mu.Unlock()

	lines := strings.Split(raw, "\n")
	var events []bridge.Event
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var e bridge.Event
		if err := json.Unmarshal([]byte(ln), &e); err == nil {
			if e.Type == bridge.EventTypeProgressUpdate {
				events = append(events, e)
			}
		}
	}

	if len(events) != numUpdates {
		t.Fatalf("expected %d progress_update events, got %d", numUpdates, len(events))
	}
}
