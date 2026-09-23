package authevents

import (
	"time"
	"uuid"
)

const TopicUserLoggedIn = "auth.user.logged_in"
const LoginConsumerGroupID = "notifier-login-consumer"

type UserLoggedIn struct {
	EventID    uuid.UUID `json:"event_id"`
	UserID     uuid.UUID `json:"user_id"`
	Email      string    `json:"email"`
	OccurredAt time.Time `json:"occurred_at"`
}
