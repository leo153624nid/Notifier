package service

import "context"

// Pinger — минимальная зависимость, нужная для проверки доступности БД.
type Pinger interface {
	Ping(ctx context.Context) error
}

type HealthService struct {
	pingerDB Pinger
}

func NewHealthService(pingerDB Pinger) *HealthService {
	return &HealthService{
		pingerDB: pingerDB,
	}
}

func (h *HealthService) CheckDB(ctx context.Context) error {
	return h.pingerDB.Ping(ctx)
}
