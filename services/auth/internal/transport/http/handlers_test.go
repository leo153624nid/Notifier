package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"authservice/internal/domain"
	"authservice/internal/repository"
	"authservice/internal/service"
)

type fakeDbPinger struct {
	err error
}

func (f fakeDbPinger) Ping(context.Context) error {
	return f.err
}

func newMockMemoryRepository(email string) *repository.MemoryRepository {
	existUser := domain.User{
		Email: email,
	}
	repo := repository.NewMemoryRepository()
	_, _ = repo.Create(context.Background(), existUser)

	return repo
}

func newTestHandler(
	t *testing.T,
	pingerDB service.Pinger,
) (*Handler, *repository.MemoryRepository) {
	t.Helper()

	existEmail := "exist@mail.com"
	repo := newMockMemoryRepository(existEmail)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	auth, err := service.NewAuthService(repo, logger, "secret", 10*time.Minute)
	if err != nil {
		t.Fatalf("NewAuthService() error: %s", err)
	}
	healthService := service.NewHealthService(pingerDB)

	return NewHandler(auth, healthService, logger, "Auth", "test"), repo
}

func TestHealthHandler(t *testing.T) {
	h, _ := newTestHandler(
		t,
		fakeDbPinger{},
	)

	r := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	h.healthHandler(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.Contains(w.Body.String(), "available") {
		t.Errorf("body = %q, want contains %q", w.Body.String(), "available")
	}
}

func TestHealthHandler_DBUnavailable(t *testing.T) {
	h, _ := newTestHandler(
		t,
		fakeDbPinger{err: errors.New("connection refused")},
	)

	r := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	h.healthHandler(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
}

func TestRegisterUser(t *testing.T) {
	existEmail := "exist@mail.com"
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid request",
			body:       `{"email":"user@example.com","password":"12345678"}`,
			wantStatus: http.StatusCreated,
		},
		{
			name:       "invalid request",
			body:       `{"mail":"user@example.com","word":"12345678"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "exist user",
			body:       fmt.Sprintf(`{"email":"%s","password":"12345678"}`, existEmail),
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "empty email",
			body:       `{"email":"","password":"12345678"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "wrong email",
			body:       `{"email":"wrong @mail.c","password":"12345678"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty password",
			body:       `{"email":"user@example.com","password":""}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "short password",
			body:       `{"email":"user@example.com","password":"1234567"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "long password",
			body:       fmt.Sprintf(`{"email":"user@example.com","password":"%s"}`, make([]byte, 61)),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "password with spaces",
			body:       `{"email":"user@example.com","password":" 1234567 "}`,
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newTestHandler(
				t,
				fakeDbPinger{},
			)

			r := httptest.NewRequest("POST", "/api/v1/auth/register", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			h.registerUser(w, r)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestLoginUser(t *testing.T) {
	existEmail := "exist2@mail.com"
	existBody := fmt.Sprintf(`{"email":"%s","password":"12345678"}`, existEmail)
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{
			name:       "valid request",
			body:       existBody,
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid request",
			body:       fmt.Sprintf(`{"mail":"%s","word":"12345678"}`, existEmail),
			wantStatus: http.StatusUnauthorized, // TODO: need fix, want StatusBadRequest
		},
		{
			name:       "no user",
			body:       `{"email":"user@example.com","password":"12345678"}`,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty email",
			body:       `{"email":"","password":"12345678"}`,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "wrong email",
			body:       `{"email":"wrong @mail.c","password":"12345678"}`,
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "empty password",
			body:       fmt.Sprintf(`{"email":"%s","password":""}`, existEmail),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "short password",
			body:       fmt.Sprintf(`{"email":"%s","password":"1234567"}`, existEmail),
			wantStatus: http.StatusUnauthorized,
		},
		{
			name:       "long password",
			body:       fmt.Sprintf(`{"email":"%s","password":"%s"}`, existEmail, make([]byte, 61)),
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "password with spaces",
			body:       fmt.Sprintf(`{"email":"%s","password":" 1234567 "}`, existEmail),
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _ := newTestHandler(
				t,
				fakeDbPinger{},
			)
			r0 := httptest.NewRequest(
				"POST",
				"/api/v1/auth/register",
				strings.NewReader(fmt.Sprintf(`{"email":"%s","password":"12345678"}`, existEmail)))
			w0 := httptest.NewRecorder()
			h.registerUser(w0, r0)

			r := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			h.loginUser(w, r)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			} else {
				if w.Code == http.StatusOK {
					var res LoginResponse
					_ = json.NewDecoder(w.Body).Decode(&res)
					if len(res.AccessToken) == 0 {
						t.Errorf("token is empty")
					}
				}
			}
		})
	}
}
