// internal/service/ratelimit/service.go
package ratelimit

import (
	"context"
	"fmt"
	"sync"

	"gateway.example/go-gateway/internal/config"

	// 导入新的 core 包
	"gateway.example/go-gateway/internal/core/ratelimit"
)

// Service 定义了限流服务的接口。
type Service interface {
	CheckLimit(ctx context.Context, ruleName, identifier string) (bool, error)
	Close() error
}

// service 是 Service 接口的具体实现。
type service struct {
	mu           sync.RWMutex
	limiters     map[string]ratelimit.Limiter      // Key: 规则名, Value: 限流器实例
	limiterRules map[string]config.RateLimiterRule // Key: 规则名, Value: 规则配置
}

// NewService 创建一个新的限流服务实例。
func NewService(cfg *config.Config) (Service, error) {
	s := &service{
		limiters:     make(map[string]ratelimit.Limiter),
		limiterRules: make(map[string]config.RateLimiterRule),
	}
	for _, rule := range cfg.RateLimiting.Rules {
		s.limiterRules[rule.Name] = rule
		var lim ratelimit.Limiter
		switch rule.Type {
		case "memory_token_bucket":
			lim = ratelimit.NewMemoryTokenBucketLimiter()
		case "", "noop":
			lim = &ratelimit.NoOpLimiter{}
		default:
			return nil, fmt.Errorf("未知的限流器类型: %s for rule %s", rule.Type, rule.Name)
		}
		s.limiters[rule.Name] = lim
		fmt.Printf("成功初始化限流规则: %s, 类型: %s\n", rule.Name, lim.Name())
	}
	return s, nil
}

// CheckLimit 实现了 Service 接口。
func (s *service) CheckLimit(ctx context.Context, ruleName, identifier string) (bool, error) {
	s.mu.RLock()
	lim, limExists := s.limiters[ruleName]
	rule, ruleExists := s.limiterRules[ruleName]
	s.mu.RUnlock()

	if !limExists || !ruleExists {
		return false, fmt.Errorf("限流规则 '%s' 未定义", ruleName)
	}

	settings := ratelimit.LimiterSettings{
		Capacity:   rule.TokenBucket.Capacity,
		RefillRate: rule.TokenBucket.RefillRate,
	}
	return lim.Allow(ctx, identifier, settings)
}

// Close 优雅地关闭所有限流器。
func (s *service) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var lastErr error
	for name, lim := range s.limiters {
		if err := lim.Close(); err != nil {
			lastErr = fmt.Errorf("关闭限流器 '%s' 失败: %w", name, err)
		}
	}
	return lastErr
}
