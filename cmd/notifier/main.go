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

	"notifier/internal/notification"
	"notifier/internal/sender"
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
	notifications map[int]Notification
	nextID        int
	logger        *slog.Logger
	senders       map[string]Sender
}

// var notifications []Notification // slice
// var notifications map[int]Notification{} // empty map

func (s *Server) findNotification(id int) (Notification, error) {
	const op = "Server.findNotification"

	n, ok := s.notifications[id]
	if !ok {
		return notification.Notification{}, fmt.Errorf("%s: %w", op, notification.ErrNotFound)
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
	var all []Notification
	for _, n := range s.notifications {
		all = append(all, n)
	}

	js, err := json.Marshal(all)
	if err != nil {
		s.logger.Error("Marshal error", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(js)
}

func (s *Server) createNotification(w http.ResponseWriter, r *http.Request) {
	const op = "Server.createNotification"

	var n notification.Notification
	err := json.NewDecoder(r.Body).Decode(&n)
	if err != nil {
		// w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "Invalid request payload"}`))
		return
	}

	err = n.Validate()
	if err != nil {
		http.Error(w, fmt.Errorf("%s: %w", op, err).Error(), http.StatusBadRequest)
		return
	}

	sender, err := s.findSender(n)
	if err != nil {
		if errors.Is(err, notification.ErrInvalidChannel) {
			s.logger.Error("Invalid channel", "channel", n.Channel)
			http.Error(w, fmt.Errorf("%s: %w", op, err).Error(), http.StatusBadRequest)
			return
		}

		s.logger.Error("Cant find channel", "channel", n.Channel)
		http.Error(w, fmt.Errorf("%s: %w", op, err).Error(), http.StatusInternalServerError)
		return
	}

	s.nextID++
	n.ID = s.nextID
	s.notifications[n.ID] = n

	err = sender.Send(n)
	if err != nil {
		s.logger.Error("Failed to send notification", "error", err)
	}

	js, err := json.Marshal(n)
	if err != nil {
		s.logger.Error(notification.ErrFailedMarshal.Error(), "error", err)
		http.Error(
			w,
			fmt.Errorf("%s: %w", op, notification.ErrFailedMarshal).Error(),
			http.StatusInternalServerError,
		)
		return
	}

	// w.Header().Set("Content-Type", "application/json")
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

func NewServer(logger *slog.Logger, senders map[string]Sender) (*Server, error) {
	const op = "NewServer"

	if logger == nil {
		return nil, fmt.Errorf("%s: logger is required", op)
	}
	if len(senders) == 0 {
		return nil, fmt.Errorf("%s: senders is required", op)
	}

	return &Server{
		notifications: map[int]Notification{},
		nextID:        0,
		logger:        logger,
		senders:       senders,
	}, nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	senders := map[string]Sender{
		"console":  LoggingSender{Sender: sender.NewConsoleSender(os.Stdout), Logger: logger},
		"email":    LoggingSender{Sender: sender.NewEmailSender(os.Stdout), Logger: logger},
		"telegram": LoggingSender{Sender: TelegramSender{}, Logger: logger},
	}

	s, err := NewServer(logger, senders)
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: %s\n", err)
		os.Exit(1)
	}

	s.nextID++
	s.notifications[s.nextID] = Notification{
		ID:        s.nextID,
		Recipient: "111",
		Subject:   "Test Notification",
		Body:      "This is a test notification.",
		Channel:   "email",
		IsUrgent:  false,
	}
	s.nextID++
	s.notifications[s.nextID] = Notification{
		ID:        s.nextID,
		Recipient: "222",
		Body:      "This is another test notification.",
		Channel:   "console",
		IsUrgent:  true,
	}

	s.logger.Info("Starting server", "app", appName, "version", appVersion, "port", 8080)

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("GET /api/notifications", s.listNotifications)
	mux.HandleFunc("GET /api/notifications/{id}", s.getNotification)
	mux.HandleFunc("POST /api/notifications", s.createNotification)

	err = http.ListenAndServe(":8080", s.logRequest(contentType(mux)))
	if err != nil {
		s.logger.Error("Error starting server", "error", err)
	}
}
