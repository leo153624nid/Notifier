package config

import (
	"os"
	"time"
)

type Config struct {
	Port      string
	DSN       string
	LogLevel  string
}

func Load() (Config, error) {
	cfg := Config{
		Port:      ":8081",
		DSN:       LoadPostgresConfig().DSN(),
		LogLevel:  "info",
	}

	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = ":" + v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
