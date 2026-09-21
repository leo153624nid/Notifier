package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

// MARK: - RequestID
type ctxKey struct{}

var requestIDKey = ctxKey{}

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

func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := newRequestID()
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// MARK: - RateLimiter
// ipRateLimiter выдаёт отдельный token-bucket лимитер на каждый IP,
// чтобы один активный клиент не выжирал лимит для остальных.
type ipRateLimiter struct {
	visitors map[string]*visitor
	stopCh   chan struct{}
	mu       sync.Mutex
	rate     rate.Limit
	burst    int
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// visitorTTL — как долго храним лимитер неактивного IP перед очисткой.
const visitorTTL = 3 * time.Minute

func newIPRateLimiter(r rate.Limit, burst int) *ipRateLimiter {
	l := &ipRateLimiter{
		visitors: make(map[string]*visitor),
		rate:     r,
		burst:    burst,
		stopCh:   make(chan struct{}),
	}

	go l.cleanupLoop()

	return l
}

func (l *ipRateLimiter) getLimiter(ip string) *rate.Limiter {
	l.mu.Lock()
	defer l.mu.Unlock()

	v, ok := l.visitors[ip]
	if !ok {
		limiter := rate.NewLimiter(l.rate, l.burst)
		l.visitors[ip] = &visitor{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}

	v.lastSeen = time.Now()
	return v.limiter
}

func (l *ipRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.mu.Lock()
			for ip, v := range l.visitors {
				if time.Since(v.lastSeen) > visitorTTL {
					delete(l.visitors, ip)
				}
			}
			l.mu.Unlock()
		case <-l.stopCh:
			return
		}
	}
}

// Stop останавливает фоновую очистку неактивных лимитеров.
func (l *ipRateLimiter) Stop() {
	close(l.stopCh)
}

func rateLimiterMiddleware(limiter *ipRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientLimiter := limiter.getLimiter(clientIP(r))
			if !clientLimiter.Allow() {
				SendJSONError(w, "too many requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return host
}

// MARK: - ContentType
func contentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		next.ServeHTTP(w, r)
	})
}

// MARK: - Auth
type userIDKeyType struct{}

var userIDKey = userIDKeyType{}

// jwtClaims — формат токена, который выдаёт auth-сервис (см.
// services/auth/internal/token.Claims). Notifier только проверяет подпись
// и вычитывает userID, сам токены не выпускает.
type jwtClaims struct {
	UserID uuid.UUID `json:"sub"`
	jwt.RegisteredClaims
}

func getUserID(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}

func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "

	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}

	tok := strings.TrimSpace(strings.TrimPrefix(h, prefix))
	if tok == "" {
		return "", false
	}

	return tok, true
}

func parseUserID(tokenString, secret string) (uuid.UUID, error) {
	var claims jwtClaims

	tok, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		if t.Header["alg"] != jwt.SigningMethodHS256.Alg() {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("parse token: %w", err)
	}
	if !tok.Valid {
		return uuid.UUID{}, fmt.Errorf("parse token: invalid token")
	}

	return claims.UserID, nil
}

func authMiddleware(jwtSecret string, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString, ok := bearerToken(r)
			if !ok {
				logger.Warn("auth failed", "path", r.URL.Path, "method", r.Method, "remote", r.RemoteAddr, "reason", "missing bearer token")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(APIError{Error: "invalid or missing bearer token"})
				return
			}

			userID, err := parseUserID(tokenString, jwtSecret)
			if err != nil {
				logger.Warn("auth failed", "path", r.URL.Path, "method", r.Method, "remote", r.RemoteAddr, "error", err)
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(APIError{Error: "invalid or expired token"})
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MARK: - Logging
func loggingMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			next.ServeHTTP(w, r)

			logger.Info(
				"request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"duration", time.Since(start),
				"request_id", getRequestID(r.Context()),
			)
		})
	}
}
