package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/gabriela-miranda-leite/lastro-api/config"
	"github.com/gabriela-miranda-leite/lastro-api/internal/auth"
	"github.com/gabriela-miranda-leite/lastro-api/internal/couple"
	"github.com/gabriela-miranda-leite/lastro-api/internal/db"
	"github.com/gabriela-miranda-leite/lastro-api/internal/transaction"
	"github.com/gabriela-miranda-leite/lastro-api/internal/health"
	"github.com/gabriela-miranda-leite/lastro-api/internal/middleware"
	"github.com/gabriela-miranda-leite/lastro-api/internal/purge"
	"github.com/gabriela-miranda-leite/lastro-api/internal/user"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	database, err := db.New(cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.LoggerWithConfig(gin.LoggerConfig{
		SkipPaths: []string{"/health"},
	}))
	r.Use(gin.Recovery())

	r.GET("/health", health.NewHandler(database).Check)

	authHandler := auth.NewHandler(auth.NewService(auth.NewRepository(database), cfg.JWTSecret))
	v1 := r.Group("/v1")

	authGroup := v1.Group("/auth")
	authGroup.Use(middleware.RateLimit("5-M"))
	authHandler.RegisterRoutes(authGroup)

	protected := v1.Group("/")
	protected.Use(middleware.Auth(database, cfg.JWTSecret))
	authHandler.RegisterProtectedRoutes(protected)

	userHandler := user.NewHandler(user.NewService(user.NewRepository(database)))
	userHandler.RegisterRoutes(protected)

	coupleHandler := couple.NewHandler(couple.NewService(couple.NewRepository(database)))
	coupleHandler.RegisterRoutes(protected)

	txHandler := transaction.NewHandler(transaction.NewService(transaction.NewRepository(database)))
	txHandler.RegisterRoutes(protected)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	go func() {
		purge.New(database).Run(ctx)
		stop()
	}()

	slog.Info("starting lastro-api", "port", cfg.Port, "env", cfg.Env)
	if err := r.Run(":" + cfg.Port); err != nil {
		stop()
		slog.Error("server error", "error", err)
		_ = database.Close()
		os.Exit(1)
	}
}
