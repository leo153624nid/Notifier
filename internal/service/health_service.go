package service

import "context"

// Pinger — минимальная зависимость, нужная для проверки доступности БД.
type Pinger interface {
	Ping(ctx context.Context) error
}

type HealthService struct {
	pingerDB    Pinger
	pingerCache Pinger
}

func NewHealthService(pingerDB, pingerCache Pinger) *HealthService {
	return &HealthService{
		pingerDB:    pingerDB,
		pingerCache: pingerCache,
	}
}

func (h *HealthService) CheckDB(ctx context.Context) error {
	return h.pingerDB.Ping(ctx)
}

func (h *HealthService) CheckCache(ctx context.Context) error {
	return h.pingerCache.Ping(ctx)
}
