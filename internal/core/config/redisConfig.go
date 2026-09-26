package config

import (
	"fmt"
)

type RedisConfig struct {
	Addr     string
	Password string
}

func LoadRedisConfig() RedisConfig {
	host := getEnv("RD_HOST", "localhost")
	port := getEnv("RD_PORT", "6379")
	return RedisConfig{
		Addr:     fmt.Sprintf("%s:%s", host, port),
		Password: getEnv("RD_PASSWORD", "notifier"),
	}
}
