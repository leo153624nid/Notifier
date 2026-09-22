package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"authservice/internal/domain"
	"authservice/internal/repository"
)

// mockNotifierClient — тестовая заглушка NotifierClient. В отличие от
// настоящего notifierclient.Client (у него нулевое значение содержит nil
// gRPC-клиент и паникует при вызове Notify), эта заглушка безопасна для
// прямого использования в тестах и позволяет проверить сам факт вызова.
type mockNotifierClient struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (m *mockNotifierClient) Notify(_ context.Context, email string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.calls = append(m.calls, email)
	return m.err
}

func (m *mockNotifierClient) callsSnapshot() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	return append([]string(nil), m.calls...)
}

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

	return newTestAuthServiceWithNotifier(t, repo, &mockNotifierClient{})
}

// newTestAuthServiceWithNotifier — как newTestAuthService, но с конкретным
// NotifierClient — нужен тестам, которые проверяют сам факт/результат
// вызова Notify (TestRegister_Notifies*).
func newTestAuthServiceWithNotifier(t *testing.T, repo repository.UserRepo, notifier NotifierClient) *AuthService {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewAuthService(
		repo,
		logger,
		"secret",
		10*time.Minute,
		24*time.Hour,
		notifier,
	)
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
	notifierClient := &mockNotifierClient{}

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
				_, err = NewAuthService(nil, logger, secret, accessTTL, refreshTTL, notifierClient)
			case "no logger":
				_, err = NewAuthService(repo, nil, secret, accessTTL, refreshTTL, notifierClient)
			case "no secret":
				_, err = NewAuthService(repo, logger, "", accessTTL, refreshTTL, notifierClient)
			case "no access ttl":
				_, err = NewAuthService(repo, logger, secret, 0, refreshTTL, notifierClient)
			case "no refresh ttl":
				_, err = NewAuthService(repo, logger, secret, accessTTL, 0, notifierClient)
			default:
				_, err = NewAuthService(repo, logger, secret, accessTTL, refreshTTL, notifierClient)
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

			u, err := s.Register(context.Background(), tt.email, tt.password, "testRequestID")

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

// TestRegister_NotifiesOnSuccess проверяет, что успешная регистрация
// асинхронно вызывает NotifierClient.Notify с email нового пользователя.
// s.Wait() дожидается завершения фоновой горутины notifyByEmail — без
// этого проверка calls была бы гонкой (см. AuthService.Register).
func TestRegister_NotifiesOnSuccess(t *testing.T) {
	repo := repository.NewMemoryRepository()
	notifier := &mockNotifierClient{}
	s := newTestAuthServiceWithNotifier(t, repo, notifier)

	u, err := s.Register(context.Background(), "new@mail.com", "12345678", "req-1")
	if err != nil {
		t.Fatalf("Register() error: %s", err)
	}

	s.Wait()

	calls := notifier.callsSnapshot()
	if len(calls) != 1 || calls[0] != u.Email {
		t.Errorf("Notify calls = %v, want exactly one call with %q", calls, u.Email)
	}
}

// TestRegister_SucceedsEvenIfNotifyFails фиксирует намеренное архитектурное
// решение: отправка уведомления — сайд-эффект, а не часть транзакции
// регистрации. Сбой Notify не должен приводить к ошибке Register.
func TestRegister_SucceedsEvenIfNotifyFails(t *testing.T) {
	repo := repository.NewMemoryRepository()
	notifier := &mockNotifierClient{err: errors.New("notifier unavailable")}
	s := newTestAuthServiceWithNotifier(t, repo, notifier)

	u, err := s.Register(context.Background(), "new2@mail.com", "12345678", "req-2")
	if err != nil {
		t.Fatalf("Register() error: %s, want nil even though Notify fails", err)
	}
	if u.Email != "new2@mail.com" {
		t.Errorf("unexpected email: %s", u.Email)
	}

	s.Wait()

	calls := notifier.callsSnapshot()
	if len(calls) != 1 {
		t.Errorf("Notify calls = %v, want exactly one attempt despite the error", calls)
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
			_, _ = s.Register(context.Background(), existEmail, existPassword, "testRequestID")

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

func TestDelete(t *testing.T) {
	existEmail := "exist@mail.com"
	existPassword := "12345678"

	t.Run("valid refresh token deletes user and revokes token", func(t *testing.T) {
		repo := repository.NewMemoryRepository()
		s := newTestAuthService(t, repo)
		_, err := s.Register(context.Background(), existEmail, existPassword, "testRequestID")
		if err != nil {
			t.Fatalf("Register() error: %s", err)
		}

		pair, err := s.Login(context.Background(), existEmail, existPassword)
		if err != nil {
			t.Fatalf("Login() error: %s", err)
		}

		if err := s.Delete(context.Background(), pair.RefreshToken); err != nil {
			t.Fatalf("Delete() error: %s", err)
		}

		// пользователь удалён — повторный вход невозможен
		if _, err := s.Login(context.Background(), existEmail, existPassword); !errors.Is(err, domain.ErrInvalidCredentials) {
			t.Errorf("Login() after Delete error = %v, want %v", err, domain.ErrInvalidCredentials)
		}

		// refresh-токен отозван вместе с удалением
		if _, err := s.Refresh(context.Background(), pair.RefreshToken); !errors.Is(err, domain.ErrInvalidToken) {
			t.Errorf("Refresh() after Delete error = %v, want %v", err, domain.ErrInvalidToken)
		}
	})

	t.Run("empty token", func(t *testing.T) {
		repo := repository.NewMemoryRepository()
		s := newTestAuthService(t, repo)

		if err := s.Delete(context.Background(), ""); !errors.Is(err, domain.ErrInvalidToken) {
			t.Errorf("Delete() error = %v, want %v", err, domain.ErrInvalidToken)
		}
	})

	t.Run("garbage token", func(t *testing.T) {
		repo := repository.NewMemoryRepository()
		s := newTestAuthService(t, repo)

		if err := s.Delete(context.Background(), "not-a-token"); !errors.Is(err, domain.ErrInvalidToken) {
			t.Errorf("Delete() error = %v, want %v", err, domain.ErrInvalidToken)
		}
	})

	t.Run("access token used instead of refresh", func(t *testing.T) {
		repo := repository.NewMemoryRepository()
		s := newTestAuthService(t, repo)
		_, _ = s.Register(context.Background(), existEmail, existPassword, "testRequestID")

		pair, err := s.Login(context.Background(), existEmail, existPassword)
		if err != nil {
			t.Fatalf("Login() error: %s", err)
		}

		if err := s.Delete(context.Background(), pair.AccessToken); !errors.Is(err, domain.ErrInvalidToken) {
			t.Errorf("Delete() with access token error = %v, want %v", err, domain.ErrInvalidToken)
		}
	})

	t.Run("already used refresh token", func(t *testing.T) {
		repo := repository.NewMemoryRepository()
		s := newTestAuthService(t, repo)
		_, _ = s.Register(context.Background(), existEmail, existPassword, "testRequestID")

		pair, err := s.Login(context.Background(), existEmail, existPassword)
		if err != nil {
			t.Fatalf("Login() error: %s", err)
		}

		// ротация делает refresh-токен использованным (отозванным),
		// но не удаляет пользователя из репозитория
		if _, err := s.Refresh(context.Background(), pair.RefreshToken); err != nil {
			t.Fatalf("Refresh() error: %s", err)
		}

		if err := s.Delete(context.Background(), pair.RefreshToken); err != nil {
			t.Errorf("Delete() with already-used refresh token error: %s, want nil (JWT itself is still structurally valid)", err)
		}
	})
}

func TestRefresh(t *testing.T) {
	existEmail := "exist@mail.com"
	existPassword := "12345678"

	t.Run("valid refresh rotates token", func(t *testing.T) {
		repo := repository.NewMemoryRepository()
		s := newTestAuthService(t, repo)
		_, _ = s.Register(context.Background(), existEmail, existPassword, "testRequestID")

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
		_, _ = s.Register(context.Background(), existEmail, existPassword, "testRequestID")

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
		_, _ = s.Register(context.Background(), existEmail, existPassword, "testRequestID")

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
