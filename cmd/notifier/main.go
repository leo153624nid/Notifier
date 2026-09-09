package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib" // need to init driver, but nothing to call

	"notifier/internal/config"
	"notifier/internal/notification"
	"notifier/internal/sender"
	"notifier/internal/store"
)

const (
	appVersion = "0.1.0"
	appName    = "Notifier"
)

type Server struct {
	store   store.Store
	logger  *slog.Logger
	senders map[string]sender.Sender
	wg      sync.WaitGroup
}

func (s *Server) OnShutdown() {
	s.wg.Wait()
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	data := struct {
		App     string `json:"app"`
		Version string `json:"version"`
		Status  string `json:"status"`
	}{
		App:     appName,
		Version: appVersion,
		Status:  "available",
	}

	js, err := json.Marshal(data)
	if err != nil {
		s.logger.Error("Marshal error", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(js)
}

func (s *Server) getNotification(w http.ResponseWriter, r *http.Request) {
	const op = "Server.getNotification"

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		bytes := fmt.Sprintf(`{"error": "%s"}`, notification.ErrInvalidId.Error())
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(bytes))
		return
	}

	n, err := s.store.GetById(id)
	if err != nil {
		if errors.Is(err, notification.ErrNotFound) {
			http.Error(w, "error: not found", http.StatusNotFound)
			return
		}

		s.logger.Error("%s: %w", op, err)
		http.Error(w, "error: internal error", http.StatusInternalServerError)
		return
	}

	js, err := json.Marshal(n)
	if err != nil {
		s.logger.Error("%s: %w", op, err)
		http.Error(w, "error: internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(js)
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	const op = "Server.listNotifications"

	notifications, err := s.store.GetAll()
	if err != nil {
		s.logger.Error("%s: getAll: %w", op, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if notifications == nil {
		notifications = []notification.Notification{}
	}

	js, err := json.Marshal(notifications)
	if err != nil {
		s.logger.Error("%s: marshal: %w", op, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(js)
}

func (s *Server) createNotification(w http.ResponseWriter, r *http.Request) {
	const op = "Server.createNotification"

	var n notification.Notification
	err := json.NewDecoder(r.Body).Decode(&n)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "Invalid request payload"}`))
		return
	}

	err = n.Validate()
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	id, err := s.store.Save(n)
	if err != nil {
		s.logger.Error("save failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	n.ID = id
	n.Status = "pending"

	sender, ok := s.senders[n.Channel]
	if ok {
		s.wg.Add(1)
		go func(n notification.Notification) {
			defer s.wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err = sender.Send(ctx, n)
			if err != nil {
				s.logger.Error("send failed", "id", id, "error", err)
				_ = s.store.UpdateStatus(id, "failed")
			} else {
				_ = s.store.UpdateStatus(id, "sent")
			}
		}(n)
	}

	js, err := json.Marshal(n)
	if err != nil {
		s.logger.Error("%s: %w", op, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	_, _ = w.Write(js)
}

func (s *Server) exportNotification(w http.ResponseWriter, r *http.Request) {
	const op = "Server.exportNotification"

	notifications, err := s.store.GetAll()
	if err != nil {
		s.logger.Error("list notifications failed", "op", op, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	err = WriteAuditLog("audit.log", notifications)
	if err != nil {
		s.logger.Error("export failed", "op", op, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]int{"exported": len(notifications)})
}

func (s *Server) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info(
			"request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start),
		)
	})
}

func contentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func NewServer(
	store store.Store,
	logger *slog.Logger,
	senders map[string]sender.Sender,
) (*Server, error) {
	const op = "NewServer"

	if store == nil {
		return nil, fmt.Errorf("%s: store is required", op)
	}
	if logger == nil {
		return nil, fmt.Errorf("%s: logger is required", op)
	}
	if len(senders) == 0 {
		return nil, fmt.Errorf("%s: senders is required", op)
	}

	return &Server{
		store:   store,
		logger:  logger,
		senders: senders,
	}, nil
}

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
	cfg := config.Load()

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

	s, err := NewServer(store, logger, senders)
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: %s\n", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.healthHandler)
	mux.HandleFunc("GET /api/notifications", s.listNotifications)
	mux.HandleFunc("GET /api/notifications/export", s.exportNotification)
	mux.HandleFunc("GET /api/notifications/{id}", s.getNotification)
	mux.HandleFunc("POST /api/notifications", s.createNotification)

	srv := &http.Server{
		Addr:    cfg.Port,
		Handler: s.logRequest(contentType(mux)),
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
