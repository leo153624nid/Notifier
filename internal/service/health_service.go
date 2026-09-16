package service

import "context"

// Pinger — минимальная зависимость, нужная для проверки доступности БД.
type Pinger interface {
	Ping(ctx context.Context) error
}

type HealthService struct {
	pinger Pinger
}

func NewHealthService(pinger Pinger) *HealthService {
	return &HealthService{pinger: pinger}
}

func (h *HealthService) Check(ctx context.Context) error {
	return h.pinger.Ping(ctx)
}
