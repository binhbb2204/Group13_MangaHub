package health_test

import (
	"net/http/httptest"
	"testing"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/internal/health"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/logger"
	"github.com/gin-gonic/gin"
)

func setupHealthTest(t *testing.T) (*health.Handler, func()) {
	tmpDir := t.TempDir()
	dbPath := tmpDir + "/test.db"
	if err := database.InitDatabase(dbPath); err != nil {
		t.Fatalf("init db: %v", err)
	}

	logger.Init(logger.INFO, false, nil)
	br := bridge.NewBridge(logger.GetLogger())
	br.Start()

	handler := health.NewHandler(br)

	cleanup := func() {
		br.Stop()
		database.Close()
	}

	return handler, cleanup
}

func TestHealthz_AlwaysReturnsOK(t *testing.T) {
	handler, cleanup := setupHealthTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/healthz", handler.Healthz)

	req := httptest.NewRequest("GET", "/healthz", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != 200 {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	body := resp.Body.String()
	if body != `{"status":"alive"}` {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestReadyz_HealthySystem(t *testing.T) {
	handler, cleanup := setupHealthTest(t)
	defer cleanup()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/readyz", handler.Readyz)

	req := httptest.NewRequest("GET", "/readyz", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}

	body := resp.Body.String()
	if body != `{"status":"ready"}` {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestReadyz_DatabaseClosed(t *testing.T) {
	handler, cleanup := setupHealthTest(t)
	defer cleanup()

	database.Close()

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/readyz", handler.Readyz)

	req := httptest.NewRequest("GET", "/readyz", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != 503 {
		t.Fatalf("expected 503, got %d", resp.Code)
	}
}
