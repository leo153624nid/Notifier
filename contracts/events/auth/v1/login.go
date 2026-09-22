package authevents

import (
	"time"
	"uuid"
)

const TopicUserLoggedIn = "auth.user.logged_in"

type UserLoggedIn struct {
	UserID     uuid.UUID `json:"user_id"`
	Email      string    `json:"email"`
	OccurredAt time.Time `json:"occurred_at"`
}
