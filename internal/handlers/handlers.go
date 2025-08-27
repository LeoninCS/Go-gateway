// File: internal/gateway/handlers.go (或者 internal/handlers/auth_handler.go)
package handlers // 如果移动了文件，这里的包名应改为 handlers

import (
	"encoding/json"
	"net/http"

	// --- 1. 导入 service 包 ---
	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/service"
)

// Handlers 结构体现在持有 AuthService，这是它的核心依赖。
type Handlers struct {
	Config      *config.Config
	authService *service.AuthService // <-- 2. 新增字段，持有业务服务
}

// NewHandlers 构造函数现在需要接收 AuthService。
// --- 3. 修改构造函数签名 ---
func NewHandlers(cfg *config.Config, authService *service.AuthService) *Handlers {
	return &Handlers{
		Config:      cfg,
		authService: authService, // <-- 4. 存储传入的 service 实例
	}
}

// ===== 新增 Handler =====

// Register 处理用户注册请求。
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	// 定义一个匿名结构体来解析请求体中的 JSON 数据
	var requestData struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	// 解码请求体
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 检查用户名和密码是否为空
	if requestData.Username == "" || requestData.Password == "" {
		http.Error(w, `{"error": "Username and password are required"}`, http.StatusBadRequest)
		return
	}

	// 调用业务逻辑层 (AuthService) 来处理注册
	user, err := h.authService.Register(requestData.Username, requestData.Password)
	if err != nil {
		// 根据业务层返回的错误，给出更友好的 HTTP 响应
		// 例如，如果 service.ErrUserExists 是一个已知的错误变量
		// if errors.Is(err, service.ErrUserExists) {
		// 	http.Error(w, `{"error": "User already exists"}`, http.StatusConflict) // 409 Conflict
		//     return
		// }
		http.Error(w, `{"error": "`+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	// 准备成功的响应数据
	response := map[string]interface{}{
		"message": "User registered successfully",
		"user_id": user.ID,
	}

	// 设置响应头为 JSON
	w.Header().Set("Content-Type", "application/json")
	// 设置状态码为 201 Created
	w.WriteHeader(http.StatusCreated)
	// 将响应数据编码为 JSON 并发送
	json.NewEncoder(w).Encode(response)
}

// ===== 重构 Handler =====

// Login 处理登录请求，现在完全依赖 AuthService。
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	// 定义匿名结构体解析请求
	var requestData struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	// 解码请求体
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// --- 5. 核心改变：调用 AuthService 的 Login 方法 ---
	// 所有关于数据库查询、密码比对、Token 生成的复杂逻辑都已被封装
	tokenString, err := h.authService.Login(requestData.Username, requestData.Password)
	if err != nil {
		// 如果登录失败 (用户名或密码错误)
		http.Error(w, `{"error": "Invalid username or password"}`, http.StatusUnauthorized) // 401 Unauthorized
		return
	}

	// 登录成功，返回 Token
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token": tokenString,
	})
}

// Healthz 保持不变，它没有外部依赖
func (h *Handlers) Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
