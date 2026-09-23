package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"notifier/internal/audit"
	"notifier/internal/cache"
	"notifier/internal/config"
	"notifier/internal/repository"
	"notifier/internal/sender"
	"notifier/internal/service"
	transportgrpc "notifier/internal/transport/grpc"
	transporthttp "notifier/internal/transport/http"
	"notifier/internal/transport/kafka"
)

const (
	appVersion = "0.2.0"
	appName    = "Notifier"

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
	// Makefile: migrate-up / docker-compose.yml: сервис migrate), а не
	// приложением — так безопаснее при нескольких репликах и позволяет
	// откатывать миграции независимо от релизов сервиса.

	postgresRepo := repository.NewPostgresRepository(db, logger)

	ctxRedisInit, cancelRedisInit := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelRedisInit()

	cacheRepo := cache.NewRedisClient(
		cfg.RedisCfg.Addr,
		cfg.RedisCfg.Password,
		100*time.Millisecond, // dial
		100*time.Millisecond, // read
		100*time.Millisecond, // write
	)
	_, errRedis := cacheRepo.Ping(ctxRedisInit).Result()
	if errRedis != nil {
		fmt.Fprintf(os.Stderr, "redis ping failed: %s\n", errRedis)
		logger.Error("redis ping failed", "error", errRedis)
	}

	repo := repository.NewCachedNotificationRepo(postgresRepo, cacheRepo, logger, 30*time.Second)

	senders := map[string]sender.Sender{
		"console":  sender.LoggingSender{Sender: sender.NewConsoleSender(os.Stdout), Logger: logger},
		"email":    sender.LoggingSender{Sender: sender.NewEmailSender(os.Stdout), Logger: logger},
		"telegram": sender.LoggingSender{Sender: sender.TelegramSender{}, Logger: logger},
	}

	auditLogger := audit.NewLogger(cfg.AuditLogPath)

	notificationService, err := service.NewNotificationService(repo, senders, auditLogger, logger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "service: %s\n", err)
		os.Exit(1)
	}
	healthService := service.NewHealthService(db, cache.NewPinger(cacheRepo))

	handler := transporthttp.NewHandler(notificationService, healthService, logger, appName, appVersion)
	router := transporthttp.NewRouter(handler, cfg.JWTSecret, logger)

	srv := &http.Server{
		Addr:              cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
	}

	grpcServer := transportgrpc.NewGRPCServer(notificationService, logger)
	grpcLis, err := net.Listen("tcp", cfg.GRPCPort)
	if err != nil {
		fmt.Fprintf(os.Stderr, "grpc listen: %s\n", err)
		os.Exit(1)
	}

	loginConsumer := kafka.NewLoginConsumer(
		cfg.KafkaCfg.Brokers,
		cfg.KafkaCfg.LoginTopic,
		cfg.KafkaCfg.LoginGroupID,
		cfg.KafkaCfg.DLQTopic,
		notificationService,
		logger,
	)

	// MARK: - Start consumers
	logger.Info("starting consumers")
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	go loginConsumer.Run(consumerCtx)

	// MARK: - Start http server
	go func() {
		logger.Info("starting http server", "port", cfg.Port)

		err = srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("Error starting http server", "error", err)
		}
	}()

	// MARK: - Start gRPC server
	go func() {
		logger.Info("starting grpc server", "port", cfg.GRPCPort)

		if err := grpcServer.Serve(grpcLis); err != nil {
			logger.Error("Error starting grpc server", "error", err)
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

	logger.Info("shutting down consumers")
	consumerCancel()
	if err := loginConsumer.Close(); err != nil {
		logger.Error("close() login consumer failed", "error", err)
	}

	logger.Info("shutting down grpc server")
	grpcServer.GracefulStop()

	logger.Info("waiting for background tasks")
	notificationService.Wait()

	router.Stop()

	logger.Info("closing database")
	if err := cacheRepo.Close(); err != nil {
		logger.Error("closing cache repo failed", "error", err)
	}
	db.Close()

	logger.Info("shutdown completed")
}
