package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"notifier/internal/notification"
)

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
		SendJSONError(w, notification.ErrInvalidId.Error(), http.StatusBadRequest)
		return
	}

	n, err := s.store.GetById(id)
	if err != nil {
		if errors.Is(err, notification.ErrNotFound) {
			SendJSONError(w, notification.ErrNotFound.Error(), http.StatusNotFound)
			return
		}

		s.logger.Error("get from store failed", "op", op, "error", err)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	js, err := json.Marshal(n)
	if err != nil {
		s.logger.Error("marshal failed", "op", op, "error", err)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
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
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	if notifications == nil {
		notifications = []notification.Notification{}
	}

	js, err := json.Marshal(notifications)
	if err != nil {
		s.logger.Error("marshal failed", "op", op, "error", err)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
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
		SendJSONError(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	err = n.Validate()
	if err != nil {
		SendJSONError(w, "invalid request", http.StatusBadRequest)
		return
	}

	sender, ok := s.senders[n.Channel]
	if !ok {
		SendJSONError(w, "invalid request: unsupported channel", http.StatusBadRequest)
		return
	}

	id, err := s.store.Save(n)
	if err != nil {
		s.logger.Error("save failed", "error", err)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	n.ID = id
	n.Status = "pending"

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

	js, err := json.Marshal(n)
	if err != nil {
		s.logger.Error("marshal failed", "op", op, "error", err)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
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
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	err = s.auditLogger.Write(notifications)
	if err != nil {
		s.logger.Error("export failed", "op", op, "error", err)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]int{"exported": len(notifications)})
}
