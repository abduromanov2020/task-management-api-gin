// Package main is the composition root for the tasks-api binary.
//
// @title       Tasks API
// @version     1.0
// @description Team-scoped task management API with idempotent writes and JWT auth.
// @BasePath    /
// @securityDefinitions.apikey ApiKeyAuth
// @in   header
// @name Authorization
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/abduromanov2020/tasks-api/internal/auth"
	"github.com/abduromanov2020/tasks-api/internal/config"
	"github.com/abduromanov2020/tasks-api/internal/handler"
	"github.com/abduromanov2020/tasks-api/internal/logger"
	"github.com/abduromanov2020/tasks-api/internal/middleware"
	"github.com/abduromanov2020/tasks-api/internal/repository/pg"
	"github.com/abduromanov2020/tasks-api/internal/usecase"
)

var (
	version = "dev"
	gitSHA  = "unknown"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	zl, err := logger.New(cfg.LogLevel, cfg.Env)
	if err != nil {
		return err
	}
	defer func() { _ = zl.Sync() }()
	base := logger.Wrap(zl)

	base.Info("server.start",
		"event", "server.start",
		"version", version,
		"git_sha", gitSHA,
		"port", cfg.Port,
		"env", cfg.Env,
		"jwt_ttl", cfg.JWTTTL.String(),
		"db_pool_max", cfg.DBMaxConns,
		"cors_origins", cfg.CORSOrigins,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := pg.NewPool(ctx, cfg.DatabaseURL, cfg.DBMaxConns, cfg.DBMinConns)
	cancel()
	if err != nil {
		return err
	}
	defer pool.Close()

	uow := pg.NewUoW(pool)
	hasher := auth.NewPasswordHasher()
	jwtIssuer := auth.NewIssuer(cfg.JWTSecret, cfg.JWTTTL, cfg.JWTIssuer, cfg.JWTAudience)

	authUC := usecase.NewAuthUsecase(uow, hasher, jwtIssuer, cfg.JWTTTL, base)
	taskUC := usecase.NewTaskUsecase(uow, base)

	authH := handler.NewAuthHandler(authUC)
	taskH := handler.NewTaskHandler(taskUC)

	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(
		middleware.RequestID(),
		middleware.AccessLog(base),
		middleware.Recovery(),
		middleware.ErrorHandler(),
		middleware.SecurityHeaders(),
		middleware.BodyLimit(1<<20),
	)

	handler.Register(r, handler.Deps{
		Auth:         authH,
		Tasks:        taskH,
		JWT:          jwtIssuer,
		CORSOrigins:  cfg.CORSOrigins,
		RateLimitRPM: cfg.AuthRateLimitRPM,
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		return err
	case sig := <-stop:
		base.Info("server.shutdown", "event", "server.shutdown", "signal", sig.String())
	}

	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	return srv.Shutdown(shutCtx)
}
