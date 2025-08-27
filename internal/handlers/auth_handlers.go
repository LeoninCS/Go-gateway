// File: internal/handlers/auth_handler.go
package handlers

import (
	"encoding/json"
	"net/http"

	"gateway.example/go-gateway/internal/service" // 依赖 service 包
)

// AuthHandler 结构体持有认证相关的业务逻辑依赖
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler 是 AuthHandler 的构造函数
func NewAuthHandler(authSvc *service.AuthService) *AuthHandler {
	return &AuthHandler{
		authService: authSvc,
	}
}

// Register 处理用户注册请求
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.authService.Register(req.Username, req.Password)
	if err != nil {
		// 这里可以根据错误类型返回不同的状态码
		// 例如: if errors.Is(err, dao.ErrUserExists) { ... }
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	resp := map[string]interface{}{
		"id":        user.ID,
		"username":  user.Username,
		"createdAt": user.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

// Login 处理用户登录请求
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	token, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	resp := map[string]string{"token": token}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// Healthz 是一个通用的健康检查处理器
func Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
