package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	Port             string
	DSN              string
	LogLevel         string
	JWTSecret        string
	JWTAccessTTL     time.Duration
	JWTRefreshTTL    time.Duration
	NotifierGRPCAddr string
}

func Load() (Config, error) {
	cfg := Config{
		Port:             ":8081",
		DSN:              LoadPostgresConfig().DSN(),
		LogLevel:         "info",
		JWTAccessTTL:     15 * time.Minute,
		JWTRefreshTTL:    30 * 24 * time.Hour,
		NotifierGRPCAddr: "localhost:9090",
	}

	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = ":" + v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("JWT_ACCESS_TTL"); v != "" {
		ttl, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("config: invalid JWT_ACCESS_TTL: %w", err)
		}
		cfg.JWTAccessTTL = ttl
	}
	if v := os.Getenv("JWT_REFRESH_TTL"); v != "" {
		ttl, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("config: invalid JWT_REFRESH_TTL: %w", err)
		}
		cfg.JWTRefreshTTL = ttl
	}
	if v := os.Getenv("NOTIFIER_GRPC_ADDR"); v != "" {
		cfg.NotifierGRPCAddr = v
	}

	secret, ok := os.LookupEnv("JWT_SECRET")
	if !ok || secret == "" {
		return Config{}, fmt.Errorf("config: JWT_SECRET is required")
	}
	cfg.JWTSecret = secret

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
