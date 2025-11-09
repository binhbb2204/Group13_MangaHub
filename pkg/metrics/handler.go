package metrics

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Handler struct{}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Metrics(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"broadcasts_total":      GetBroadcasts(),
		"broadcast_fails_total": GetBroadcastFails(),
		"active_connections":    GetActiveConnections(),
	})
}
