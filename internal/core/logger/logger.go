package core_logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	*zap.Logger

	file *os.File
}

func NewLogger(cfg LoggerConfig) (*Logger, error) {
	const op = "Logger.NewLogger"

	zapLvl := zap.NewAtomicLevel()
	if err := zapLvl.UnmarshalText([]byte(cfg.Level)); err != nil {
		return nil, fmt.Errorf("%s: UnmarshalText: %w", op, err)
	}

	if err := os.MkdirAll(cfg.Folder, 0755); err != nil {
		return nil, fmt.Errorf("%s: MkdirAll: %w", op, err)
	}

	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05.000000")
	logFilePath := filepath.Join(
		cfg.Folder,
		fmt.Sprintf("%s.log", timestamp),
	)

	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("%s: OpenFile: %w", op, err)
	}

	zapConfig := zap.NewDevelopmentEncoderConfig()
	zapConfig.EncodeTime = zapcore.TimeEncoderOfLayout("2006-01-02T15:04:05.000000")

	zapEncoder := zapcore.NewConsoleEncoder(zapConfig)
	core := zapcore.NewTee(
		zapcore.NewCore(zapEncoder, zapcore.AddSync(os.Stdout), zapLvl),
		zapcore.NewCore(zapEncoder, zapcore.AddSync(logFile), zapLvl),
	)

	zapLogger := zap.New(core, zap.AddCaller())

	return &Logger{
		Logger: zapLogger,
		file:   logFile,
	}, nil
}

// NewNop возвращает логгер, который ничего не пишет и не трогает файловую
// систему. Нужен тестам, чтобы не зависеть от env и лог-файлов.
func NewNop() *Logger {
	return &Logger{Logger: zap.NewNop()}
}

func (l *Logger) Close() {
	const op = "Logger.Close"

	if l.file == nil {
		return
	}

	if err := l.file.Close(); err != nil {
		fmt.Println("failed to close log file", "op", op, "err", err)
	}
}

func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{
		Logger: l.Logger.With(fields...),
		file:   l.file,
	}
}
