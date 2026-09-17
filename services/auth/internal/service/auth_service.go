package service

import (
	"fmt"
	"log/slog"

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
