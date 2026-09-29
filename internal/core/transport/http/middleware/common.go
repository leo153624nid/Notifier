package core_http_middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"go.uber.org/zap"

	core_logger "notifier/internal/core/logger"
	core_http_response "notifier/internal/core/transport/http/response"
)

// MARK: - RequestID
const (
	requestIDHeader = "X-Request-ID"
)

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get(requestIDHeader)
			if requestID == "" {
				requestID = newRequestID()
			}

			r.Header.Set(requestIDHeader, requestID)
			w.Header().Set(requestIDHeader, requestID)

			next.ServeHTTP(w, r)
		})
	}
}

// MARK: - Logger
func Logger(l *core_logger.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get(requestIDHeader)

			logger := l.With(
				zap.String("request_id", requestID),
				zap.String("url", r.URL.String()),
			)

			ctx := core_logger.ToContext(r.Context(), logger)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// MARK: - Trace
func Trace() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			logger := core_logger.FromContext(ctx)
			rw := core_http_response.NewResponseWriter(w)

			start := time.Now()
			logger.Info(
				">>>> incoming HTTP request",
				zap.Time("time", start.UTC()),
			)

			next.ServeHTTP(rw, r)

			logger.Info(
				">>>> done HTTP request",
				zap.Int("status_code", rw.GetStatusCode()),
				zap.Duration("latency", time.Since(start)),
			)
		})
	}
}

// MARK: - Panic
func Panic() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			logger := core_logger.FromContext(ctx)
			responseHandler := core_http_response.NewHTTPResponseHandler(logger, w)

			defer func() {
				if p := recover(); p != nil {
					responseHandler.PanicResponse(p, "unexpected panic")
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
