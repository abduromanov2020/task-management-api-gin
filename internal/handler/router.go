package handler

import (
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "github.com/abduromanov2020/tasks-api/docs"
	"github.com/abduromanov2020/tasks-api/internal/auth"
	"github.com/abduromanov2020/tasks-api/internal/middleware"
)

type Deps struct {
	Auth         *AuthHandler
	Tasks        *TaskHandler
	JWT          *auth.Issuer
	CORSOrigins  []string
	RateLimitRPM int
}

// Register wires every HTTP route into the provided engine. Middlewares
// declared on r already include RequestID, AccessLog, Recovery, ErrorHandler,
// SecurityHeaders, BodyLimit (see cmd/api/main.go).
func Register(r *gin.Engine, d Deps) {
	corsCfg := cors.Config{
		AllowOrigins:     d.CORSOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type", "Idempotency-Key", "X-Request-ID"},
		ExposeHeaders:    []string{"X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           300,
	}
	r.Use(cors.New(corsCfg))

	r.GET("/healthz", Health)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))

	authGroup := r.Group("/auth")
	authLimiter := middleware.NewIPRateLimiter(d.RateLimitRPM)
	authGroup.Use(authLimiter.Middleware())
	{
		authGroup.POST("/register", d.Auth.Register)
		authGroup.POST("/login", d.Auth.Login)
	}

	api := r.Group("/")
	api.Use(middleware.JWTAuth(d.JWT))
	{
		api.POST("/tasks", d.Tasks.Create)
		api.GET("/tasks", d.Tasks.List)
		api.GET("/tasks/:id", d.Tasks.Get)
		api.PUT("/tasks/:id", d.Tasks.Update)
		api.DELETE("/tasks/:id", d.Tasks.Delete)
		api.POST("/tasks/:id/assign", d.Tasks.Assign)
	}
}
