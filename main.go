package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	appVersion = "0.1.0"
	appName    = "Notifier"
)

var ErrNotFound = errors.New("notification not found")
var ErrInvalidId = errors.New("invalid notification id")
var ErrInvalidChannel = errors.New("invalid channel")
var ErrFailedMarshal = errors.New("failed marshal")
var ErrInternal = errors.New("internal server error")

type Server struct {
	notifications map[int]Notification
	nextID        int
	logger        *slog.Logger
	senders       map[string]Sender
}

// var notifications []Notification // slice
// var notifications map[int]Notification{} // empty map

type Notification struct {
	ID        int    `json:"id"`
	Recipient string `json:"to"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Channel   string `json:"channel"`
	IsUrgent  bool   `json:"urgent"`
}

type Sender interface {
	Send(Notification) error
}

type LoggingSender struct {
	Sender
	logger *slog.Logger
}

type ConsoleSender struct {
	w io.Writer
}
type EmailSender struct {
	w io.Writer
}
type TelegramSender struct{}

func (ls LoggingSender) Send(n Notification) error {
	ls.logger.Info("sending", "to", n.Recipient, "channel", n.Channel)
	err := ls.Sender.Send(n)
	if err != nil {
		ls.logger.Error("Failed to send notification", "error", err)
		return err
	}
	ls.logger.Info("sent", "to", n.Recipient, "channel", n.Channel)
	return nil
}

func NewConsoleSender(w io.Writer) *ConsoleSender {
	if w == nil {
		w = os.Stdout
	}
	return &ConsoleSender{w}
}

func NewEmailSender(w io.Writer) *EmailSender {
	if w == nil {
		w = os.Stdout
	}
	return &EmailSender{w}
}

func (cs ConsoleSender) Send(n Notification) error {
	_, err := fmt.Fprintf(cs.w, "[console] to %s | %s\n", n.Recipient, n.Subject)
	return err
}

func (es EmailSender) Send(n Notification) error {
	_, err := fmt.Fprintf(es.w, "[email] to %s | %s\n", n.Recipient, n.Subject)
	return err
}

func (tg TelegramSender) Send(n Notification) error {
	fmt.Printf("[telegram] to %s | %s\n", n.Recipient, n.Subject)
	return nil
}

func (n Notification) String() string {
	return fmt.Sprintf("Notification{to:%s, subject:%s, channel:%s}", n.Recipient, n.Subject, n.Channel)
}

func (n Notification) Format() string {
	return fmt.Sprintf(
		"id: %d | to: %s | subject: %s | body: %s | channel: %s",
		n.ID,
		n.Recipient,
		n.Subject,
		n.Body,
		n.Channel,
	)
}

func (n Notification) Send() (message string, status string) {
	message = n.Format()
	status = "queued"
	return message, status
}

func (n Notification) Validate() error {
	const op = "Notification.Validate"

	if n.Recipient == "" {
		return fmt.Errorf("%s: recipient is required", op)
	}
	if n.Channel == "" {
		return fmt.Errorf("%s: channel is required", op)
	}
	return nil
}

func (s *Server) findNotification(id int) (Notification, error) {
	const op = "Server.findNotification"

	n, ok := s.notifications[id]
	if !ok {
		return Notification{}, fmt.Errorf("%s: %w", op, ErrNotFound)
	}

	return n, nil
}

func (s *Server) findSender(n Notification) (Sender, error) {
	const op = "Server.findSender"

	sender, ok := s.senders[n.Channel]
	if !ok {
		return ConsoleSender{}, fmt.Errorf("%s: %w", op, ErrInvalidChannel)
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
	const op = "Server.getNotification"

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, fmt.Errorf("%s: %w", op, ErrInvalidId).Error(), http.StatusBadRequest)
		return
	}

	n, err := s.findNotification(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, fmt.Errorf("%s: %w", op, err).Error(), http.StatusNotFound)
			return
		}

		http.Error(w, fmt.Errorf("%s: %w", op, ErrInternal).Error(), http.StatusInternalServerError)
		return
	}

	js, err := json.Marshal(n)
	if err != nil {
		http.Error(w, fmt.Errorf("%s: %w", op, ErrFailedMarshal).Error(), http.StatusInternalServerError)
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

	var n Notification
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
		if errors.Is(err, ErrInvalidChannel) {
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
		s.logger.Error(ErrFailedMarshal.Error(), "error", err)
		http.Error(w, fmt.Errorf("%s: %w", op, ErrFailedMarshal).Error(), http.StatusInternalServerError)
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
		"console":  LoggingSender{NewConsoleSender(os.Stdout), logger},
		"email":    LoggingSender{NewEmailSender(os.Stdout), logger},
		"telegram": LoggingSender{TelegramSender{}, logger},
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
