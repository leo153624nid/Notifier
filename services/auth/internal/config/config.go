package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	Port      string
	DSN       string
	LogLevel  string
	JWTSecret string
	JWTTTL    time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		Port:     ":8081",
		DSN:      LoadPostgresConfig().DSN(),
		LogLevel: "info",
		JWTTTL:   15 * time.Minute,
	}

	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = ":" + v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}
	if v := os.Getenv("JWT_TTL"); v != "" {
		ttl, err := time.ParseDuration(v)
		if err != nil {
			return Config{}, fmt.Errorf("config: invalid JWT_TTL: %w", err)
		}
		cfg.JWTTTL = ttl
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
