package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/abduromanov2020/tasks-api/internal/apperr"
	"github.com/abduromanov2020/tasks-api/internal/auth"
	"github.com/abduromanov2020/tasks-api/internal/domain"
)

const (
	CtxKeyActor   = "actor"
	CtxKeyUserID  = "user_id"
	CtxKeyTeamID  = "team_id"
	authHeader    = "Authorization"
	bearerPrefix  = "Bearer "
)

// JWTAuth verifies the bearer token via the configured issuer and stores
// the resulting Actor in gin.Context. The token itself is never logged.
func JWTAuth(issuer *auth.Issuer) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetHeader(authHeader)
		if raw == "" || !strings.HasPrefix(raw, bearerPrefix) {
			_ = c.Error(apperr.Unauthorized("Missing or malformed Authorization header"))
			c.Abort()
			return
		}
		token := strings.TrimPrefix(raw, bearerPrefix)
		claims, err := issuer.Parse(token)
		if err != nil {
			_ = c.Error(apperr.Wrap(401, "UNAUTHORIZED", "Invalid or expired token", err))
			c.Abort()
			return
		}
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			_ = c.Error(apperr.Unauthorized("Invalid token subject"))
			c.Abort()
			return
		}
		actor := domain.Actor{UserID: userID, TeamID: claims.TeamID}
		c.Set(CtxKeyActor, actor)
		c.Set(CtxKeyUserID, userID.String())
		c.Set(CtxKeyTeamID, claims.TeamID.String())

		// Attach actor IDs to the per-request logger.
		// (AccessLog runs first and has already bound request_id.)
		c.Next()
	}
}

// ActorFromCtx fetches the Actor put on the context by JWTAuth.
func ActorFromCtx(c *gin.Context) domain.Actor {
	v, _ := c.Get(CtxKeyActor)
	a, _ := v.(domain.Actor)
	return a
}
