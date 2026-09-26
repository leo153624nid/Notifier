package core_config

import (
	"fmt"
	"os"
)

type Config struct {
	JWTSecret    string
	AuditLogPath string
}

func Load() (Config, error) {
	cfg := Config{
		AuditLogPath: "audit.log",
	}

	if v := os.Getenv("AUDIT_LOG_PATH"); v != "" {
		cfg.AuditLogPath = v
	}

	// Общий секрет с auth-сервисом: им notifier проверяет подпись JWT,
	// который выдаёт auth (см. services/auth/internal/token).
	jwtSecret, ok := os.LookupEnv("JWT_SECRET")
	if !ok || jwtSecret == "" {
		return Config{}, fmt.Errorf("config: JWT_SECRET is required")
	}
	cfg.JWTSecret = jwtSecret

	return cfg, nil
}

func GetEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
