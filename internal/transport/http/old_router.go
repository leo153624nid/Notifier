package transport_http

import (
	"log/slog"
	"net/http"

	"golang.org/x/time/rate"
)

// Router — собранный http.Handler со всеми маршрутами и middleware,
// плюс доступ к фоновым ресурсам (rate limiter), которые нужно останавливать при shutdown.
type OldRouter struct { // TODO: take limiter and delete ?
	http.Handler
	limiter *ipRateLimiter
}

func NewOldRouter(h *NotificationsHTTPHandler, jwtSecret string, logger *slog.Logger) *OldRouter {
	auth := authMiddleware(jwtSecret, logger)

	ipLimiter := newIPRateLimiter(rate.Limit(10), 20)
	rateLimit := rateLimiterMiddleware(ipLimiter)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", h.healthHandler)
	mux.Handle("GET /api/v1/notifications", rateLimit(auth(http.HandlerFunc(h.listNotifications))))
	mux.Handle("GET /api/v1/notifications/export", rateLimit(auth(http.HandlerFunc(h.exportNotifications))))
	mux.Handle("GET /api/v1/notifications/{id}", rateLimit(auth(http.HandlerFunc(h.getNotification))))
	mux.Handle("DELETE /api/v1/notifications/{id}", rateLimit(auth(http.HandlerFunc(h.deleteNotification))))
	mux.Handle("POST /api/v1/notifications", rateLimit(auth(http.HandlerFunc(h.createNotification))))

	// handler := requestID(loggingRecoverMiddleware(logger)(contentType(mux)))
	handler := mux

	return &OldRouter{
		Handler: handler,
		limiter: ipLimiter,
	}
}

// Stop останавливает фоновую очистку rate limiter'а — вызывать при graceful shutdown.
func (rt *OldRouter) Stop() {
	rt.limiter.Stop()
}
