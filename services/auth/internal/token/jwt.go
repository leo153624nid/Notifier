package token

import (
	"fmt"
	"time"
	"uuid"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID uuid.UUID `json:"sub"`
	jwt.RegisteredClaims
}

func Issue(userID uuid.UUID, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return tok.SignedString([]byte(secret))
}

func Parse(tokenString, secret string) (Claims, error) {
	var claims Claims

	tok, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) {
		// TODO: check that crypto algorithm is HS256
		return []byte(secret), nil
	})
	if err != nil || !tok.Valid {
		return Claims{}, fmt.Errorf("token invalid: %w", err)
	}

	return claims, nil
}
