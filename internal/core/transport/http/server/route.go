package core_http_server

import "net/http"

type Route struct {
	Handler http.HandlerFunc
	Method  string
	Path    string
}

func NewRoute(
	method string,
	path string,
	handler http.HandlerFunc,
) Route {
	return Route{
		Method:  method,
		Path:    path,
		Handler: handler,
	}
}
