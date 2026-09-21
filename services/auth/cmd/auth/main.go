package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	notifierclient "authservice/internal/client/notifierclient"
	"authservice/internal/config"
	"authservice/internal/repository"
	"authservice/internal/service"
	transporthttp "authservice/internal/transport/http"
)

const (
	appVersion = "0.1.0"
	appName    = "Auth"

	serverReadHeaderTimeout = 5 * time.Second
	serverReadTimeout       = 10 * time.Second
	serverWriteTimeout      = 15 * time.Second
	serverIdleTimeout       = 60 * time.Second
)

// parseLevel преобразует строковый уровень логирования из конфигурации в slog.Level.
func parseLevel(s string) slog.Level {
	levels := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"info":  slog.LevelInfo,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
	}

	if lvl, ok := levels[strings.ToLower(s)]; ok {
		return lvl
	}

	return slog.LevelInfo
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %s\n", err)
		os.Exit(1)
	}

	opts := &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts)).With("service", appName, "version", appVersion)

	ctxInit, cancelInit := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelInit()

	db, err := pgxpool.New(ctxInit, cfg.DSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "pgxpool.new: %s\n", err)
		os.Exit(1)
	}

	err = db.Ping(ctxInit)
	if err != nil {
		fmt.Fprintf(os.Stderr, "db ping failed: %s\n", err)
		os.Exit(1)
	}
	logger.Info("database connected")

	// Схема БД управляется отдельным шагом деплоя (cmd/migrate, см.
	// Makefile: migrate-up / docker-compose.yml: сервис auth_migrate), а не
	// приложением — так безопаснее при нескольких репликах и позволяет
	// откатывать миграции независимо от релизов сервиса.

	repo := repository.NewPostgresRepository(db, logger)
	notifierClient, err := notifierclient.Dial(cfg.NotifierGRPCAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "notifier client failed: %s\n", err)
		os.Exit(1)
	}
	defer notifierClient.Close()

	authService, err := service.NewAuthService(
		repo,
		logger,
		cfg.JWTSecret,
		cfg.JWTAccessTTL,
		cfg.JWTRefreshTTL,
		notifierClient,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "service: %s\n", err)
		os.Exit(1)
	}
	healthService := service.NewHealthService(db)

	handler := transporthttp.NewHandler(authService, healthService, logger, appName, appVersion)
	router := transporthttp.NewRouter(handler, logger)

	srv := &http.Server{
		Addr:              cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
	}

	go func() {
		logger.Info("starting http server", "port", cfg.Port)

		err = srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Error starting http server", "error", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	received := <-sig // freeze here and waiting signal
	logger.Info("shutdown signal received", "signal", received.String())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	logger.Info("shutting down http server")
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("http server shutdown failed", "error", err)
	}

	logger.Info("waiting for background tasks")
	authService.Wait()

	router.Stop()

	logger.Info("closing database")
	db.Close()

	logger.Info("shutdown completed")
}
