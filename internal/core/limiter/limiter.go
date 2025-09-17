// file: internal/core/limiter/limiter.go
package limiter

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

// Limiter 是所有限流算法必须实现的接口。
// ★ 修改点: Allow 方法现在只接收 identifier，不再需要 settings 和 request。
//
//	因为限流器在创建时就已经知道了自己的配置。
type Limiter interface {
	Allow(ctx context.Context, identifier string) bool
	Name() string
}

// IdentifierFunc 是一个函数类型，用于从 HTTP 请求中提取唯一的标识符。
type IdentifierFunc func(r *http.Request) string

type NoOpLimiter struct{}

// GetIdentifierFunc 根据策略名称返回对应的标识符提取函数。
// 这个函数现在是插件层使用的工具，但定义在核心层是合适的。
func GetIdentifierFunc(strategy string) (IdentifierFunc, error) {
	switch strings.ToLower(strategy) {
	case "ip":
		return func(r *http.Request) string {
			// 优先从 X-Forwarded-For 获取，适配代理场景
			ip := r.Header.Get("X-Forwarded-For")
			if ip == "" {
				// X-Real-IP 是另一个常见Header
				ip = r.Header.Get("X-Real-Ip")
			}
			if ip == "" {
				// 最后回退到 RemoteAddr
				// 注意：RemoteAddr 可能包含端口，需要处理
				ip = strings.Split(r.RemoteAddr, ":")[0]
			}
			return ip
		}, nil
	case "path":
		return func(r *http.Request) string {
			return r.URL.Path
		}, nil
	case "global":
		return func(r *http.Request) string {
			return "global"
		}, nil
	default:
		return nil, fmt.Errorf("不支持的限流策略: '%s'", strategy)
	}
}

// Allow 总是返回 true。
func (l *NoOpLimiter) Allow(ctx context.Context, identifier string) bool {
	return true
}

// Name 返回此限流器的名称。
func (l *NoOpLimiter) Name() string {
	return "NoOpLimiter"
}
