package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Health is a liveness check; no auth.
//
// @Summary Liveness check
// @Tags    health
// @Produce json
// @Success 200 {object} map[string]string
// @Router  /healthz [get]
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
