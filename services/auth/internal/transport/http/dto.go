package http

import (
	"encoding/json"
	"net/http"
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

type LoginResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
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

func toLoginResponse(token string) LoginResponse {
	return LoginResponse{
		AccessToken: token,
		TokenType:   "Bearer",
	}
}
