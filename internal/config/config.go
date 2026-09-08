package config

import (
	"log"
	"os"
)

type Config struct {
	Port     string
	DSN      string
	LogLevel string
	APIkey   string
}

func Load() Config {
	cfg := Config{
		Port:     ":8080",
		DSN:      LoadPostgresConfig().DSN(),
		LogLevel: "info",
	}

	if v := os.Getenv("PORT"); v != "" {
		cfg.Port = ":" + v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}

	apiKey, ok := os.LookupEnv("API_KEY")
	if !ok || apiKey == "" {
		log.Fatal("API_KEY is required")
	}
	cfg.APIkey = apiKey

	return cfg
}
