// File: internal/loadbalancer/loadbalancer.go
package loadbalancer

import (
	"net/http"
	"sync/atomic"
)

// LoadBalancer 是一个实现了 http.Handler 的负载均衡器
type LoadBalancer struct {
	backends []*Backend
	next     uint32 // 用于轮询的原子计数器
}

// NewLoadBalancer 创建一个新的负载均衡器实例
func NewLoadBalancer(backends []*Backend) *LoadBalancer {
	return &LoadBalancer{
		backends: backends,
	}
}

// getNextAvailablePeer 使用轮询策略选择一个健康的后端
func (lb *LoadBalancer) getNextAvailablePeer() *Backend {
	// 遍历所有后端 len(lb.backends) 次，以确保我们检查了每一个
	// 这是一个健壮的循环，即使有多个后端同时宕机也能找到一个可用的
	numBackends := uint32(len(lb.backends))
	for i := uint32(0); i < numBackends; i++ {
		// 原子地增加计数器并取模，得到下一个要检查的索引
		nextIdx := atomic.AddUint32(&lb.next, 1) % numBackends

		// 检查该后端的健康状况
		if lb.backends[nextIdx].IsAlive() {
			// 如果健康，就返回它
			return lb.backends[nextIdx]
		}
	}
	// 如果循环结束都没有找到健康的后端
	return nil
}

// ServeHTTP 是处理请求的入口，它实现了 http.Handler 接口
func (lb *LoadBalancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	peer := lb.getNextAvailablePeer()
	if peer != nil {
		// 找到了一个健康的后端，将请求转发给它
		peer.ReverseProxy.ServeHTTP(w, r)
		return
	}

	// 所有后端都不可用，返回 503 Service Unavailable 错误
	http.Error(w, "Service not available", http.StatusServiceUnavailable)
}
