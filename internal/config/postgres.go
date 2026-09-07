package config

import (
	"fmt"
	"os"
)

type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

func (c PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%s/%s?sslmode=%s",
		c.User, c.Password, c.Host, c.Port, c.DBName, c.SSLMode,
	)
}

func LoadPostgresConfig() PostgresConfig {
	return PostgresConfig{
		Host:     getEnv("PG_HOST", "localhost"),
		Port:     getEnv("PG_PORT", "5432"),
		User:     getEnv("PG_USER", "notifier"),
		Password: getEnv("PG_PASSWORD", "notifier"),
		DBName:   getEnv("PG_DBNAME", "notifier"),
		SSLMode:  getEnv("PG_SSLMODE", "disable"),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
