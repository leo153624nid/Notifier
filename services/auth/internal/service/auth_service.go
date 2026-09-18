package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
	"uuid"

	"golang.org/x/crypto/bcrypt"

	"authservice/internal/domain"
	"authservice/internal/repository"
	"authservice/internal/token"
)

type AuthService struct {
	repo      repository.UserRepo
	logger    *slog.Logger
	jwtSecret string
	jwtTTL    time.Duration
}

func NewAuthService(
	repo repository.UserRepo,
	logger *slog.Logger,
	jwtSecret string,
	jwtTTL time.Duration,
) (*AuthService, error) {
	const op = "NewAuthService"

	if repo == nil {
		return nil, fmt.Errorf("%s: repo is required", op)
	}
	if logger == nil {
		return nil, fmt.Errorf("%s: logger is required", op)
	}
	if jwtSecret == "" {
		return nil, fmt.Errorf("%s: jwt secret is required", op)
	}
	if jwtTTL == 0 {
		return nil, fmt.Errorf("%s: jwt ttl is required", op)
	}

	return &AuthService{
		repo:      repo,
		logger:    logger,
		jwtSecret: jwtSecret,
		jwtTTL:    jwtTTL,
	}, nil
}

func passwordToHash(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}

func compareHashAndPassword(hash string, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func emailIsValid(e string) bool {
	email := strings.ToLower(strings.TrimSpace(e))
	if email == "" {
		return false
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return false
	}
	return true
}

func passwordIsValid(p string) bool {
	password := strings.TrimSpace(p)
	if password == "" {
		return false
	}
	if utf8.RuneCountInString(password) < 8 {
		return false
	}
	if len(password) > 60 {
		return false
	}
	return true
}

func (s *AuthService) Register(ctx context.Context, email, password string) (domain.User, error) {
	const op = "AuthService.Register"

	email = strings.ToLower(strings.TrimSpace(email))

	if !emailIsValid(email) || !passwordIsValid(password) {
		return domain.User{}, domain.ErrInvalidCredentials
	}

	hash, err := passwordToHash(password)
	if err != nil {
		s.logger.Error("password to hash failed", "op", op, "error", err)
		return domain.User{}, domain.ErrInvalidCredentials
	}

	u := domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    time.Now(),
	}
	if err := u.Validate(); err != nil {
		return domain.User{}, fmt.Errorf("%s: validate: %w", op, err)
	}

	id, err := s.repo.Create(ctx, u)
	if err != nil {
		if errors.Is(err, domain.ErrUserExists) {
			return domain.User{}, domain.ErrUserExists
		}
		s.logger.Error("create failed", "op", op, "error", err)
		return domain.User{}, fmt.Errorf("%s: create: %w", op, err)
	}

	u.ID = id
	u.PasswordHash = "" // without password
	return u, nil
}

func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	const op = "AuthService.Login"

	email = strings.ToLower(strings.TrimSpace(email))
	if !emailIsValid(email) {
		return "", domain.ErrInvalidCredentials
	}

	u, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			return "", domain.ErrInvalidCredentials
		}
		s.logger.Error("get by email failed", "op", op, "error", err)
		return "", err
	}

	if !compareHashAndPassword(u.PasswordHash, password) {
		return "", domain.ErrInvalidCredentials
	}

	tok, err := token.Issue(u.ID, s.jwtSecret, s.jwtTTL)
	if err != nil {
		s.logger.Error("token failed", "op", op, "error", err)
		return "", err
	}

	return tok, nil
}
