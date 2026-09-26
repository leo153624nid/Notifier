package core_http_server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	core_logger "notifier/internal/core/logger"
)

const (
	serverReadHeaderTimeout = 5 * time.Second
	serverReadTimeout       = 10 * time.Second
	serverWriteTimeout      = 15 * time.Second
	serverIdleTimeout       = 60 * time.Second
)

type HTTPServer struct {
	mux    *http.ServeMux
	logger *core_logger.Logger
	config Config
}

func NewHTTPServer(
	cfg Config,
	logger *core_logger.Logger,
) *HTTPServer {
	return &HTTPServer{
		mux:    http.NewServeMux(),
		config: cfg,
		logger: logger,
	}
}

func (srv *HTTPServer) RegisterApiRoutes(routers ...*ApiVersionRouter) {
	for _, router := range routers {
		prefix := "/api/" + string(router.apiVersion)

		srv.mux.Handle(
			prefix+"/",
			http.StripPrefix(prefix, router),
		)
	}
}

func (srv *HTTPServer) Run(ctx context.Context) error {
	server := http.Server{
		Addr:              ":" + srv.config.Port,
		Handler:           srv.mux,
		ReadHeaderTimeout: serverReadHeaderTimeout,
		ReadTimeout:       serverReadTimeout,
		WriteTimeout:      serverWriteTimeout,
		IdleTimeout:       serverIdleTimeout,
	}

	ch := make(chan error, 1)

	go func() {
		defer close(ch)

		srv.logger.Warn(
			"start HTTP server",
			zap.String("port", srv.config.Port),
		)

		err := server.ListenAndServe()
		if !errors.Is(err, http.ErrServerClosed) {
			ch <- err
		}
	}()

	select {
	case err := <-ch:
		if err != nil {
			return fmt.Errorf("listen and serve HTTP: %w", err)
		}
	case <-ctx.Done():
		srv.logger.Warn("shutdown HTTP server...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), srv.config.ShutdownTimeout)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			srv.logger.Error(
				"failed to shutdownd",
				zap.Error(err),
			)

			_ = server.Close()

			return fmt.Errorf("shutdown HTTP server: %w", err)
		}

		srv.logger.Warn("HTTP server stopped")
	}

	return nil
}
