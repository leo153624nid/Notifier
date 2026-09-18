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
	const op = "Token.Issue"

	if userID == uuid.Nil() {
		return "", fmt.Errorf("%s: wrong user ID", op)
	}
	if secret == "" {
		return "", fmt.Errorf("%s: secret is empty", op)
	}
	if ttl == 0 {
		return "", fmt.Errorf("%s: ttl is zero", op)
	}

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
	const op = "Token.Parse"

	if tokenString == "" {
		return Claims{}, fmt.Errorf("%s: tokenString is empty", op)
	}
	if secret == "" {
		return Claims{}, fmt.Errorf("%s: secret is empty", op)
	}

	var claims Claims

	tok, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		if t.Header["alg"] != jwt.SigningMethodHS256.Alg() {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})
	if err != nil || !tok.Valid {
		return Claims{}, ErrInvalidToken
	}

	return claims, nil
}
