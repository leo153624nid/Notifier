package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
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

type ctxKey struct{}

var requestIDKey = ctxKey{}

func (s *Server) OnShutdown() {
	s.wg.Wait()
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	const op = "Server.health"

	ctx, cancel := context.WithTimeout(r.Context(), 1*time.Second)
	defer cancel()

	err := s.db.PingContext(ctx)
	if err != nil {
		s.logger.Error("db ping failed", "op", op, "error", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "db unavailable", "error": err.Error()})
		return
	}

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
		s.logger.Error("marshal failed", "op", op, "error", err)
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

		s.logger.Error("get from store failed", "op", op, "error", err)
		http.Error(w, "error: internal error", http.StatusInternalServerError)
		return
	}

	js, err := json.Marshal(n)
	if err != nil {
		s.logger.Error("marshal failed", "op", op, "error", err)
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
		s.logger.Error("store getAll failed", "op", op, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if notifications == nil {
		notifications = []notification.Notification{}
	}

	js, err := json.Marshal(notifications)
	if err != nil {
		s.logger.Error("marshal failed", "op", op, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(js)
}

func (s *Server) createNotification(w http.ResponseWriter, r *http.Request) {
	const op = "Server.createNotification"

	reqID := getRequestID(r.Context())
	reqLogger := s.logger.With("request_id", reqID)

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

			if senderErr := sender.Send(ctx, n); senderErr != nil {
				reqLogger.Error("send failed", "id", id, "error", senderErr)
				_ = s.store.UpdateStatus(id, "failed")
			} else {
				reqLogger.Info("notification sent", "id", id, "channel", n.Channel)
				_ = s.store.UpdateStatus(id, "sent")
			}
		}(n)
	}

	js, err := json.Marshal(n)
	if err != nil {
		s.logger.Error("marshal failed", "op", op, "error", err)
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
		s.logger.Error("get all notifications failed", "op", op, "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	err = s.auditLogger.Write(notifications)
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
			"request_id", getRequestID(r.Context()),
		)
	})
}

func contentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func authMiddleware(apiKey string, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-KEY")

			if subtle.ConstantTimeCompare([]byte(key), []byte(apiKey)) != 1 {
				logger.Warn("auth failed", "path", r.URL.Path, "method", r.Method, "remote", r.RemoteAddr)
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid or missing api key"})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newRequestID()
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
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

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func getRequestID(ctx context.Context) string {
	id, ok := ctx.Value(requestIDKey).(string)
	if !ok {
		return ""
	}
	return id
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
	mux.Handle("GET /api/notifications", auth(http.HandlerFunc(s.listNotifications)))
	mux.Handle("GET /api/notifications/export", auth(http.HandlerFunc(s.exportNotification)))
	mux.Handle("GET /api/notifications/{id}", auth(http.HandlerFunc(s.getNotification)))
	mux.Handle("POST /api/notifications", auth(http.HandlerFunc(s.createNotification)))

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
