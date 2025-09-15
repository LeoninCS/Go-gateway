// internal/core/ratelimit/memory_limiter.go
package ratelimit

import (
	"context"
	"fmt"
	"sync"

	"golang.org/x/time/rate"
)

// MemoryTokenBucketLimiter 使用内存 map 来存储每个标识符的令牌桶。
type MemoryTokenBucketLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rate.Limiter
}

// NewMemoryTokenBucketLimiter 创建一个新的内存令牌桶限流器实例。
func NewMemoryTokenBucketLimiter() Limiter { // 返回接口类型
	return &MemoryTokenBucketLimiter{
		buckets: make(map[string]*rate.Limiter),
	}
}

// Allow 检查给定标识符的请求是否被允许。
func (m *MemoryTokenBucketLimiter) Allow(_ context.Context, identifier string, settings LimiterSettings) (bool, error) {
	if identifier == "" {
		return false, fmt.Errorf("限流标识符不能为空")
	}
	m.mu.Lock()
	lim, exists := m.buckets[identifier]
	if !exists {
		// 如果桶不存在，则根据传入的 settings 创建一个新的令牌桶
		lim = rate.NewLimiter(rate.Limit(settings.RefillRate), settings.Capacity)
		m.buckets[identifier] = lim
	}
	m.mu.Unlock()

	return lim.Allow(), nil
}

// Name 返回限流器的名称。
func (m *MemoryTokenBucketLimiter) Name() string {
	return "MemoryTokenBucketLimiter"
}

// Close 对于简单内存实现，无需操作。
func (m *MemoryTokenBucketLimiter) Close() error {
	return nil
}
