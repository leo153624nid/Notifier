package service

import (
	"context"
	"uuid"
)

type EventPublisher interface {
	PublishUserLoggedInEvent(ctx context.Context, userID uuid.UUID, email string) error
}
