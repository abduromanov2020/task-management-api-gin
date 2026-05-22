package middleware

import (
	"context"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	HeaderRequestID = "X-Request-ID"
	CtxKeyRequestID = "request_id"
)

type ridCtxKey struct{}

// RequestID generates (or accepts) a request UUID and stores it in both
// gin.Context and context.Context so the downstream logger / error middleware
// can pick it up uniformly.
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader(HeaderRequestID)
		if _, err := uuid.Parse(rid); err != nil || rid == "" {
			rid = uuid.NewString()
		}
		c.Writer.Header().Set(HeaderRequestID, rid)
		c.Set(CtxKeyRequestID, rid)
		ctx := context.WithValue(c.Request.Context(), ridCtxKey{}, rid)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

// RequestIDFromCtx returns the request id stored in ctx, or empty.
func RequestIDFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(ridCtxKey{}).(string); ok {
		return v
	}
	return ""
}
