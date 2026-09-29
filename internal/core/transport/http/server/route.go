package core_http_server

import (
	"net/http"

	core_http_middleware "notifier/internal/core/transport/http/middleware"
)

type Route struct {
	Handler    http.HandlerFunc
	Method     string
	Path       string
	Middleware []core_http_middleware.Middleware
}

func (r *Route) WithMiddleware() http.Handler {
	return core_http_middleware.Chain(
		r.Handler,
		r.Middleware...,
	)
}
