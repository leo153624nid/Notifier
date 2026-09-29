package core_http_middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"

	core_http_response "notifier/internal/core/transport/http/response"
)

// ipRateLimiter выдаёт отдельный token-bucket лимитер на каждый IP,
// чтобы один активный клиент не выжирал лимит для остальных.
type IpRateLimiter struct {
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

func NewIPRateLimiter(r rate.Limit, burst int) *IpRateLimiter {
	l := &IpRateLimiter{
		visitors: make(map[string]*visitor),
		rate:     r,
		burst:    burst,
		stopCh:   make(chan struct{}),
	}

	go l.cleanupLoop()

	return l
}

func (l *IpRateLimiter) getLimiter(ip string) *rate.Limiter {
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

func (l *IpRateLimiter) cleanupLoop() {
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
func (l *IpRateLimiter) Stop() {
	close(l.stopCh)
}

func RateLimiter(limiter *IpRateLimiter) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientLimiter := limiter.getLimiter(clientIP(r))
			if !clientLimiter.Allow() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)

				resp := core_http_response.APIError{
					Error:   "too many requests",
					Message: "too many requests",
				}
				_ = json.NewEncoder(w).Encode(resp)
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
