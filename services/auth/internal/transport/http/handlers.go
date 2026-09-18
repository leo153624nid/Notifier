package http

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"authservice/internal/domain"
	"authservice/internal/service"
)

// Handler отвечает за перевод HTTP-запросов в вызовы сервисного слоя и обратно.
type Handler struct {
	auth       *service.AuthService
	health     *service.HealthService
	logger     *slog.Logger
	appName    string
	appVersion string
}

func NewHandler(
	auth *service.AuthService,
	health *service.HealthService,
	logger *slog.Logger,
	appName string,
	appVersion string,
) *Handler {
	return &Handler{
		auth:       auth,
		health:     health,
		logger:     logger,
		appName:    appName,
		appVersion: appVersion,
	}
}

func (h *Handler) healthHandler(w http.ResponseWriter, r *http.Request) {
	const op = "Handler.health"

	ctxDB, cancelDB := context.WithTimeout(r.Context(), 1*time.Second)
	defer cancelDB()

	if err := h.health.CheckDB(ctxDB); err != nil {
		h.logger.Error("db health check failed", "op", op, "error", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "service unavailable", "error": err.Error()})
		return
	}

	resp := HealthResponse{
		App:     h.appName,
		Version: h.appVersion,
		Status:  "available",
	}

	js, err := json.Marshal(resp)
	if err != nil {
		h.logger.Error("marshal failed", "op", op, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(js)
}

func (h *Handler) registerUser(w http.ResponseWriter, r *http.Request) {
	const op = "Handler.registerUser"

	var req UserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendJSONError(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	_, err := h.auth.Register(r.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidCredentials):
			SendJSONError(w, "invalid credentials", http.StatusBadRequest)
		case errors.Is(err, domain.ErrUserExists):
			SendJSONError(w, "user already exists", http.StatusForbidden)
		default:
			h.logger.Error("register user failed", "op", op, "error", err)
			SendJSONError(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) loginUser(w http.ResponseWriter, r *http.Request) {
	const op = "Handler.loginUser"

	var req UserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		SendJSONError(w, "invalid request payload", http.StatusBadRequest)
		return
	}

	tok, err := h.auth.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrInvalidCredentials):
			SendJSONError(w, "invalid credentials", http.StatusUnauthorized)
		default:
			h.logger.Error("login user failed", "op", op, "error", err)
			SendJSONError(w, "internal error", http.StatusInternalServerError)
		}
		return
	}

	js, err := json.Marshal(toLoginResponse(tok))
	if err != nil {
		h.logger.Error("marshal failed", "op", op, "error", err)
		SendJSONError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(js)
}
