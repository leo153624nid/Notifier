package config

import (
	"fmt"
	"net/url"
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
	return c.dsn("postgres")
}

// MigrateDSN возвращает DSN со схемой pgx5, которую ожидает драйвер
// golang-migrate/migrate/v4/database/pgx/v5 (см. cmd/migrate).
func (c PostgresConfig) MigrateDSN() string {
	return c.dsn("pgx5")
}

func (c PostgresConfig) dsn(scheme string) string {
	u := url.URL{
		Scheme: scheme,
		User:   url.UserPassword(c.User, c.Password),
		Host:   fmt.Sprintf("%s:%s", c.Host, c.Port),
		Path:   "/" + c.DBName,
	}

	q := u.Query()
	q.Set("sslmode", c.SSLMode)
	u.RawQuery = q.Encode()

	return u.String()
}

func LoadPostgresConfig() PostgresConfig {
	return PostgresConfig{
		Host:     getEnv("PG_HOST", "localhost"),
		Port:     getEnv("PG_PORT", "5432"),
		User:     getEnv("PG_USER", "auth"),
		Password: getEnv("PG_PASSWORD", "auth"),
		DBName:   getEnv("PG_DBNAME", "auth"),
		SSLMode:  getEnv("PG_SSLMODE", "disable"),
	}
}
