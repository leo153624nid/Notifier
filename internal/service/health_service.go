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

func (s *HealthService) CheckDB(ctx context.Context) error {
	return s.pingerDB.Ping(ctx)
}

func (s *HealthService) CheckCache(ctx context.Context) error {
	return s.pingerCache.Ping(ctx)
}
