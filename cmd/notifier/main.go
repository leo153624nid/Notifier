package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
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
	transport_kafka "notifier/internal/transport/kafka"
)

const (
	appVersion = "0.3.0"
	appName    = "Notifier"

	shutdownTimeout = 30 * time.Second
)

func main() {
	runCtx, runCancel := signal.NotifyContext(
		context.Background(),
		syscall.SIGTERM,
		syscall.SIGINT,
	)
	defer runCancel()

	logger, loggerErr := core_logger.NewLogger(
		core_logger.NewConfigMust(),
	)
	if loggerErr != nil {
		fmt.Fprintf(os.Stderr, "logger: %s\n", loggerErr)
		os.Exit(1)
	}
	defer logger.Close()
	logger = logger.With(
		zap.String("app", appName),
		zap.String("version", appVersion),
	)
	logger.Warn("start notifier app ...")

	cfg, cfgErr := core_config.Load()
	if cfgErr != nil {
		logger.Error("config load failed", zap.Error(cfgErr))
		os.Exit(1)
	}

	// MARK: - Start DB connection
	logger.Warn("start database connection ...")

	ctxDbInit, cancelDbInit := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelDbInit()

	postgresCfg := postgres.LoadConfig()
	db, dbErr := pgxpool.New(ctxDbInit, postgresCfg.DSN())
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

	// MARK: Start cache client
	logger.Warn("start cache client ...")
	ctxRedisInit, cancelRedisInit := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelRedisInit()

	redisCfg := cache.LoadConfig()
	cacheRepo := cache.NewRedisClient(
		redisCfg.Addr,
		redisCfg.Password,
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
	logger.Warn("cache client connected")

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

	notificationService, serviceErr := service.NewNotificationService(
		repo,
		senders,
		auditLogger,
		// logger,
	)
	if serviceErr != nil {
		logger.Error(
			"notifications service init failed",
			zap.Error(serviceErr),
		)
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
	notificationsRoutes := notificationsTransportHTTP.Routes()
	notificationsApiVersionRouter := core_http_server.NewApiVersionRouter(core_http_server.ApiVersion1)
	notificationsApiVersionRouter.RegisterRoutes(notificationsRoutes...)

	httpServer := core_http_server.NewHTTPServer(
		core_http_server.NewConfigMust(),
		logger,
	)
	httpServer.RegisterApiRoutes(notificationsApiVersionRouter)

	grpcServer := transport_grpc.NewGRPCServer(notificationService, logger)
	grpcCfg := transport_grpc.LoadConfig()
	grpcLis, grpcErr := net.Listen("tcp", grpcCfg.GRPCPort)
	if grpcErr != nil {
		logger.Error(
			"grpc listen failed",
			zap.Error(grpcErr),
		)
		os.Exit(1)
	}

	kafkaCfg := transport_kafka.LoadConfig()
	loginConsumer := transport_kafka.NewLoginConsumer(
		kafkaCfg.Brokers,
		kafkaCfg.LoginTopic,
		kafkaCfg.LoginGroupID,
		kafkaCfg.DLQTopic,
		notificationService,
		logger,
	)

	// MARK: - Start HTTP server
	go func() {
		if err := httpServer.Run(runCtx); err != nil { // freeze here
			logger.Error(
				"HTTP server run error",
				zap.Error(err),
			)
		}
	}()

	// MARK: - Start gRPC server
	go func() {
		logger.Warn(
			"start gRPC server ...",
			zap.String("port", grpcCfg.GRPCPort),
		)

		if err := grpcServer.Serve(grpcLis); err != nil {
			logger.Error(
				"Error starting grpc server",
				zap.Error(err),
			)
		}
	}()

	// MARK: - Start consumers
	logger.Warn("start consumers ...")

	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	go loginConsumer.Run(consumerCtx)

	// MARK: Wait interrupt signal
	<-runCtx.Done() // freeze here and waiting signal
	logger.Warn("shutdown signal received")

	shutDownCtx, shutDownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutDownCancel()

	logger.Warn("shutdown consumers ...")
	consumerCancel()
	if err := loginConsumer.Close(shutDownCtx); err != nil {
		logger.Error(
			"close() login consumer failed",
			zap.Error(err),
		)
	}

	logger.Warn("shutdown down gRPC server ...")
	grpcServer.GracefulStop()

	logger.Debug("waiting for background tasks ...")
	notificationService.Wait()

	// router.Stop() // TODO

	logger.Debug("close cache ...")
	if err := cacheRepo.Close(); err != nil {
		logger.Error(
			"closing cache repo failed",
			zap.Error(err),
		)
	}
	logger.Debug("close database ...")
	db.Close()

	logger.Warn("shutdown completed")
}
