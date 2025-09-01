package cache

import (
	"errors"
	"time"
)

// 定义错误类型
var (
	ErrKeyNotFound = errors.New("key not found")
)

// Cache 定义缓存接口
type Cache interface {
	// Set 设置一个键值对，并指定过期时间
	Set(key string, value interface{}, expiration time.Duration) error
	// Get 获取一个键的值。如果键不存在，返回 ErrKeyNotFound
	Get(key string) (string, error)
	// Delete 删除一个键
	Delete(key string) error
	// Exists 检查键是否存在
	Exists(key string) (bool, error)
	// 可以根据需要扩展其他方法，如 Incr, Decr 等
}
