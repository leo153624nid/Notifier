package service

import (
	"fmt"
	"log/slog"

	"golang.org/x/crypto/bcrypt"

	"authservice/internal/repository"
)

type AuthService struct {
	repo   repository.UserRepo
	logger *slog.Logger
}

func NewAuthService(
	repo repository.UserRepo,
	logger *slog.Logger,
) (*AuthService, error) {
	const op = "NewAuthService"

	if repo == nil {
		return nil, fmt.Errorf("%s: repo is required", op)
	}
	if logger == nil {
		return nil, fmt.Errorf("%s: logger is required", op)
	}

	return &AuthService{
		repo:   repo,
		logger: logger,
	}, nil
}

func passwordToHash(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}

func compareHashAndPassword(hash string, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
