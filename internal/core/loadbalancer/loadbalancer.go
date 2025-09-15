// internal/loadbalancer/loadbalancer.go
package loadbalancer

import (
	"sync"
)

// ServiceInstance 表示一个服务实例
type ServiceInstance struct {
	URL         string
	Weight      int
	Alive       bool
	Connections int // 用于最少连接数算法
}

// LoadBalancer 负载均衡器接口
type LoadBalancer interface {
	GetNextInstance(serviceName string) (*ServiceInstance, error)
	RegisterInstance(serviceName string, instance *ServiceInstance)
	GetAllInstances(serviceName string) []*ServiceInstance
}

// LoadBalancerFactory 负载均衡器工厂
type LoadBalancerFactory struct {
	balancers map[string]LoadBalancer
	mutex     sync.RWMutex
}

func NewLoadBalancerFactory() *LoadBalancerFactory {
	return &LoadBalancerFactory{
		balancers: make(map[string]LoadBalancer),
	}
}

// GetOrCreateLoadBalancer 获取或创建负载均衡器
func (f *LoadBalancerFactory) GetOrCreateLoadBalancer(serviceName, algorithm string) LoadBalancer {
	f.mutex.RLock()
	if lb, exists := f.balancers[serviceName]; exists {
		f.mutex.RUnlock()
		return lb
	}
	f.mutex.RUnlock()

	f.mutex.Lock()
	defer f.mutex.Unlock()

	// 再次检查，防止其他协程已经创建了
	if lb, exists := f.balancers[serviceName]; exists {
		return lb
	}

	// 创建新的负载均衡器
	var lb LoadBalancer
	switch algorithm {
	case "round_robin":
		lb = NewRoundRobinBalancer(serviceName)
	case "weighted_round_robin":
		lb = NewWeightedRoundRobinBalancer(serviceName)
	case "least_connections":
		lb = NewLeastConnectionsBalancer(serviceName)
	default:
		lb = NewRoundRobinBalancer(serviceName)
	}

	f.balancers[serviceName] = lb
	return lb
}
