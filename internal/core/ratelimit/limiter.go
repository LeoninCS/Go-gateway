// internal/core/ratelimit/limiter.go
package ratelimit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
)

// Limiter 定义了任何限流算法的通用接口。
type Limiter interface {
	Allow(ctx context.Context, identifier string, settings LimiterSettings) (bool, error)
	Name() string
	Close() error
}

// LimiterSettings 定义了限流器的运行时设置。
type LimiterSettings struct {
	Capacity   int
	RefillRate int
}

// IdentifierFunc 定义了从 HTTP 请求中提取唯一标识符的函数签名。
type IdentifierFunc func(*http.Request) string

// --- 标识符提取函数注册表 ---

// identifierFuncRegistry 是一个存储策略名称到对应函数的映射。
// 这是一个注册表模式，使得添加新的标识符提取策略变得简单。
var identifierFuncRegistry = make(map[string]IdentifierFunc)

// init 函数在包被首次导入时执行，用于注册所有内置的 IdentifierFunc。
func init() {
	RegisterIdentifierFunc("ip", FromIP)
	RegisterIdentifierFunc("path", FromPath)
	RegisterIdentifierFunc("global", Global)
}

// RegisterIdentifierFunc 允许外部包注册自定义的标识符提取函数。
// 这不是必需的，但提供了极高的扩展性。
func RegisterIdentifierFunc(name string, fn IdentifierFunc) {
	if _, exists := identifierFuncRegistry[name]; exists {
		// 在实际项目中，您可能希望这里 panic 或记录一个严重警告
		// 因为重复注册可能是一个bug
	}
	identifierFuncRegistry[strings.ToLower(name)] = fn
}

// GetIdentifierFunc 是一个工厂函数，根据策略名称返回对应的函数。
// 插件代码将调用此函数。
func GetIdentifierFunc(name string) (IdentifierFunc, error) {
	fn, exists := identifierFuncRegistry[strings.ToLower(name)]
	if !exists {
		return nil, fmt.Errorf("不支持的限流标识符策略: '%s'", name)
	}
	return fn, nil
}

// --- 内置的 IdentifierFunc 实现 ---

// FromIP 从请求中提取客户端 IP 地址作为标识符。
// 它会优先检查 X-Forwarded-For 和 X-Real-IP 头。
func FromIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := splitCommas(xff)
		if len(parts) > 0 {
			return parts[0]
		}
	}
	if xrealip := r.Header.Get("X-Real-IP"); xrealip != "" {
		return xrealip
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// FromPath 使用请求的 URL 路径作为标识符。
func FromPath(r *http.Request) string {
	return r.URL.Path
}

// Global 提供一个固定的全局标识符，用于对整个端点进行统一限流。
func Global(r *http.Request) string {
	return "global_fixed_limit"
}

// --- 辅助函数 ---

func splitCommas(s string) []string {
	var parts []string
	for _, part := range strings.Split(s, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			parts = append(parts, trimmed)
		}
	}
	return parts
}

// NoOpLimiter 是一个哑限流器，总是允许请求。
type NoOpLimiter struct{}

func (n *NoOpLimiter) Allow(ctx context.Context, identifier string, settings LimiterSettings) (bool, error) {
	return true, nil
}
func (n *NoOpLimiter) Name() string { return "NoOpLimiter" }
func (n *NoOpLimiter) Close() error { return nil }
