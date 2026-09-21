package http

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"sync"
	"time"

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

// MARK: - Logging and Recover panic
func loggingRecoverMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqID := getRequestID(r.Context())

			defer func() {
				if p := recover(); p != nil {
					logger.Error("panic recovered", "request_id", reqID, "panic", p, "stack", debug.Stack())
					SendJSONError(w, "internal error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)

			logger.Info(
				"request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"duration", time.Since(start),
				"request_id", reqID,
			)
		})
	}
}
