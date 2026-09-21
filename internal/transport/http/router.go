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

func NewRouter(h *Handler, jwtSecret string, logger *slog.Logger) *Router {
	auth := authMiddleware(jwtSecret, logger)

	ipLimiter := newIPRateLimiter(rate.Limit(10), 20)
	rateLimit := rateLimiterMiddleware(ipLimiter)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.healthHandler)
	mux.Handle("GET /api/v1/notifications", rateLimit(auth(http.HandlerFunc(h.listNotifications))))
	mux.Handle("GET /api/v1/notifications/export", rateLimit(auth(http.HandlerFunc(h.exportNotification))))
	mux.Handle("GET /api/v1/notifications/{id}", rateLimit(auth(http.HandlerFunc(h.getNotification))))
	mux.Handle("POST /api/v1/notifications", rateLimit(auth(http.HandlerFunc(h.createNotification))))

	handler := requestID(loggingMiddleware(logger)(contentType(mux)))

	return &Router{
		Handler: handler,
		limiter: ipLimiter,
	}
}

// Stop останавливает фоновую очистку rate limiter'а — вызывать при graceful shutdown.
func (rt *Router) Stop() {
	rt.limiter.Stop()
}
