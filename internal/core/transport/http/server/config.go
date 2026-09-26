package core_http_server

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Port            string        `envconfig:"PORT" required:"true"`
	ShutdownTimeout time.Duration `envconfig:"SHUTDOWN_TIMEOUT" required:"true"`
}

func NewConfig() (Config, error) {
	const op = "NewConfig"

	var cfg Config

	if err := envconfig.Process("HTTP", &cfg); err != nil {
		return Config{}, fmt.Errorf("%s: process: %w", op, err)
	}

	cfg.ShutdownTimeout = 30

	return cfg, nil
}

func NewConfigMust() Config {
	cfg, err := NewConfig()
	if err != nil {
		panic(err)
	}

	return cfg
}
