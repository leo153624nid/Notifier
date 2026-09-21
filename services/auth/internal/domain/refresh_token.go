package domain

import (
	"time"
	"uuid"
)

// RefreshToken — хранимая в БД запись о выданном refresh-токене.
// В базе лежит не сам токен, а его hash (см. AuthService), чтобы утечка
// БД не давала возможности восстановить рабочие токены.
type RefreshToken struct {
	ExpiresAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time
	TokenHash string
	ID        uuid.UUID
	UserID    uuid.UUID
}

// IsActive сообщает, можно ли ещё обменять этот refresh-токен на новую пару.
func (t RefreshToken) IsActive() bool {
	return t.RevokedAt == nil && time.Now().Before(t.ExpiresAt)
}
