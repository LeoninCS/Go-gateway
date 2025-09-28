// file: internal/service/ratelimit/service.go
package ratelimit

import (
	"context"
	"fmt"
	"sync"

	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/core/limiter"
	"gateway.example/go-gateway/pkg/logger"
)

// Service 定义了限流服务的接口。
// 它解耦合了插件层与具体的限流逻辑实现。
type Service interface {
	CheckLimit(ctx context.Context, ruleName, identifier string) (bool, error)
	Close() error
}

// service 是 Service 接口的具体实现。
type service struct {
	mu sync.RWMutex
	// 只需存储限流器实例即可，规则配置已在实例内部。
	limiters map[string]limiter.Limiter
	// 用于管理所有限流器生命周期的 context。
	ctx    context.Context
	cancel context.CancelFunc
	log    logger.Logger
}

// NewService 创建一个新的限流服务实例。
func NewService(cfg config.RateLimitingConfig, log logger.Logger) (Service, error) {
	// 创建一个可被取消的 context，用于优雅关闭。
	ctx, cancel := context.WithCancel(context.Background())

	s := &service{
		limiters: make(map[string]limiter.Limiter),
		ctx:      ctx,
		cancel:   cancel,
		log:      log,
	}

	log.Info(ctx, "Initializing rate limit service",
		"total_rules", len(cfg.Rules),
		"service", "ratelimit",
		"action", "initialize")

	for _, rule := range cfg.Rules {
		// 复制 rule 变量，防止闭包问题
		currentRule := rule

		var lim limiter.Limiter
		var err error

		switch currentRule.Type {
		case "memory_token_bucket":
			// 使用新的构造函数，并传入 service 的 context。
			lim = limiter.NewMemoryTokenBucket(
				s.ctx, // 传入 context
				currentRule.TokenBucket.Capacity,
				currentRule.TokenBucket.RefillRate,
				currentRule.Name,
			)
		case "", "noop":
			// 引用 core/limiter 包中的 NoOpLimiter。
			lim = &limiter.NoOpLimiter{}
		default:
			err = fmt.Errorf("未知的限流器类型: %s for rule %s", currentRule.Type, currentRule.Name)
		}

		if err != nil {
			// 如果有任何一个限流器创建失败，则立即取消上下文并返回错误。
			log.Error(ctx, "Failed to initialize rate limiter",
				"rule_name", currentRule.Name,
				"limiter_type", currentRule.Type,
				"error", err.Error(),
				"service", "ratelimit",
				"action", "initialization_failed")
			cancel()
			return nil, err
		}

		s.limiters[currentRule.Name] = lim
		log.Info(ctx, "Successfully initialized rate limit rule",
			"rule_name", currentRule.Name,
			"limiter_type", lim.Name(),
			"service", "ratelimit",
			"action", "rule_initialized")
	}

	log.Info(ctx, "Rate limit service initialization completed",
		"active_limiters", len(s.limiters),
		"service", "ratelimit",
		"action", "initialization_completed")

	return s, nil
}

// CheckLimit 实现了 Service 接口。它检查给定的标识符是否被特定规则所允许。
func (s *service) CheckLimit(ctx context.Context, ruleName, identifier string) (bool, error) {
	s.mu.RLock()
	lim, exists := s.limiters[ruleName]
	s.mu.RUnlock()

	if !exists {
		// 这是一个配置错误：插件引用了一个不存在的规则。
		s.log.Error(ctx, "Rate limit rule not found",
			"rule_name", ruleName,
			"identifier", identifier,
			"service", "ratelimit",
			"action", "rule_not_found")
		return false, fmt.Errorf("限流规则 '%s' 未定义", ruleName)
	}

	// 调用新的、简化的 Allow 接口。
	// 注意: 这里传入的 ctx 是来自上游请求的 context，用于处理请求级别的超时。
	// 而限流器内部运行的后台任务使用的是 service 级别的 ctx。
	isAllowed := lim.Allow(ctx, identifier)

	if isAllowed {
		s.log.Debug(ctx, "Rate limit check passed",
			"rule_name", ruleName,
			"identifier", identifier,
			"limiter_type", lim.Name(),
			"service", "ratelimit",
			"action", "check_passed")
	} else {
		s.log.Info(ctx, "Rate limit check failed - request blocked",
			"rule_name", ruleName,
			"identifier", identifier,
			"limiter_type", lim.Name(),
			"service", "ratelimit",
			"action", "check_failed")
	}

	return isAllowed, nil
}

// Close 优雅地关闭所有限流器（例如，停止后台的清理goroutine）。
func (s *service) Close() error {
	ctx := context.Background()

	s.log.Info(ctx, "Starting graceful shutdown of rate limit service",
		"active_limiters", len(s.limiters),
		"service", "ratelimit",
		"action", "shutdown_start")

	// 通过取消 context 来通知所有子 goroutine 停止。
	s.cancel()
	// 通常，这里可以加一个等待组（WaitGroup）来确保所有 goroutine 都已退出，
	// 但对于 MemoryTokenBucket 的简单清理任务，直接 cancel 已经足够。

	s.log.Info(ctx, "Rate limit service shutdown completed",
		"service", "ratelimit",
		"action", "shutdown_completed")

	return nil
}
