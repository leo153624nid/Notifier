package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // need to init driver, but nothing to call

	"notifier/internal/config"
	"notifier/internal/sender"
	"notifier/internal/store"
)

const (
	appVersion = "0.2.0"
	appName    = "Notifier"

	serverReadHeaderTimeout = 5 * time.Second
	serverReadTimeout       = 10 * time.Second
	serverWriteTimeout      = 15 * time.Second
	serverIdleTimeout       = 60 * time.Second
)

type Server struct {
	store       store.Store
	db          *sql.DB
	logger      *slog.Logger
	senders     map[string]sender.Sender
	auditLogger *AuditLogger
	wg          sync.WaitGroup
}

func NewServer(
	store store.Store,
	db *sql.DB,
	logger *slog.Logger,
	senders map[string]sender.Sender,
	auditLogger *AuditLogger,
) (*Server, error) {
	const op = "NewServer"

	if store == nil {
		return nil, fmt.Errorf("%s: store is required", op)
	}
	// Check db == nil not needed
	if logger == nil {
		return nil, fmt.Errorf("%s: logger is required", op)
	}
	if len(senders) == 0 {
		return nil, fmt.Errorf("%s: senders is required", op)
	}
	if auditLogger == nil {
		return nil, fmt.Errorf("%s: auditLogger is required", op)
	}

	return &Server{
		store:       store,
		db:          db,
		logger:      logger,
		senders:     senders,
		auditLogger: auditLogger,
	}, nil
}

func (s *Server) OnShutdown() {
	s.wg.Wait()
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %s\n", err)
		os.Exit(1)
	}

	opts := &slog.HandlerOptions{Level: parseLevel(cfg.LogLevel)}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, opts)).With("service", appName, "version", appVersion)

	db, err := sql.Open("pgx", cfg.DSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sql.open: %s\n", err)
		os.Exit(1)
	}

	err = db.Ping()
	if err != nil {
		fmt.Fprintf(os.Stderr, "db ping failed: %s\n", err)
		os.Exit(1)
	}
	logger.Info("database connected")

	const createTable = `
	CREATE TABLE IF NOT EXISTS notifications (
		id SERIAL PRIMARY KEY,
		recipient TEXT NOT NULL,
		subject TEXT NOT NULL,
		body TEXT,
		channel TEXT,
		is_urgent BOOLEAN DEFAULT false,
		status TEXT NOT NULL DEFAULT 'pending'
	)
	`
	_, err = db.Exec(createTable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create table failed: %s\n", err)
		os.Exit(1)
	}
	logger.Info("table ready")

	store := store.NewPostgresStore(db, logger)

	senders := map[string]sender.Sender{
		"console":  sender.LoggingSender{Sender: sender.NewConsoleSender(os.Stdout), Logger: logger},
		"email":    sender.LoggingSender{Sender: sender.NewEmailSender(os.Stdout), Logger: logger},
		"telegram": sender.LoggingSender{Sender: sender.TelegramSender{}, Logger: logger},
	}

	auditLogger := NewAuditLogger(cfg.AuditLogPath)

	s, err := NewServer(store, db, logger, senders, auditLogger)
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: %s\n", err)
		os.Exit(1)
	}

	auth := authMiddleware(cfg.APIkey, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.healthHandler)
	mux.Handle("GET /api/v1/notifications", auth(http.HandlerFunc(s.listNotifications)))
	mux.Handle("GET /api/v1/notifications/export", auth(http.HandlerFunc(s.exportNotification)))
	mux.Handle("GET /api/v1/notifications/{id}", auth(http.HandlerFunc(s.getNotification)))
	mux.Handle("POST /api/v1/notifications", auth(http.HandlerFunc(s.createNotification)))

	srv := &http.Server{
		Addr:              cfg.Port,
		Handler:           requestID(s.logRequest(contentType(mux))),
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
	}

	go func() {
		s.logger.Info("starting http server", "port", cfg.Port)

		err = srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logger.Error("Error starting http server", "error", err)
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
	s.OnShutdown()

	logger.Info("closing database")
	if err := db.Close(); err != nil {
		logger.Error("db closing failed", "error", err)
	}

	logger.Info("shutdown completed")
}
