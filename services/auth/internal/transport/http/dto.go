package http

import (
	"encoding/json"
	"net/http"

	"authservice/internal/service"
)

type APIError struct {
	Error string `json:"error"`
}

// UserRequest — тело запроса POST /api/v1/auth/register
// UserRequest — тело запроса POST /api/v1/auth/login
type UserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RefreshRequest — тело запроса POST /api/v1/auth/refresh и POST /api/v1/auth/logout
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

// TokenResponse — тело ответа при логине и обновлении токенов
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
}

type HealthResponse struct {
	App     string `json:"app"`
	Version string `json:"version"`
	Status  string `json:"status"`
}

func SendJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(APIError{Error: message})
}

func toTokenResponse(pair service.TokenPair) TokenResponse {
	return TokenResponse{
		AccessToken:  pair.AccessToken,
		RefreshToken: pair.RefreshToken,
		TokenType:    "Bearer",
	}
}
