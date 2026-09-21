package http

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

func TestRateLimiterMiddleware_AllowsWithinBurst(t *testing.T) {
	limiter := newIPRateLimiter(rate.Limit(1), 2)
	defer limiter.Stop()

	handler := rateLimiterMiddleware(limiter)(okHandler())

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r := newRequestFromIP("1.2.3.4:1111")

		handler.ServeHTTP(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("request %d: status = %d, want %d", i+1, w.Code, http.StatusOK)
		}
	}
}

func TestRateLimiterMiddleware_RejectsOverBurst(t *testing.T) {
	limiter := newIPRateLimiter(rate.Limit(1), 2)
	defer limiter.Stop()

	handler := rateLimiterMiddleware(limiter)(okHandler())

	// исчерпываем весь burst для одного IP
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r := newRequestFromIP("1.2.3.4:1111")
		handler.ServeHTTP(w, r)
	}

	w := httptest.NewRecorder()
	r := newRequestFromIP("1.2.3.4:1111")
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	var apiErr APIError
	if err := json.NewDecoder(w.Body).Decode(&apiErr); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if apiErr.Error != "too many requests" {
		t.Errorf("error message = %q, want %q", apiErr.Error, "too many requests")
	}
}

func TestRateLimiterMiddleware_RefillsOverTime(t *testing.T) {
	limiter := newIPRateLimiter(rate.Limit(100), 1) // ~10ms на восстановление токена
	defer limiter.Stop()

	handler := rateLimiterMiddleware(limiter)(okHandler())
	ip := "1.2.3.4:1111"

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newRequestFromIP(ip))
	if w.Code != http.StatusOK {
		t.Fatalf("first request: status = %d, want %d", w.Code, http.StatusOK)
	}

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, newRequestFromIP(ip))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("second request (no refill yet): status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	time.Sleep(50 * time.Millisecond)

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, newRequestFromIP(ip))
	if w.Code != http.StatusOK {
		t.Fatalf("third request (after refill): status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRateLimiterMiddleware_IsolatesByIP(t *testing.T) {
	limiter := newIPRateLimiter(rate.Limit(1), 1)
	defer limiter.Stop()

	handler := rateLimiterMiddleware(limiter)(okHandler())

	// первый IP исчерпывает свой лимит
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, newRequestFromIP("1.1.1.1:1111"))
	if w.Code != http.StatusOK {
		t.Fatalf("ip1 first request: status = %d, want %d", w.Code, http.StatusOK)
	}

	w = httptest.NewRecorder()
	handler.ServeHTTP(w, newRequestFromIP("1.1.1.1:1111"))
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("ip1 second request: status = %d, want %d", w.Code, http.StatusTooManyRequests)
	}

	// второй IP не должен пострадать от лимита первого
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, newRequestFromIP("2.2.2.2:2222"))
	if w.Code != http.StatusOK {
		t.Fatalf("ip2 first request: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		want       string
	}{
		{"host with port", "1.2.3.4:5678", "1.2.3.4"},
		{"no port fallback", "1.2.3.4", "1.2.3.4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tt.remoteAddr

			if got := clientIP(r); got != tt.want {
				t.Errorf("clientIP(%q) = %q, want %q", tt.remoteAddr, got, tt.want)
			}
		})
	}
}

func newTestToken(t *testing.T, userID uuid.UUID, secret string, ttl time.Duration) string {
	t.Helper()

	return newTestTokenWithType(t, userID, secret, ttl, accessTokenType)
}

func newTestTokenWithType(t *testing.T, userID uuid.UUID, secret string, ttl time.Duration, typ string) string {
	t.Helper()

	claims := jwtClaims{
		UserID: userID,
		Type:   typ,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}

	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("sign token: %s", err)
	}

	return tok
}

func TestAuthMiddleware_AllowsValidToken(t *testing.T) {
	secret := "test-secret"
	userID := uuid.New()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	var gotUserID uuid.UUID
	var gotOK bool

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUserID, gotOK = getUserID(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	handler := authMiddleware(secret, logger)(next)

	tok := newTestToken(t, userID, secret, time.Minute)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+tok)

	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !gotOK {
		t.Fatal("userID was not set in context")
	}
	if gotUserID != userID {
		t.Errorf("userID = %s, want %s", gotUserID, userID)
	}
}

func TestAuthMiddleware_Rejects(t *testing.T) {
	secret := "test-secret"
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	handler := authMiddleware(secret, logger)(okHandler())

	tests := []struct {
		name   string
		header string
	}{
		{"no header", ""},
		{"no bearer prefix", newTestToken(t, uuid.New(), secret, time.Minute)},
		{"empty bearer token", "Bearer "},
		{"wrong secret", "Bearer " + newTestToken(t, uuid.New(), "wrong-secret", time.Minute)},
		{"expired token", "Bearer " + newTestToken(t, uuid.New(), secret, -time.Minute)},
		{"garbage token", "Bearer not.a.jwt"},
		{"refresh token used as access", "Bearer " + newTestTokenWithType(t, uuid.New(), secret, time.Minute, "refresh")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				r.Header.Set("Authorization", tt.header)
			}

			handler.ServeHTTP(w, r)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
			}

			var apiErr APIError
			if err := json.NewDecoder(w.Body).Decode(&apiErr); err != nil {
				t.Fatalf("decode body: %v", err)
			}
			if apiErr.Error == "" {
				t.Error("error message is empty")
			}
		})
	}
}

func newRequestFromIP(remoteAddr string) *http.Request {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = remoteAddr
	return r
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}
