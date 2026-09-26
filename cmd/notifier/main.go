package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"notifier/internal/audit"
	"notifier/internal/cache"
	core_config "notifier/internal/core/config"
	core_logger "notifier/internal/core/logger"
	core_http_server "notifier/internal/core/transport/http/server"
	cached_repo "notifier/internal/repository/cache"
	"notifier/internal/repository/postgres"
	"notifier/internal/sender"
	"notifier/internal/service"
	transport_grpc "notifier/internal/transport/grpc"
	transport_http "notifier/internal/transport/http"
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
	ctx, cancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGTERM,
		syscall.SIGINT,
	)
	defer cancel()

	logger, loggerErr := core_logger.NewLogger(
		core_logger.NewConfigMust(),
	)
	if loggerErr != nil {
		fmt.Fprintf(os.Stderr, "logger: %s\n", loggerErr)
		os.Exit(1)
	}
	defer logger.Close()
	logger.Warn("starting notifier app")

	opts := &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts)).With("service", appName, "version", appVersion)
	cfg, cfgErr := core_config.Load()
	if cfgErr != nil {
		logger.Error("config load", zap.Error(cfgErr))
		os.Exit(1)
	}


	// MARK: - Start DB connection
	logger.Warn("starting database connection")

	ctxDbInit, cancelDbInit := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelDbInit()

	db, dbErr := pgxpool.New(ctxDbInit, cfg.DSN)
	if dbErr != nil {
		logger.Error("pgxpool.new", zap.Error(dbErr))
		os.Exit(1)
	}

	if err := db.Ping(ctxDbInit); err != nil {
		logger.Error("db ping failed", zap.Error(err))
		os.Exit(1)
	}
	logger.Warn("database connected")

	// Схема БД управляется отдельным шагом деплоя (cmd/migrate, см.
	// Makefile: migrate-up / docker-compose.yml: сервис migrate), а не
	// приложением — так безопаснее при нескольких репликах и позволяет
	// откатывать миграции независимо от релизов сервиса.

	postgresRepo := postgres.NewPostgresRepository(db, logger)

	// MARK: Init cache client
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
		logger.Error(
			"redis ping failed",
			zap.Error(errRedis),
		)
	}

	repo := cached_repo.NewCachedNotificationRepo(
		postgresRepo,
		cacheRepo,
		logger,
		30*time.Second, // TTL
	)

	senders := map[string]sender.Sender{
		"console":  sender.LoggingSender{Sender: sender.NewConsoleSender(os.Stdout), Logger: logger},
		"email":    sender.LoggingSender{Sender: sender.NewEmailSender(os.Stdout), Logger: logger},
		"telegram": sender.LoggingSender{Sender: sender.TelegramSender{}, Logger: logger},
	}

	auditLogger := audit.NewLogger(cfg.AuditLogPath)

	notificationService, err := service.NewNotificationService(
		repo,
		senders,
		auditLogger,
		// logger,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "service: %s\n", err)
		os.Exit(1)
	}
	healthService := service.NewHealthService(
		db,
		cache.NewPinger(cacheRepo),
	)

	notificationsTransportHTTP := transport_http.NewNotificationsHTTPHandler(
		notificationService,
		healthService,
		logger,
		appName,
		appVersion,
	)
	router := transport_http.NewRouter(
		notificationsTransportHTTP,
		cfg.JWTSecret,
		logger,
	)

	srv := &http.Server{
		Addr:              cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
	}

	grpcServer := transport_grpc.NewGRPCServer(notificationService, logger)
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

	notificationsRoutes := notificationsTransportHTTP.Routes()
	notificationsApiVersionRouter := core_http_server.NewApiVersionRouter(core_http_server.ApiVersion1)
	notificationsApiVersionRouter.RegisterRoutes(notificationsRoutes...)

	httpServer := core_http_server.NewHTTPServer(
		core_http_server.NewConfigMust(),
		logger,
	)
	httpServer.RegisterApiRoutes(notificationsApiVersionRouter)

	// MARK: - Start gRPC server
	go func() {
		logger.Warn(
			"starting gRPC server",
			zap.String("port", cfg.GRPCPort),
		)

		if err := grpcServer.Serve(grpcLis); err != nil {
			logger.Error(
				"Error starting grpc server",
				zap.Error(err),
			)
		}
	}()

	// MARK: - Start consumers
	logger.Warn("starting consumers")
	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	go loginConsumer.Run(consumerCtx)

	if err := httpServer.Run(ctx); err != nil { // freeze here
		logger.Error(
			"HTTP server run error",
			zap.Error(err),
		)
	}

	// MARK: Wait interrupt signal
	sig := make(chan os.Signal, 1) // TODO: delete ?
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	received := <-sig // freeze here and waiting signal
	logger.Warn(
		"shutdown signal received",
		zap.String("signal", received.String()),
	)

	shutDownCtx, shutDownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutDownCancel()

	// logger.Warn("shutting down http server")
	// if err := srv.Shutdown(shutDownCtx); err != nil {
	// 	logger.Error("http server shutdown failed", "error", err)
	// }

	logger.Warn("shutting down consumers")
	consumerCancel()
	if err := loginConsumer.Close(shutDownCtx); err != nil {
		logger.Error(
			"close() login consumer failed",
			zap.Error(err),
		)
	}

	logger.Warn("shutting down grpc server")
	grpcServer.GracefulStop()

	logger.Warn("waiting for background tasks")
	notificationService.Wait()

	// router.Stop() // TODO

	logger.Warn("closing database")
	if err := cacheRepo.Close(); err != nil {
		logger.Error(
			"closing cache repo failed",
			zap.Error(err),
		)
	}
	db.Close()

	logger.Warn("shutdown completed")
}
