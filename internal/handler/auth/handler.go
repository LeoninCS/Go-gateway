package auth

import (
	"encoding/json"
	"net/http"

	"gateway.example/go-gateway/internal/service/auth"
)

type AuthHandler struct {
	authService *auth.AuthService
}

func NewAuthHandler(service *auth.AuthService) *AuthHandler {
	return &AuthHandler{authService: service}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	token, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}
