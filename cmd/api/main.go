package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joe/distributed-rate-limiter/internal/auth"
	"github.com/joe/distributed-rate-limiter/internal/config"
	"github.com/joe/distributed-rate-limiter/internal/db"
	dbsqlc "github.com/joe/distributed-rate-limiter/internal/db/sqlc"
	"github.com/joe/distributed-rate-limiter/internal/handlers"
	"github.com/joe/distributed-rate-limiter/internal/redisstate"
	"github.com/joe/distributed-rate-limiter/internal/routes"
)

var version = "dev"

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger := newLogger(cfg)
	startupContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	dbPool, err := db.Open(startupContext, cfg.Postgres.DSN)
	if err != nil {
		panic(err)
	}
	defer dbPool.Close()

	if err := dbPool.Ping(startupContext); err != nil {
		panic(err)
	}

	redisClient := redisstate.NewClient(cfg.Redis.Addr, cfg.Redis.DB)
	defer redisClient.Close()

	apiKeyQueries := dbsqlc.New(dbPool)
	apiKeyCodec := auth.NewAPIKeyCodec(cfg.Security.KeyHashPepper)
	apiKeyCache := redisstate.NewAPIKeyAuthCache(redisClient, cfg.Redis.APIKeyCacheTTL)
	apiKeyService := auth.NewAPIKeyService(apiKeyQueries, apiKeyCodec, apiKeyCache, logger)

	router := routes.New(cfg, logger, version, time.Now().UTC(), routes.Dependencies{
		APIKeys: handlers.NewAPIKeysHandler(apiKeyService),
	})

	server := &http.Server{
		Addr:              cfg.Server.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("server starting",
			slog.String("addr", cfg.Server.Addr),
			slog.String("environment", cfg.AppEnv),
			slog.String("redis_addr", cfg.Redis.Addr),
		)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	waitForShutdown(logger, server)
}

func newLogger(cfg config.Config) *slog.Logger {
	level := slog.LevelInfo
	if cfg.AppEnv == "development" {
		level = slog.LevelDebug
	}

	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}

func waitForShutdown(logger *slog.Logger, server *http.Server) {
	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	<-signalContext.Done()
	logger.Info("shutdown signal received")

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		logger.Error("graceful shutdown failed", slog.String("error", err.Error()))
		return
	}

	logger.Info("server stopped")
}
