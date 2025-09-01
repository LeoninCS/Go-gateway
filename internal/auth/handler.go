package auth

import (
	"encoding/json"
	"net/http"
)

type AuthHandler struct {
	authService *AuthService
}

func NewAuthHandler(authSvc *AuthService) *AuthHandler {
	return &AuthHandler{authService: authSvc}
}

// Register 处理用户注册请求
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Phone    string `json:"phone"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
		return
	}

	// 调用 Service 执行核心业务逻辑
	user, err := h.authService.Register(req.Username, req.Password, req.Phone)
	if err != nil {
		// 根据错误类型返回不同的 HTTP 状态码
		if err == ErrUserExists {
			http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusConflict)
		} else {
			http.Error(w, `{"error": "Internal server error"}`, http.StatusInternalServerError)
		}
		return
	}

	// 构建响应，排除敏感信息
	response := map[string]interface{}{
		"id":        user.ID,
		"username":  user.Username,
		"phone":     user.Phone,
		"createdAt": user.CreatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// Login 处理用户登录请求
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
		return
	}

	token, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		if err == ErrInvalidCredentials {
			http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusUnauthorized)
		} else {
			http.Error(w, `{"error": "Internal server error"}`, http.StatusInternalServerError)
		}
		return
	}

	response := map[string]string{"token": token}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
		return
	}
	err := h.authService.ChangePassword(req.Username, req.OldPassword, req.NewPassword)
	if err != nil {
		if err == ErrInvalidCredentials {
			http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusUnauthorized)
		} else {
			http.Error(w, `{"error": "Internal server error"}`, http.StatusInternalServerError)
		}
		return
	}

	response := map[string]string{"message": "Password changed successfully"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *AuthHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username         string `json:"username"`
		Phone            string `json:"phone"`
		VerificationCode string `json:"verification_code"`
		NewPassword      string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
		return
	}
	err := h.authService.ResetPassword(req.Username, req.Phone, req.VerificationCode, req.NewPassword)
	if err != nil {
		if err == ErrInvalidVerificationCode || err == ErrUserNotFound || err == ErrPhoneNotMatch {
			http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusBadRequest)
		} else {
			http.Error(w, `{"error": "Internal server error"}`, http.StatusInternalServerError)
		}
		return
	}

	response := map[string]string{"message": "Password reset successfully"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *AuthHandler) Unregister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
		return
	}
	err := h.authService.Unregister(req.Username, req.Password)
	if err != nil {
		if err == ErrInvalidCredentials || err == ErrUserNotFound {
			http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusBadRequest)
		} else {
			http.Error(w, `{"error": "Internal server error"}`, http.StatusInternalServerError)
		}
		return
	}

	response := map[string]string{"message": "User unregistered successfully"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
		return
	}
	err := h.authService.Logout(req.Token)
	if err != nil {
		http.Error(w, `{"error": "Internal server error"}`, http.StatusInternalServerError)
		return
	}

	response := map[string]string{"message": "Logged out successfully"}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *AuthHandler) SendVerificationCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Phone    string `json:"phone"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
		return
	}
	message, err := h.authService.SendVerificationCode(req.Username, req.Phone)
	if err != nil {
		http.Error(w, `{"error": "Internal server error"}`, http.StatusInternalServerError)
		return
	}

	response := map[string]string{"message": message}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
