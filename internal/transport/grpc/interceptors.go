package transport_grpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	core_logger "notifier/internal/core/logger"
	"runtime/debug"
	"time"

	"go.uber.org/zap"
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

// requestIDInterceptor кладёт в контекст новый request id на каждый вызов
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
func loggingRecoveryInterceptor(l *core_logger.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		start := time.Now()
		reqID := requestIDFromContext(ctx)

		logger := l.With(
			zap.String("request_id", reqID),
		)
		ctx = core_logger.ToContext(ctx, logger)

		defer func() {
			if p := recover(); p != nil {
				logger.Error(
					"panic recovered",
					zap.Any("panic", p),
					zap.String("stack", string(debug.Stack())),
				)
				err = status.Error(codes.Internal, "internal error")
			}
		}()

		resp, err = handler(ctx, req)

		logger.Info(
			"request completed",
			zap.String("method", info.FullMethod),
			zap.Duration("duration", time.Since(start)),
			zap.Error(err),
		)

		return resp, err
	}
}
