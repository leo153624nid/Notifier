package service

import (
	"context"
	"errors"
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

func newTestAuthService(t *testing.T, repo repository.UserRepo) *AuthService {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewAuthService(repo, logger, "secret", 10*time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatalf("NewAuthService() failed: %s", err)
	}

	return s
}

func TestNewAuthService(t *testing.T) {
	repo := repository.NewMemoryRepository()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	secret := "secret"
	accessTTL := 10 * time.Minute
	refreshTTL := 24 * time.Hour

	tests := []struct {
		name    string
		wantErr bool
	}{
		{name: "valid", wantErr: false},
		{name: "no repo", wantErr: true},
		{name: "no logger", wantErr: true},
		{name: "no secret", wantErr: true},
		{name: "no access ttl", wantErr: true},
		{name: "no refresh ttl", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			switch tt.name {
			case "no repo":
				_, err = NewAuthService(nil, logger, secret, accessTTL, refreshTTL)
			case "no logger":
				_, err = NewAuthService(repo, nil, secret, accessTTL, refreshTTL)
			case "no secret":
				_, err = NewAuthService(repo, logger, "", accessTTL, refreshTTL)
			case "no access ttl":
				_, err = NewAuthService(repo, logger, secret, 0, refreshTTL)
			case "no refresh ttl":
				_, err = NewAuthService(repo, logger, secret, accessTTL, 0)
			default:
				_, err = NewAuthService(repo, logger, secret, accessTTL, refreshTTL)
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
			s := newTestAuthService(t, repo)

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
			s := newTestAuthService(t, repo)
			_, _ = s.Register(context.Background(), existEmail, existPassword)

			pair, err := s.Login(context.Background(), tt.email, tt.password)

			if err != nil && !tt.wantErr {
				t.Errorf("Login() error: %s", err)
			}
			if err == nil && tt.wantErr {
				t.Errorf("err is nil, want error")
			}

			if err == nil && !tt.wantErr {
				if utf8.RuneCountInString(pair.AccessToken) == 0 {
					t.Errorf("empty access token")
				}
				if utf8.RuneCountInString(pair.RefreshToken) == 0 {
					t.Errorf("empty refresh token")
				}
			}
		})
	}
}

func TestRefresh(t *testing.T) {
	existEmail := "exist@mail.com"
	existPassword := "12345678"

	t.Run("valid refresh rotates token", func(t *testing.T) {
		repo := repository.NewMemoryRepository()
		s := newTestAuthService(t, repo)
		_, _ = s.Register(context.Background(), existEmail, existPassword)

		pair, err := s.Login(context.Background(), existEmail, existPassword)
		if err != nil {
			t.Fatalf("Login() error: %s", err)
		}

		newPair, err := s.Refresh(context.Background(), pair.RefreshToken)
		if err != nil {
			t.Fatalf("Refresh() error: %s", err)
		}
		if newPair.RefreshToken == pair.RefreshToken {
			t.Errorf("refresh token wasn't rotated")
		}
		if utf8.RuneCountInString(newPair.AccessToken) == 0 {
			t.Errorf("empty access token")
		}

		// старый refresh-токен использовать повторно нельзя
		if _, err := s.Refresh(context.Background(), pair.RefreshToken); !errors.Is(err, domain.ErrInvalidToken) {
			t.Errorf("Refresh() with used token error = %v, want %v", err, domain.ErrInvalidToken)
		}
	})

	t.Run("empty token", func(t *testing.T) {
		repo := repository.NewMemoryRepository()
		s := newTestAuthService(t, repo)

		if _, err := s.Refresh(context.Background(), ""); !errors.Is(err, domain.ErrInvalidToken) {
			t.Errorf("Refresh() error = %v, want %v", err, domain.ErrInvalidToken)
		}
	})

	t.Run("garbage token", func(t *testing.T) {
		repo := repository.NewMemoryRepository()
		s := newTestAuthService(t, repo)

		if _, err := s.Refresh(context.Background(), "not-a-token"); !errors.Is(err, domain.ErrInvalidToken) {
			t.Errorf("Refresh() error = %v, want %v", err, domain.ErrInvalidToken)
		}
	})

	t.Run("access token used as refresh", func(t *testing.T) {
		repo := repository.NewMemoryRepository()
		s := newTestAuthService(t, repo)
		_, _ = s.Register(context.Background(), existEmail, existPassword)

		pair, err := s.Login(context.Background(), existEmail, existPassword)
		if err != nil {
			t.Fatalf("Login() error: %s", err)
		}

		if _, err := s.Refresh(context.Background(), pair.AccessToken); !errors.Is(err, domain.ErrInvalidToken) {
			t.Errorf("Refresh() with access token error = %v, want %v", err, domain.ErrInvalidToken)
		}
	})
}

func TestLogout(t *testing.T) {
	existEmail := "exist@mail.com"
	existPassword := "12345678"

	t.Run("valid logout revokes refresh token", func(t *testing.T) {
		repo := repository.NewMemoryRepository()
		s := newTestAuthService(t, repo)
		_, _ = s.Register(context.Background(), existEmail, existPassword)

		pair, err := s.Login(context.Background(), existEmail, existPassword)
		if err != nil {
			t.Fatalf("Login() error: %s", err)
		}

		if err := s.Logout(context.Background(), pair.RefreshToken); err != nil {
			t.Fatalf("Logout() error: %s", err)
		}

		if _, err := s.Refresh(context.Background(), pair.RefreshToken); !errors.Is(err, domain.ErrInvalidToken) {
			t.Errorf("Refresh() after logout error = %v, want %v", err, domain.ErrInvalidToken)
		}
	})

	t.Run("empty token", func(t *testing.T) {
		repo := repository.NewMemoryRepository()
		s := newTestAuthService(t, repo)

		if err := s.Logout(context.Background(), ""); !errors.Is(err, domain.ErrInvalidToken) {
			t.Errorf("Logout() error = %v, want %v", err, domain.ErrInvalidToken)
		}
	})
}
