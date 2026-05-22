package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"github.com/abduromanov2020/tasks-api/internal/apperr"
	"github.com/abduromanov2020/tasks-api/internal/logger"
)

// ErrorHandler runs after the route handler and inspects c.Errors. If any
// error was attached via c.Error(), it is translated to the canonical JSON
// envelope. Handlers must NEVER write JSON for error paths themselves.
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		errs := c.Errors
		if len(errs) == 0 {
			return
		}
		ae := apperr.From(errs.Last().Err)
		if ae == nil {
			return
		}

		// Log the underlying cause at the appropriate level. The envelope
		// returned to the client never includes the cause or stack.
		log := logger.FromCtx(c.Request.Context())
		fields := []any{
			"error_code", ae.Code,
			"http_status", ae.HTTPStatus,
		}
		if ae.Cause != nil {
			fields = append(fields, "cause", ae.Cause.Error())
		}
		switch {
		case ae.HTTPStatus >= 500:
			log.Error("request error", fields...)
		default:
			log.Warn("request error", fields...)
		}

		rid, _ := c.Get(CtxKeyRequestID)
		ridStr, _ := rid.(string)

		body := gin.H{
			"status":     "error",
			"code":       ae.Code,
			"message":    ae.Message,
			"timestamp":  time.Now().UTC().Format(time.RFC3339Nano),
			"request_id": ridStr,
		}
		if len(ae.Details) > 0 {
			body["details"] = ae.Details
		}
		// If a previous middleware already wrote (rare), don't double-write.
		if c.Writer.Written() {
			return
		}
		c.AbortWithStatusJSON(ae.HTTPStatus, body)
	}
}
