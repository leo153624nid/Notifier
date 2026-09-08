package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"sync"
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

type Store = store.Store
type Notification = notification.Notification
type Sender = sender.Sender
type LoggingSender = sender.LoggingSender
type ConsoleSender = sender.ConsoleSender
type EmailSender = sender.EmailSender
type TelegramSender = sender.TelegramSender

type Server struct {
	store   Store
	logger  *slog.Logger
	senders map[string]Sender
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

	// w.Header().Set("Content-type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(js)
}

func (s *Server) getNotification(w http.ResponseWriter, r *http.Request) {
	const op = "Server.getNotification"

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		bytes := fmt.Sprintf(`{"error": "%s"}`, notification.ErrInvalidId.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(bytes))
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
	w.Write(js)
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	const op = "Server.listNotifications"

	notifications, err := s.store.GetAll()
	if err != nil {
		s.logger.Error("list notifications failed", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if notifications == nil {
		notifications = []Notification{}
	}

	js, err := json.Marshal(notifications)
	if err != nil {
		s.logger.Error("Marshal error", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(js)
}

func (s *Server) createNotification(w http.ResponseWriter, r *http.Request) {
	const op = "Server.createNotification"

	var n Notification
	err := json.NewDecoder(r.Body).Decode(&n)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "Invalid request payload"}`))
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
		go func() {
			defer s.wg.Done()
			err := sender.Send(n)
			if err != nil {
				s.logger.Error("send failed", "id", id, "error", err)
				_ = s.store.UpdateStatus(id, "failed")
			} else {
				_ = s.store.UpdateStatus(id, "sent")
			}
		}()
	}

	js, err := json.Marshal(n)
	if err != nil {
		s.logger.Error("%s: %w", op, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	w.Write(js)
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

func NewServer(store Store, logger *slog.Logger, senders map[string]Sender) (*Server, error) {
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
	json.NewEncoder(w).Encode(map[string]int{"exported": len(notifications)})
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	pgCfg := config.LoadPostgresConfig()
	db, err := sql.Open("pgx", pgCfg.DSN())
	if err != nil {
		fmt.Fprintf(os.Stderr, "sql.open: %s\n", err)
		os.Exit(1)
	}
	defer db.Close()

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

	store := store.NewPostgresStore(db)

	senders := map[string]Sender{
		"console":  LoggingSender{Sender: sender.NewConsoleSender(os.Stdout), Logger: logger},
		"email":    LoggingSender{Sender: sender.NewEmailSender(os.Stdout), Logger: logger},
		"telegram": LoggingSender{Sender: TelegramSender{}, Logger: logger},
	}

	s, err := NewServer(store, logger, senders)
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: %s\n", err)
		os.Exit(1)
	}
	defer s.OnShutdown()

	s.logger.Info("starting server", "app", appName, "version", appVersion, "port", 8080)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.healthHandler)
	mux.HandleFunc("GET /api/notifications", s.listNotifications)
	mux.HandleFunc("GET /api/notifications/export", s.exportNotification)
	mux.HandleFunc("GET /api/notifications/{id}", s.getNotification)
	mux.HandleFunc("POST /api/notifications", s.createNotification)

	err = http.ListenAndServe(":8080", s.logRequest(contentType(mux)))
	if err != nil {
		s.logger.Error("Error starting server", "error", err)
	}
}
