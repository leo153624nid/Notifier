package core_http_server

import (
	"fmt"
	"net/http"

	core_http_middleware "notifier/internal/core/transport/http/middleware"
)

type ApiVersion string

var (
	ApiVersion1 = ApiVersion("v1")
	ApiVersion2 = ApiVersion("v2")
	ApiVersion3 = ApiVersion("v3")
)

type ApiVersionRouter struct {
	*http.ServeMux
	limiter    *core_http_middleware.IpRateLimiter
	apiVersion ApiVersion
}

func NewApiVersionRouter(
	apiVersion ApiVersion,
	limiter *core_http_middleware.IpRateLimiter,
) *ApiVersionRouter {
	return &ApiVersionRouter{
		ServeMux:   http.NewServeMux(),
		apiVersion: apiVersion,
		limiter:    limiter,
	}
}

func (r *ApiVersionRouter) RegisterRoutes(routes ...Route) {
	for _, route := range routes {
		pattern := fmt.Sprintf("%s %s", route.Method, route.Path)
		r.Handle(pattern, route.Handler)
	}
}

// Stop останавливает фоновую очистку rate limiter'а — вызывать при graceful shutdown.
func (r *ApiVersionRouter) Stop() {
	r.limiter.Stop()
}
