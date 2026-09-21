package grpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MARK: RequestID
type ctxKey struct{}

var requestIDKey = ctxKey{}

func newRequestID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func requestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// requestIDInterceptor кладёт в контекст новый request id на каждый вызов —
// аналог requestID middleware из internal/transport/http/middlewares.go.
func requestIDInterceptor(
	ctx context.Context,
	req any,
	_ *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	ctx = context.WithValue(ctx, requestIDKey, newRequestID())

	return handler(ctx, req)
}

// MARK: - Logging and Recovery
func loggingRecoveryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		start := time.Now()
		reqID := requestIDFromContext(ctx)

		defer func() {
			if p := recover(); p != nil {
				logger.Error("panic recovered", "request_id", reqID, "panic", p, "stack", debug.Stack())
				err = status.Error(codes.Internal, "internal error")
			}
		}()

		resp, err = handler(ctx, req)

		logger.Info(
			"request completed",
			"method", info.FullMethod,
			"duration", time.Since(start),
			"request_id", reqID,
			"error", err,
		)

		return resp, err
	}
}
