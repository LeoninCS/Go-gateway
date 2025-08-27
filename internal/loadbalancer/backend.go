// File: internal/loadbalancer/backend.go
package loadbalancer

import (
	"net/http/httputil"
	"net/url"
	"sync"
)

// Backend 代表一个后端服务器及其元数据
type Backend struct {
	URL          *url.URL
	ReverseProxy *httputil.ReverseProxy

	// 使用 RWMutex 以允许并发地读写存活状态
	mu    sync.RWMutex
	Alive bool
}

// SetAlive 原子地设置后端的存活状态
func (b *Backend) SetAlive(alive bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.Alive = alive
}

// IsAlive 原子地检查后端是否存活
func (b *Backend) IsAlive() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Alive
}
