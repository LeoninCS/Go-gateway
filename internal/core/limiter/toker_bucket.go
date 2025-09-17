// file: internal/core/limiter/token_bucket.go
package limiter

import (
	"context"
	"sync"
	"time"
)

// bucket 定义了每个标识符的状态
type bucket struct {
	tokens    int
	lastCheck time.Time
}

// MemoryTokenBucket 是一个基于内存的令牌桶限流器实现。
type MemoryTokenBucket struct {
	name       string
	capacity   int
	refillRate int
	// ★ 修改点: 现在的桶是 string -> *bucket，因为 identifier 是 string
	buckets  map[string]*bucket
	mu       sync.Mutex
	stopChan chan struct{}
}

// NewMemoryTokenBucket 创建一个新的内存令牌桶。
func NewMemoryTokenBucket(ctx context.Context, capacity, refillRate int, name string) *MemoryTokenBucket {
	b := &MemoryTokenBucket{
		name:       name,
		capacity:   capacity,
		refillRate: refillRate,
		buckets:    make(map[string]*bucket),
		stopChan:   make(chan struct{}),
	}
	// ... (清理 goroutine 的逻辑保持不变) ...
	return b
}

// ★ 修改点: Allow 方法签名更新，逻辑也相应简化
func (b *MemoryTokenBucket) Allow(ctx context.Context, identifier string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 查找或创建标识符对应的桶
	currentBucket, ok := b.buckets[identifier]
	if !ok {
		// 首次访问，创建一个满的桶
		currentBucket = &bucket{
			tokens:    b.capacity,
			lastCheck: time.Now(),
		}
		b.buckets[identifier] = currentBucket
	}

	// 补充令牌
	now := time.Now()
	elapsed := now.Sub(currentBucket.lastCheck)
	// 注意: elapsed.Seconds() 返回的是 float64
	refillCount := int(elapsed.Seconds() * float64(b.refillRate))
	if refillCount > 0 {
		currentBucket.tokens += refillCount
		currentBucket.lastCheck = now
	}
	if currentBucket.tokens > b.capacity {
		currentBucket.tokens = b.capacity
	}

	// 检查并消耗令牌
	if currentBucket.tokens > 0 {
		currentBucket.tokens--
		return true
	}

	return false
}

// Name 返回限流器的名称
func (b *MemoryTokenBucket) Name() string {
	return b.name
}
