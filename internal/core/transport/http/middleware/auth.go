package core_http_middleware

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"uuid"

	"github.com/golang-jwt/jwt/v5"

	core_errors "notifier/internal/core/errors"
	core_logger "notifier/internal/core/logger"
	core_http_response "notifier/internal/core/transport/http/response"
)

type userIDKeyType struct{}

var userIDKey = userIDKeyType{}

// accessTokenType — значение claim'а "type" у access-токенов, которые
// выпускает auth-сервис (см. services/auth/internal/token.TypeAccess).
// Refresh-токены (TypeRefresh) сюда предъявлять нельзя — они предназначены
// только для эндпоинта POST /api/v1/auth/refresh самого auth-сервиса.
const accessTokenType = "access"

// jwtClaims — формат токена, который выдаёт auth-сервис (см.
// services/auth/internal/token.Claims). Notifier только проверяет подпись
// и вычитывает userID, сам токены не выпускает.
type jwtClaims struct {
	jwt.RegisteredClaims
	Type   string    `json:"type"`
	UserID uuid.UUID `json:"sub"`
}

func getUserID(ctx context.Context) (uuid.UUID, bool) { // TODO: delete ?
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}

func bearerToken(r *http.Request) (string, bool) {
	const prefix = "Bearer "

	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, prefix) {
		return "", false
	}

	tok := strings.TrimSpace(strings.TrimPrefix(h, prefix))
	if tok == "" {
		return "", false
	}

	return tok, true
}

func parseUserID(tokenString, secret string) (uuid.UUID, error) {
	var claims jwtClaims

	tok, err := jwt.ParseWithClaims(tokenString, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		if t.Header["alg"] != jwt.SigningMethodHS256.Alg() {
			return nil, jwt.ErrTokenSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("parse token: %w", err)
	}
	if !tok.Valid {
		return uuid.UUID{}, fmt.Errorf("parse token: invalid token")
	}
	if claims.Type != accessTokenType {
		return uuid.UUID{}, fmt.Errorf("parse token: wrong token type")
	}

	return claims.UserID, nil
}

func Auth(jwtSecret string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			logger := core_logger.FromContext(ctx)
			rh := core_http_response.NewHTTPResponseHandler(logger, w)

			tokenString, ok := bearerToken(r)
			if !ok {
				rh.AuthErrorResponse(
					"invalid or missing bearer token",
					r.URL.Path,
					r.Method,
					r.RemoteAddr,
					core_errors.ErrAuth,
				)
				return
			}

			userID, err := parseUserID(tokenString, jwtSecret)
			if err != nil {
				rh.AuthErrorResponse(
					"invalid or expired token",
					r.URL.Path,
					r.Method,
					r.RemoteAddr,
					err,
				)
				return
			}

			ctx = context.WithValue(ctx, userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
