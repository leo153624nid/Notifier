package core_logger

import "context"

type loggerKey struct{}

var (
	LoggerKey = loggerKey{}
)

func FromContext(ctx context.Context) *Logger {
	logger, ok := ctx.Value(LoggerKey).(*Logger)
	if !ok {
		panic("no logger in ctx")
	}

	return logger
}

func ToContext(
	parent context.Context,
	logger *Logger,
) context.Context {
	return context.WithValue(parent, LoggerKey, logger)
}
