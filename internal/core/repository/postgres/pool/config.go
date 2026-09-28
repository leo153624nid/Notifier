package core_postgres_pool

import (
	"fmt"
	"net/url"
	"time"

	core_config "notifier/internal/core/config"
)

type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
	Timeout  time.Duration
}

func (c Config) DSN() string {
	return c.dsn("postgres")
}

// MigrateDSN возвращает DSN со схемой pgx5, которую ожидает драйвер
// golang-migrate/migrate/v4/database/pgx/v5 (см. cmd/migrate).
func (c Config) MigrateDSN() string {
	return c.dsn("pgx5")
}

func (c Config) dsn(scheme string) string {
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

func LoadConfig() Config {
	dur, _ := time.ParseDuration(core_config.GetEnv("PG_TIMEOUT", "10s"))

	return Config{
		Host:     core_config.GetEnv("PG_HOST", "localhost"),
		Port:     core_config.GetEnv("PG_PORT", "5432"),
		User:     core_config.GetEnv("PG_USER", "notifier"),
		Password: core_config.GetEnv("PG_PASSWORD", "notifier"),
		DBName:   core_config.GetEnv("PG_DBNAME", "notifier"),
		SSLMode:  core_config.GetEnv("PG_SSLMODE", "disable"),
		Timeout:  dur,
	}
}
