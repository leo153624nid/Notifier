package domain

import (
	"fmt"
	"strings"
	"time"
	"uuid"
)

type User struct {
	CreatedAt    time.Time `json:"created_at"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	ID           uuid.UUID `json:"id"`
}

func (u User) Validate() error {
	const op = "User.Validate"

	if u.ID == uuid.Nil() {
		return fmt.Errorf("%s: id is required", op)
	}
	if strings.TrimSpace(u.Email) == "" {
		return fmt.Errorf("%s: email is required", op)
	}
	if strings.TrimSpace(u.PasswordHash) == "" {
		return fmt.Errorf("%s: password is required", op)
	}
	if u.CreatedAt.IsZero() {
		return fmt.Errorf("%s: wrong creation time", op)
	}

	return nil
}

func (u User) String() string {
	return fmt.Sprintf("User{email:%s, password_hash:%s}", u.Email, u.PasswordHash)
}
