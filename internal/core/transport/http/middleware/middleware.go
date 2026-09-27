package core_http_middleware

import "net/http"

type Middleware func(http.Handler) http.Handler

func Chain(
	h http.Handler,
	middlewares ...Middleware,
) http.Handler {
	count := len(middlewares)
	if count == 0 {
		return h
	}

	for i := count - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}

	return h
}
