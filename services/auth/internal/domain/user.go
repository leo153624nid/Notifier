package domain

import (
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

func (u User) Validate() error {
	const op = "User.Validate"

	if u.ID == uuid.Nil() {
		return fmt.Errorf("%s: id is required", op)
	}
	email := strings.ToLower(strings.TrimSpace(u.Email))
	if email == "" {
		return fmt.Errorf("%s: email is required", op)
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("%s: wrong email format", op)
	}
	if strings.TrimSpace(u.PasswordHash) == "" {
		return fmt.Errorf("%s: password is required", op)
	}
	if utf8.RuneCountInString(u.PasswordHash) > 60 {
		return fmt.Errorf("%s: password is long", op)
	}
	if u.CreatedAt.IsZero() {
		return fmt.Errorf("%s: wrong creation time", op)
	}

	return nil
}

func (u User) String() string {
	return fmt.Sprintf("User{email:%s, password_hash:%s}", u.Email, u.PasswordHash)
}
