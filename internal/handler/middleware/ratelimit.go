// internal/handler/middleware/ratelimit.go
package middleware

import (
	"log"
	"net/http"

	"gateway.example/go-gateway/internal/core/ratelimit"
	// 使用别名 svcr (service-rate-limit) 避免与 core.ratelimit 包名冲突
	svcr "gateway.example/go-gateway/internal/service/ratelimit"
)

// RateLimit 是创建限流中间件的工厂函数。
func NewRateLimiter(
	svc svcr.Service,
	identifierFunc ratelimit.IdentifierFunc,
	ruleName string,
) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identifier := identifierFunc(r)
			if identifier == "" {
				log.Printf("[WARN] RateLimit: 无法从请求 '%s' 中为规则 '%s' 提取标识符", r.URL.Path, ruleName)
				next.ServeHTTP(w, r) // 无法识别则放行，或根据策略拒绝
				return
			}

			allowed, err := svc.CheckLimit(r.Context(), ruleName, identifier)
			if err != nil {
				log.Printf("[ERROR] RateLimit: 检查限流时出错: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			if !allowed {
				log.Printf("[INFO] RateLimit: 请求被拒绝. 规则: '%s', 标识符: '%s'", ruleName, identifier)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				w.Write([]byte(`{"error": "Too Many Requests"}`))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
