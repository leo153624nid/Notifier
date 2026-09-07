package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"database/sql"
	"notifier/internal/config"
	"notifier/internal/notification"
	"notifier/internal/sender"

	_ "github.com/jackc/pgx/v5/stdlib" // need to init driver, but nothing to call
)

const (
	appVersion = "0.1.0"
	appName    = "Notifier"
)

type Notification = notification.Notification
type Sender = sender.Sender
type LoggingSender = sender.LoggingSender
type ConsoleSender = sender.ConsoleSender
type EmailSender = sender.EmailSender
type TelegramSender = sender.TelegramSender

type Server struct {
	db      *sql.DB
	logger  *slog.Logger
	senders map[string]Sender
}

func (s *Server) findNotification(id int) (Notification, error) {
	const op = "Server.findNotification"

	n, err := notificationById(s.db, id)
	if err != nil {
		return Notification{}, fmt.Errorf("%s: %w", op, err)
	}

	return n, nil
}

func (s *Server) findSender(n Notification) (Sender, error) {
	const op = "Server.findSender"

	sender, ok := s.senders[n.Channel]
	if !ok {
		return ConsoleSender{}, fmt.Errorf("%s: %w", op, notification.ErrInvalidChannel)
	}

	return sender, nil
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
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		bytes := fmt.Sprintf(`{"error": "%s"}`, notification.ErrInvalidId.Error())
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(bytes))
		return
	}

	n, err := s.findNotification(id)
	if err != nil {
		if errors.Is(err, notification.ErrNotFound) {
			bytes := fmt.Sprintf(`{"error": "%s"}`, notification.ErrNotFound.Error())
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(bytes))
			return
		}

		bytes := fmt.Sprintf(`{"error": "%s"}`, notification.ErrInternal.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(bytes))
		return
	}

	js, err := json.Marshal(n)
	if err != nil {
		bytes := fmt.Sprintf(`{"error": "%s"}`, notification.ErrFailedMarshal.Error())
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(bytes))
		return
	}

	// w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(js)
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	const op = "Server.listNotifications"

	notifications, err := getAllNotifications(s.db)
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

	sender, err := s.findSender(n)
	if err != nil {
		if errors.Is(err, notification.ErrInvalidChannel) {
			s.logger.Error("Invalid channel", "channel", n.Channel)
			http.Error(w, "invalid channel", http.StatusBadRequest)
			return
		}

		s.logger.Error("Cant find channel", "channel", n.Channel)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	id, err := insertNotification(s.db, n)
	if err != nil {
		s.logger.Error("Failed insert notification to db", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	n.ID = id

	err = sender.Send(n)
	if err != nil {
		s.logger.Error("Failed to send notification", "error", err)
	}

	js, err := json.Marshal(n)
	if err != nil {
		s.logger.Error(notification.ErrFailedMarshal.Error(), "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
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

func NewServer(db *sql.DB, logger *slog.Logger, senders map[string]Sender) (*Server, error) {
	const op = "NewServer"

	if db == nil {
		return nil, fmt.Errorf("%s: db is required", op)
	}
	if logger == nil {
		return nil, fmt.Errorf("%s: logger is required", op)
	}
	if len(senders) == 0 {
		return nil, fmt.Errorf("%s: senders is required", op)
	}

	return &Server{
		db:      db,
		logger:  logger,
		senders: senders,
	}, nil
}

func (s *Server) exportNotification(w http.ResponseWriter, r *http.Request) {
	const op = "Server.exportNotification"

	notifications, err := getAllNotifications(s.db)
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
		is_urgent BOOLEAN DEFAULT FALSE
	)
	`
	_, err = db.Exec(createTable)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create table failed: %s\n", err)
		os.Exit(1)
	}
	logger.Info("table ready")

	senders := map[string]Sender{
		"console":  LoggingSender{Sender: sender.NewConsoleSender(os.Stdout), Logger: logger},
		"email":    LoggingSender{Sender: sender.NewEmailSender(os.Stdout), Logger: logger},
		"telegram": LoggingSender{Sender: TelegramSender{}, Logger: logger},
	}

	s, err := NewServer(db, logger, senders)
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: %s\n", err)
		os.Exit(1)
	}

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
