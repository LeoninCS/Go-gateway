// file: internal/core/limiter/manager.go
package limiter

import (
	"context"
	"fmt"
	"sync"

	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/pkg/logger"
)

// Manager 负责管理所有已定义的限流器实例。
type Manager struct {
	limiters map[string]Limiter // 存储规则名到 Limiter 实例的映射
	mu       sync.RWMutex
}

// NewManager 根据配置创建并初始化限流管理器。
func NewManager(cfg config.RateLimitingConfig, log logger.Logger) *Manager {
	m := &Manager{
		limiters: make(map[string]Limiter),
	}

	for _, rule := range cfg.Rules {
		// 确保不会有同名规则
		if _, exists := m.limiters[rule.Name]; exists {
			log.Fatal(context.Background(), "[限流管理器] 致命错误: 发现重复的限流规则名称 '%s'", rule.Name)
		}

		var newLimiter Limiter
		var err error

		switch rule.Type {
		case "memory_token_bucket":
			settings := rule.TokenBucket
			if settings.Capacity <= 0 || settings.RefillRate <= 0 {
				log.Fatal(context.Background(), "[限流管理器] 致命错误: 规则 '%s' 的 capacity 和 refillRate 必须为正数", rule.Name)
			}
			// 使用全局上下文来创建限流器，它的生命周期和网关一样长
			newLimiter = NewMemoryTokenBucket(context.Background(), settings.Capacity, settings.RefillRate, rule.Name)
		default:
			err = fmt.Errorf("不支持的限流器类型 '%s'", rule.Type)
		}

		if err != nil {
			log.Fatal(context.Background(), "[限流管理器] 致命错误: 创建规则 '%s' 失败: %v", rule.Name, err)
		}

		m.limiters[rule.Name] = newLimiter
		log.Info(context.Background(), "[限流管理器] 成功加载规则 '%s' (类型: %s)", rule.Name, rule.Type)
	}

	return m
}

// Allow 是提供给插件调用的核心方法。
// ★ 修改点: 它现在接收 ruleName 和 identifier，完全与 HTTP 请求解耦。
func (m *Manager) Allow(ruleName string, identifier string) (bool, error) {
	m.mu.RLock()
	limiter, ok := m.limiters[ruleName]
	m.mu.RUnlock()

	if !ok {
		// 返回错误，因为插件引用了一个不存在的规则，这是一个配置错误。
		return false, fmt.Errorf("引用的限流规则 '%s' 不存在", ruleName)
	}

	// 使用全局上下文，实际的超时应由上游请求的 context 控制
	return limiter.Allow(context.Background(), identifier), nil
}
