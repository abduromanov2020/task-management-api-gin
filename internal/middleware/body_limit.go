package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BodyLimit caps each request body to maxBytes. Returns 413 if exceeded.
func BodyLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}
