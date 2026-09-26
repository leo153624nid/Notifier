package core_logger

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

type LoggerConfig struct {
	Level  string `envconfig:"LEVEL" required:"true"`
	Folder string `envconfig:"FOLDER" required:"true"`
}

func NewConfig() (LoggerConfig, error) {
	const op = "NewConfig"

	var cfg LoggerConfig

	if err := envconfig.Process("LOG", &cfg); err != nil {
		return LoggerConfig{}, fmt.Errorf("%s: process: %w", op, err)
	}

	return cfg, nil
}

func NewConfigMust() LoggerConfig {
	cfg, err := NewConfig()
	if err != nil {
		panic(err)
	}

	return cfg
}
