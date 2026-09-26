package transport_grpc

import "os"

type Config struct {
	GRPCPort string
}

func LoadConfig() Config {
	cfg := Config{
		GRPCPort: ":9090",
	}

	if v := os.Getenv("GRPC_PORT"); v != "" {
		cfg.GRPCPort = ":" + v
	}

	return cfg
}
