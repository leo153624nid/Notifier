package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port         string
	DSN          string
	LogLevel     string
	APIkey       string
	AuditLogPath string
}

func Load() (Config, error) {
	cfg := Config{
		Port:         ":8080",
		DSN:          LoadPostgresConfig().DSN(),
		LogLevel:     "info",
		AuditLogPath: "audit.log",
	}

	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = ":" + v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("AUDIT_LOG_PATH"); v != "" {
		cfg.AuditLogPath = v
	}

	apiKey, ok := os.LookupEnv("API_KEY")
	if !ok || apiKey == "" {
		return Config{}, fmt.Errorf("config: API_KEY is required")
	}
	cfg.APIkey = apiKey

	return cfg, nil
}
