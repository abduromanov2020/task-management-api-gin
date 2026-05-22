package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/abduromanov2020/tasks-api/internal/domain"
	"github.com/abduromanov2020/tasks-api/internal/logger"
)

// AccessLog emits one structured JSON log line per request and exposes a
// per-request domain.Logger (with request_id bound) on context.Context so
// usecases and handlers can call logger.FromCtx(ctx) for child logs.
func AccessLog(base domain.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		rid, _ := c.Get(CtxKeyRequestID)
		ridStr, _ := rid.(string)

		reqLog := base.With("request_id", ridStr)
		c.Request = c.Request.WithContext(logger.Into(c.Request.Context(), reqLog))

		c.Next()

		latency := time.Since(start).Milliseconds()
		status := c.Writer.Status()
		path := c.Request.URL.Path
		if raw := c.Request.URL.RawQuery; raw != "" {
			path = path + "?" + raw
		}

		kv := []any{
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"latency_ms", latency,
			"client_ip", c.ClientIP(),
		}
		if errs := c.Errors; len(errs) > 0 {
			kv = append(kv, "error", errs.Last().Error())
		}

		switch {
		case status >= 500:
			reqLog.Error("request", kv...)
		case status >= 400:
			reqLog.Warn("request", kv...)
		default:
			reqLog.Info("request", kv...)
		}
	}
}
