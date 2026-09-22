package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/mail"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
	"uuid"

	"golang.org/x/crypto/bcrypt"

	"authservice/internal/domain"
	"authservice/internal/repository"
	"authservice/internal/token"
)

// TokenPair — пара токенов, выдаваемая при логине и обновлении сессии.
// AccessToken живёт недолго и используется для авторизации запросов,
// RefreshToken — долго и используется только для получения новой пары.
type TokenPair struct {
	AccessToken  string
	RefreshToken string
}

type AuthService struct {
	repo          repository.UserRepo
	logger        *slog.Logger
	jwtSecret     string
	jwtAccessTTL  time.Duration
	jwtRefreshTTL time.Duration
	notifier      NotifierClient
	wg            sync.WaitGroup
}

func NewAuthService(
	repo repository.UserRepo,
	logger *slog.Logger,
	jwtSecret string,
	jwtAccessTTL time.Duration,
	jwtRefreshTTL time.Duration,
	notifier NotifierClient,
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
	if jwtAccessTTL == 0 {
		return nil, fmt.Errorf("%s: jwt access ttl is required", op)
	}
	if jwtRefreshTTL == 0 {
		return nil, fmt.Errorf("%s: jwt refresh ttl is required", op)
	}
	if notifier == nil {
		return nil, fmt.Errorf("%s: notifier client is required", op)
	}

	return &AuthService{
		repo:          repo,
		logger:        logger,
		jwtSecret:     jwtSecret,
		jwtAccessTTL:  jwtAccessTTL,
		jwtRefreshTTL: jwtRefreshTTL,
		notifier:      notifier,
	}, nil
}

func passwordToHash(password string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
}

func compareHashAndPassword(hash string, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func hashRefreshToken(tok string) string {
	sum := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(sum[:])
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

func (s *AuthService) Register(
	ctx context.Context,
	email string,
	password string,
	requestID string,
) (domain.User, error) {
	const op = "AuthService.Register"

	reqLogger := s.logger.With("request_id", requestID)
	email = strings.ToLower(strings.TrimSpace(email))

	if !emailIsValid(email) || !passwordIsValid(password) {
		return domain.User{}, domain.ErrInvalidCredentials
	}

	hash, err := passwordToHash(password)
	if err != nil {
		reqLogger.Error("password to hash failed", "op", op, "error", err)
		return domain.User{}, domain.ErrInvalidCredentials
	}

	u := domain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    time.Now(),
	}
	if validErr := u.Validate(); validErr != nil {
		return domain.User{}, fmt.Errorf("%s: validate: %w", op, validErr)
	}

	id, err := s.repo.Create(ctx, u)
	if err != nil {
		if errors.Is(err, domain.ErrUserExists) {
			return domain.User{}, domain.ErrUserExists
		}
		reqLogger.Error("create failed", "op", op, "error", err)
		return domain.User{}, fmt.Errorf("%s: create: %w", op, err)
	}

	s.wg.Add(1)
	go s.notifyByEmail(u.Email, requestID)

	u.ID = id
	u.PasswordHash = "" // without password
	return u, nil
}

func (s *AuthService) notifyByEmail(email string, requestID string) {
	defer s.wg.Done()

	reqLogger := s.logger.With("request_id", requestID)

	notifyCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := s.notifier.Notify(notifyCtx, email)
	if err != nil {
		reqLogger.Error("notify failed", "email", email, "error", err)
	}
}

func (s *AuthService) Login(ctx context.Context, email, password string) (TokenPair, error) {
	const op = "AuthService.Login"

	email = strings.ToLower(strings.TrimSpace(email))
	if !emailIsValid(email) {
		return TokenPair{}, domain.ErrInvalidCredentials
	}

	u, err := s.repo.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidCredentials) {
			return TokenPair{}, domain.ErrInvalidCredentials
		}
		s.logger.Error("get by email failed", "op", op, "error", err)
		return TokenPair{}, err
	}

	if !compareHashAndPassword(u.PasswordHash, password) {
		return TokenPair{}, domain.ErrInvalidCredentials
	}

	pair, err := s.issueTokenPair(ctx, u.ID)
	if err != nil {
		s.logger.Error("token failed", "op", op, "error", err)
		return TokenPair{}, err
	}

	return pair, nil
}

func (s *AuthService) Delete(ctx context.Context, refreshToken string) error {
	const op = "AuthService.Delete"

	if refreshToken == "" {
		return domain.ErrInvalidToken
	}

	claims, err := token.Parse(refreshToken, s.jwtSecret)
	if err != nil {
		return domain.ErrInvalidToken
	}
	if claims.Type != token.TypeRefresh {
		return domain.ErrInvalidToken
	}

	logErr := s.Logout(ctx, refreshToken)
	if logErr != nil {
		return fmt.Errorf("%s: %w", op, logErr)
	}

	delErr := s.repo.Delete(ctx, claims.UserID)
	if delErr != nil {
		return fmt.Errorf("%s: %w", op, delErr)
	}

	return nil
}

// Refresh обменивает действующий refresh-токен на новую пару токенов.
// Использованный refresh-токен сразу отзывается (ротация) — повторное
// его предъявление больше не сработает.
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	const op = "AuthService.Refresh"

	if refreshToken == "" {
		return TokenPair{}, domain.ErrInvalidToken
	}

	claims, err := token.Parse(refreshToken, s.jwtSecret)
	if err != nil {
		return TokenPair{}, domain.ErrInvalidToken
	}
	if claims.Type != token.TypeRefresh {
		return TokenPair{}, domain.ErrInvalidToken
	}

	hash := hashRefreshToken(refreshToken)

	stored, err := s.repo.GetRefreshToken(ctx, hash)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidToken) {
			return TokenPair{}, domain.ErrInvalidToken
		}
		s.logger.Error("get refresh token failed", "op", op, "error", err)
		return TokenPair{}, fmt.Errorf("%s: %w", op, err)
	}
	if !stored.IsActive() {
		return TokenPair{}, domain.ErrInvalidToken
	}

	if revokeErr := s.repo.RevokeRefreshToken(ctx, hash); revokeErr != nil {
		s.logger.Error("revoke refresh token failed", "op", op, "error", revokeErr)
		return TokenPair{}, fmt.Errorf("%s: %w", op, revokeErr)
	}

	pair, err := s.issueTokenPair(ctx, claims.UserID)
	if err != nil {
		s.logger.Error("token failed", "op", op, "error", err)
		return TokenPair{}, err
	}

	return pair, nil
}

// Logout отзывает refresh-токен, лишая его возможности выпустить новую пару.
// Уже выданный access-токен продолжит действовать до истечения своего TTL.
func (s *AuthService) Logout(ctx context.Context, refreshToken string) error {
	const op = "AuthService.Logout"

	if refreshToken == "" {
		return domain.ErrInvalidToken
	}

	if err := s.repo.RevokeRefreshToken(ctx, hashRefreshToken(refreshToken)); err != nil {
		s.logger.Error("revoke refresh token failed", "op", op, "error", err)
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (s *AuthService) issueTokenPair(ctx context.Context, userID uuid.UUID) (TokenPair, error) {
	const op = "AuthService.issueTokenPair"

	access, err := token.IssueAccess(userID, s.jwtSecret, s.jwtAccessTTL)
	if err != nil {
		return TokenPair{}, fmt.Errorf("%s: access: %w", op, err)
	}

	refresh, err := token.IssueRefresh(userID, s.jwtSecret, s.jwtRefreshTTL)
	if err != nil {
		return TokenPair{}, fmt.Errorf("%s: refresh: %w", op, err)
	}

	rt := domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		TokenHash: hashRefreshToken(refresh),
		ExpiresAt: time.Now().Add(s.jwtRefreshTTL),
		CreatedAt: time.Now(),
	}
	if err := s.repo.CreateRefreshToken(ctx, rt); err != nil {
		return TokenPair{}, fmt.Errorf("%s: create refresh token: %w", op, err)
	}

	return TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
	}, nil
}

// Wait блокируется до завершения всех фоновых отправок — используется при graceful shutdown.
func (s *AuthService) Wait() {
	s.wg.Wait()
}
