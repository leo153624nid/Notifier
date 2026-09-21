package token

import (
	"testing"
	"time"
	"unicode/utf8"
	"uuid"

	"github.com/golang-jwt/jwt/v5"
)

func checkPrefixSuffix(str, substr string) bool {
	return len(str) >= len(substr) && (str == substr || (len(substr) > 0 && (str[:len(substr)] == substr || str[len(str)-len(substr):] == substr)))
}

//nolint:govet
func TestIssue(t *testing.T) {
	tests := []struct {
		name    string
		id      uuid.UUID
		secret  string
		ttl     time.Duration
		wantErr bool
	}{
		{
			name:    "valid",
			id:      uuid.New(),
			secret:  "secret",
			ttl:     1 * time.Minute,
			wantErr: false,
		},
		{
			name:    "no id",
			id:      uuid.Nil(),
			secret:  "secret",
			ttl:     1 * time.Minute,
			wantErr: true,
		},
		{
			name:    "empty secret",
			id:      uuid.New(),
			secret:  "",
			ttl:     1 * time.Minute,
			wantErr: true,
		},
		{
			name:    "no ttl",
			id:      uuid.New(),
			secret:  "secret",
			ttl:     0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tok, err := Issue(tt.id, tt.secret, tt.ttl)

			if err != nil && !tt.wantErr {
				t.Errorf("Issue() error: %s", err)
			}
			if err == nil && tt.wantErr {
				t.Errorf("err is nil, want error")
			}

			if err == nil && !tt.wantErr {
				if utf8.RuneCountInString(tok) == 0 {
					t.Errorf("token is empty")
				}

				claims, err := Parse(tok, tt.secret)
				if err != nil {
					t.Errorf("Parse() error: %s", err)
				}

				if claims.UserID != tt.id {
					t.Errorf("wrong ID, want: %s, got: %s", tt.id, claims.UserID)
				}
				if claims.ExpiresAt.Before(time.Now()) {
					t.Errorf("token is expired")
				}
			}
		})
	}
}

func TestParse(t *testing.T) {
	secret := "correct_super_secret_key_123"
	userID := uuid.New()

	// 1. Готовим просроченный токен
	expiredTime := time.Now().Add(-1 * time.Hour) // Время в прошлом
	expiredClaims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(expiredTime.Add(-15 * time.Minute)),
			ExpiresAt: jwt.NewNumericDate(expiredTime),
		},
	}
	expiredToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, expiredClaims).SignedString([]byte(secret))

	// 2. Готовим токен с правильным временем, но подписанный другим секретом (битая подпись)
	validClaims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(15 * time.Minute)),
		},
	}
	badSignatureToken, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, validClaims).SignedString([]byte("WRONG_SECRET_KEY"))

	tests := []struct {
		name        string
		tokenString string
		secret      string
		expectedErr string
	}{
		{
			name:        "empty tokenString",
			tokenString: "",
			secret:      secret,
			expectedErr: "tokenString is empty",
		},
		{
			name:        "empty secret",
			tokenString: expiredToken,
			secret:      "",
			expectedErr: "secret is empty",
		},
		{
			name:        "Просроченный токен должен возвращать ошибку",
			tokenString: expiredToken,
			secret:      secret,
			expectedErr: ErrInvalidToken.Error(),
		},
		{
			name:        "Токен c битой подписью должен возвращать ошибку",
			tokenString: badSignatureToken,
			secret:      secret,
			expectedErr: ErrInvalidToken.Error(),
		},
		{
			name:        "Абсолютно случайная строка вместо токена",
			tokenString: "not.a.valid.jwt.string",
			secret:      secret,
			expectedErr: ErrInvalidToken.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.tokenString, tt.secret)

			if err == nil {
				t.Fatalf("want err, but got nil")
			}

			if !checkPrefixSuffix(err.Error(), tt.expectedErr) {
				t.Errorf("want: %q, got: %q", tt.expectedErr, err.Error())
			}
		})
	}
}
