package auth

import (
	"context"
	"net/http"
	"strings"

	// 确保导入你的 config 包
	"gateway.example/go-gateway/internal/config"
)

// claimsKey 是一个私有类型，用于在 context 中创建唯一的键
type claimsKey string

const key claimsKey = "userClaims"

// Middleware 是一个工厂函数，它接收配置并返回一个 http 中间件
func Middleware(cfg *config.JWTConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header is required", http.StatusUnauthorized)
				return
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, "Authorization header format must be Bearer {token}", http.StatusUnauthorized)
				return
			}

			tokenString := parts[1]
			claims, err := ValidateToken(tokenString, []byte(cfg.SecretKey))
			if err != nil {
				http.Error(w, "Invalid token: "+err.Error(), http.StatusUnauthorized)
				return
			}

			// 将 claims 存入 context，供后续的 handler 使用
			ctx := context.WithValue(r.Context(), key, claims)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// GetClaimsFromContext 从请求的 context 中安全地提取 claims
func GetClaimsFromContext(ctx context.Context) (*MyClaims, bool) {
	claims, ok := ctx.Value(key).(*MyClaims)
	return claims, ok
}
