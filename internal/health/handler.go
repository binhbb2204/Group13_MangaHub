package health

import (
	"net/http"

	"github.com/binhbb2204/Manga-Hub-Group13/internal/bridge"
	"github.com/binhbb2204/Manga-Hub-Group13/pkg/database"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	bridge *bridge.Bridge
}

func NewHandler(br *bridge.Bridge) *Handler {
	return &Handler{bridge: br}
}

func (h *Handler) Healthz(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "alive"})
}

func (h *Handler) Readyz(c *gin.Context) {
	if database.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "reason": "database_not_initialized"})
		return
	}

	if err := database.DB.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "reason": "database_ping_failed"})
		return
	}

	if h.bridge.GetTotalConnectionCount() < 0 {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not_ready", "reason": "bridge_not_running"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
