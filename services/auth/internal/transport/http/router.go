package http

import (
	"log/slog"
	"net/http"

	"golang.org/x/time/rate"
)

// Router — собранный http.Handler со всеми маршрутами и middleware,
// плюс доступ к фоновым ресурсам (rate limiter), которые нужно останавливать при shutdown.
type Router struct {
	http.Handler
	limiter *ipRateLimiter
}

func NewRouter(h *Handler, logger *slog.Logger) *Router {
	ipLimiter := newIPRateLimiter(rate.Limit(10), 20)
	rateLimit := rateLimiterMiddleware(ipLimiter)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.healthHandler)
	mux.Handle("POST /api/v1/auth/register", rateLimit(http.HandlerFunc(h.registerUser)))
	mux.Handle("POST /api/v1/auth/login", rateLimit(http.HandlerFunc(h.loginUser)))
	mux.Handle("POST /api/v1/auth/refresh", rateLimit(http.HandlerFunc(h.refreshToken)))
	mux.Handle("POST /api/v1/auth/logout", rateLimit(http.HandlerFunc(h.logoutUser)))

	handler := requestID(loggingRecoverMiddleware(logger)(contentType(mux)))

	return &Router{
		Handler: handler,
		limiter: ipLimiter,
	}
}

// Stop останавливает фоновую очистку rate limiter'а — вызывать при graceful shutdown.
func (rt *Router) Stop() {
	rt.limiter.Stop()
}
