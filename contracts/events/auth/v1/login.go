package authevents

import (
	"time"
	"uuid"
)

type UserLoggedIn struct {
	EventID    uuid.UUID `json:"event_id"`
	UserID     uuid.UUID `json:"user_id"`
	Email      string    `json:"email"`
	OccurredAt time.Time `json:"occurred_at"`
}
