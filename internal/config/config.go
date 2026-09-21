package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port         string
	GRPCPort     string
	DSN          string
	LogLevel     string
	JWTSecret    string
	AuditLogPath string
	RedisCfg     RedisConfig
}

func Load() (Config, error) {
	cfg := Config{
		Port:         ":8080",
		GRPCPort:     ":9090",
		DSN:          LoadPostgresConfig().DSN(),
		LogLevel:     "info",
		AuditLogPath: "audit.log",
		RedisCfg:     LoadRedisConfig(),
	}

	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = ":" + v
	}
	if v := os.Getenv("GRPC_PORT"); v != "" {
		cfg.GRPCPort = ":" + v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
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

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
