// file: internal/auth/middleware.go
package auth

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"gateway.example/go-gateway/internal/config" // 确保这是你 go.mod 中的模块名
	"github.com/golang-jwt/jwt/v5"
)

// claimsKey 是一个自定义类型，用于在请求上下文中存储和检索用户信息
type claimsKeyType struct{}

var claimsKey = claimsKeyType{}

// Middleware 是一个中间件工厂函数
// 它接收 JWT 配置，并返回一个标准的 http 中间件处理器
func Middleware(jwtConfig *config.JWTConfig) func(http.Handler) http.Handler {
	// 返回真正的中间件函数
	return func(next http.Handler) http.Handler {
		// 返回最终的 http.HandlerFunc
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. 从请求头中获取 Authorization 字段
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				log.Println("Authorization header is missing")
				http.Error(w, "Unauthorized: Missing Authorization Header", http.StatusUnauthorized)
				return
			}

			// 2. 验证 Header 格式是否为 "Bearer <token>"
			headerParts := strings.Split(authHeader, " ")
			if len(headerParts) != 2 || strings.ToLower(headerParts[0]) != "bearer" {
				log.Printf("Authorization header format is invalid: %s", authHeader)
				http.Error(w, "Unauthorized: Invalid Authorization Header Format", http.StatusUnauthorized)
				return
			}

			tokenString := headerParts[1]

			// 3. 解析和验证 token
			claims := &jwt.RegisteredClaims{}
			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				// 确保签名算法是我们期望的 HMAC
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, errors.New("unexpected signing method")
				}
				return []byte(jwtConfig.SecretKey), nil
			})

			if err != nil {
				log.Printf("Token validation failed: %v", err)
				if errors.Is(err, jwt.ErrTokenExpired) {
					http.Error(w, "Unauthorized: Token is expired", http.StatusUnauthorized)
				} else {
					http.Error(w, "Unauthorized: Invalid Token", http.StatusUnauthorized)
				}
				return
			}

			if !token.Valid {
				log.Println("Token is not valid")
				http.Error(w, "Unauthorized: Invalid Token", http.StatusUnauthorized)
				return
			}

			// (可选但推荐) 将解析出的用户信息（比如用户ID）存入请求的 context 中，
			// 以便下游服务可以获取。
			// 网关本身可能用不到，但这是一个标准的微服务实践。
			ctx := context.WithValue(r.Context(), claimsKey, claims)
			r = r.WithContext(ctx)

			// 4. 如果一切正常，将请求传递给下一个处理器
			log.Printf("JWT validation successful for subject: %s", claims.Subject)
			next.ServeHTTP(w, r)
		})
	}
}
