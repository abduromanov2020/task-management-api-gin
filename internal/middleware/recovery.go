package middleware

import (
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/abduromanov2020/tasks-api/internal/logger"
)

// Recovery catches panics, logs at ERROR with stack, returns a generic 500
// envelope. The stack never leaks to the client.
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				log := logger.FromCtx(c.Request.Context())
				log.Error("panic recovered",
					"error", r,
					"stack", string(debug.Stack()),
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
				)
				if !c.Writer.Written() {
					rid, _ := c.Get(CtxKeyRequestID)
					ridStr, _ := rid.(string)
					c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
						"status":     "error",
						"code":       "INTERNAL_ERROR",
						"message":    "Internal server error",
						"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
						"request_id": ridStr,
					})
				}
			}
		}()
		c.Next()
	}
}
