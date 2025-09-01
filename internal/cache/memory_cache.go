package cache

import (
	"fmt"
	"sync"
	"time"
)

// memoryCacheItem 存储在缓存中的项目，包含值和过期时间
type memoryCacheItem struct {
	value      string
	expiration time.Time
}

// MemoryCache 内存缓存实现
type MemoryCache struct {
	items map[string]memoryCacheItem
	mu    sync.RWMutex // 读写锁，保证并发安全
}

// NewMemoryCache 创建一个新的内存缓存实例
func NewMemoryCache() *MemoryCache {
	return &MemoryCache{
		items: make(map[string]memoryCacheItem),
	}
}

// Set 设置键值对
func (m *MemoryCache) Set(key string, value interface{}, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 将值转换为字符串
	var valueStr string
	switch v := value.(type) {
	case string:
		valueStr = v
	case []byte:
		valueStr = string(v)
	default:
		// 如果不是字符串或字节切片，可以尝试其他方式转换，这里简单处理
		valueStr = fmt.Sprintf("%v", v)
	}

	m.items[key] = memoryCacheItem{
		value:      valueStr,
		expiration: time.Now().Add(expiration),
	}

	// 启动一个 goroutine 在过期后清理（可选，也可以惰性删除）
	if expiration > 0 {
		time.AfterFunc(expiration, func() {
			m.Delete(key)
		})
	}

	return nil
}

// Get 获取键的值
func (m *MemoryCache) Get(key string) (string, error) {
	m.mu.RLock() // 读锁
	defer m.mu.RUnlock()

	item, exists := m.items[key]
	if !exists {
		return "", ErrKeyNotFound
	}

	// 检查是否过期
	if !item.expiration.IsZero() && time.Now().After(item.expiration) {
		// 虽然过期，但这里先返回错误，删除操作由惰性清理或定期清理完成
		return "", ErrKeyNotFound
	}

	return item.value, nil
}

// Delete 删除键
func (m *MemoryCache) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.items, key)
	return nil
}

// Exists 检查键是否存在且未过期
func (m *MemoryCache) Exists(key string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.items[key]
	if !exists {
		return false, nil
	}

	// 如果设置了过期时间且已过期，则认为不存在
	if !item.expiration.IsZero() && time.Now().After(item.expiration) {
		return false, nil
	}

	return true, nil
}

// 可选：添加一个后台goroutine定期清理过期键
func (m *MemoryCache) StartCleanup(interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		for range ticker.C {
			m.mu.Lock()
			for key, item := range m.items {
				if !item.expiration.IsZero() && time.Now().After(item.expiration) {
					delete(m.items, key)
				}
			}
			m.mu.Unlock()
		}
	}()
}
