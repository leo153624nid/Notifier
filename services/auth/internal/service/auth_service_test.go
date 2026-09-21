package service

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"authservice/internal/domain"
	"authservice/internal/repository"
)

func newMockMemoryRepository(email string) *repository.MemoryRepository {
	existUser := domain.User{
		Email: email,
	}
	repo := repository.NewMemoryRepository()
	_, _ = repo.Create(context.Background(), existUser)

	return repo
}

func TestNewAuthService(t *testing.T) {
	repo := repository.NewMemoryRepository()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	secret := "secret"
	ttl := 10 * time.Minute

	tests := []struct {
		name    string
		wantErr bool
	}{
		{name: "valid", wantErr: false},
		{name: "no repo", wantErr: true},
		{name: "no logger", wantErr: true},
		{name: "no secret", wantErr: true},
		{name: "no ttl", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			switch tt.name {
			case "no repo":
				_, err = NewAuthService(nil, logger, secret, ttl)
			case "no logger":
				_, err = NewAuthService(repo, nil, secret, ttl)
			case "no secret":
				_, err = NewAuthService(repo, nil, "", ttl)
			case "no ttl":
				_, err = NewAuthService(repo, logger, secret, 0)
			default:
				_, err = NewAuthService(repo, logger, secret, ttl)
			}

			if err != nil && !tt.wantErr {
				t.Errorf("NewAuthService() error: %s", err)
			}
			if err == nil && tt.wantErr {
				t.Errorf("err is nil, want error")
			}
		})
	}
}

func TestRegister(t *testing.T) {
	existEmail := "exist@mail.com"

	tests := []struct {
		name     string
		email    string
		password string
		wantErr  bool
	}{
		{
			name:     "correct new user",
			email:    "test@mail.com",
			password: "12345678",
			wantErr:  false,
		},
		{
			name:     "email with problems",
			email:    " Test@mail.Com",
			password: "12345678",
			wantErr:  false,
		},
		{
			name:     "exist user",
			email:    existEmail,
			password: "12345678",
			wantErr:  true,
		},
		{
			name:     "empty email",
			email:    "",
			password: "12345678",
			wantErr:  true,
		},
		{
			name:     "wrong email",
			email:    "warong @mail.com",
			password: "12345678",
			wantErr:  true,
		},
		{
			name:     "password with spaces",
			email:    "test@mail.com",
			password: " 12345678 ",
			wantErr:  false,
		},
		{
			name:     "empty password",
			email:    "test@mail.com",
			password: "",
			wantErr:  true,
		},
		{
			name:     "short password",
			email:    "test@mail.com",
			password: "1234567",
			wantErr:  true,
		},
		{
			name:     "long password",
			email:    "test@mail.com",
			password: string(make([]byte, 61)),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockMemoryRepository(existEmail)
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			secret := "secret"
			ttl := 10 * time.Minute

			s, err := NewAuthService(repo, logger, secret, ttl)
			if err != nil {
				t.Fatalf("NewAuthService() failed")
			}

			u, err := s.Register(context.Background(), tt.email, tt.password)

			if err != nil && !tt.wantErr {
				t.Errorf("Register() error: %s", err)
			}
			if err == nil && tt.wantErr {
				t.Errorf("err is nil, want error")
			}

			if err == nil && !tt.wantErr {
				if u.Email != strings.ToLower(strings.TrimSpace(tt.email)) {
					t.Errorf("wrong email, want: %s, got: %s", tt.email, u.Email)
				}
				if u.PasswordHash != "" {
					t.Errorf("passwordHash isnt empty, got: %s", u.PasswordHash)
				}
			}
		})
	}
}

func TestLogin(t *testing.T) {
	existEmail := "exist@mail.com"
	existPassword := "12345678"

	tests := []struct {
		name     string
		email    string
		password string
		wantErr  bool
	}{
		{
			name:     "no such user",
			email:    "test@mail.com",
			password: "12345678",
			wantErr:  true,
		},
		{
			name:     "correct",
			email:    existEmail,
			password: existPassword,
			wantErr:  false,
		},
		{
			name:     "empty email",
			email:    "",
			password: "12345678",
			wantErr:  true,
		},
		{
			name:     "wrong email",
			email:    "warong @mail.com",
			password: "12345678",
			wantErr:  true,
		},
		{
			name:     "wrong password",
			email:    existEmail,
			password: existPassword + "1",
			wantErr:  true,
		},
		{
			name:     "password with spaces",
			email:    existEmail,
			password: " " + existPassword + " ",
			wantErr:  true,
		},
		{
			name:     "empty password",
			email:    existEmail,
			password: "",
			wantErr:  true,
		},
		{
			name:     "short password",
			email:    existEmail,
			password: "1234567",
			wantErr:  true,
		},
		{
			name:     "long password",
			email:    existEmail,
			password: string(make([]byte, 61)),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := repository.NewMemoryRepository()
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			secret := "secret"
			ttl := 10 * time.Minute

			s, err := NewAuthService(repo, logger, secret, ttl)
			if err != nil {
				t.Fatalf("NewAuthService() failed")
			}
			_, _ = s.Register(context.Background(), existEmail, existPassword)

			tok, err := s.Login(context.Background(), tt.email, tt.password)

			if err != nil && !tt.wantErr {
				t.Errorf("Login() error: %s", err)
			}
			if err == nil && tt.wantErr {
				t.Errorf("err is nil, want error")
			}

			if err == nil && !tt.wantErr {
				if utf8.RuneCountInString(tok) == 0 {
					t.Errorf("empty token")
				}
			}
		})
	}
}
