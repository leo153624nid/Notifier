package service

import "context"

// NotifierClient — исходящий порт к notifier-сервису. AuthService не знает,
// что за ним gRPC, — только этот контракт. Реализация (адаптер) лежит в
// internal/client/notifierclient и подключается через main.go.
type NotifierClient interface {
	Notify(ctx context.Context, email string) error
}
