package cache

import (
	"fmt"

	core_config "notifier/internal/core/config"
)

type Config struct {
	Addr     string
	Password string
}

func LoadConfig() Config {
	host := core_config.GetEnv("RD_HOST", "localhost")
	port := core_config.GetEnv("RD_PORT", "6379")

	return Config{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: core_config.GetEnv("RD_PASSWORD", "notifier"),
	}
}
