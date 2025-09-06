// internal/loadbalancer/least_connections.go
package loadbalancer

import (
	"errors"
	"sync"
)

type LeastConnectionsBalancer struct {
	serviceName string
	instances   []*ServiceInstance
	mutex       sync.RWMutex
}

func NewLeastConnectionsBalancer(serviceName string) *LeastConnectionsBalancer {
	return &LeastConnectionsBalancer{
		serviceName: serviceName,
	}
}

func (l *LeastConnectionsBalancer) RegisterInstance(serviceName string, instance *ServiceInstance) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.instances = append(l.instances, instance)
}

func (l *LeastConnectionsBalancer) GetNextInstance(serviceName string) (*ServiceInstance, error) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	if len(l.instances) == 0 {
		return nil, errors.New("no instances available")
	}

	// 过滤出健康的实例
	healthyInstances := make([]*ServiceInstance, 0)
	for _, instance := range l.instances {
		if instance.Alive {
			healthyInstances = append(healthyInstances, instance)
		}
	}

	if len(healthyInstances) == 0 {
		return nil, errors.New("no healthy instances available")
	}

	// 找到连接数最少的实例
	minConnections := healthyInstances[0].Connections
	selectedInstance := healthyInstances[0]

	for _, instance := range healthyInstances {
		if instance.Connections < minConnections {
			minConnections = instance.Connections
			selectedInstance = instance
		}
	}

	// 增加连接计数
	selectedInstance.Connections++
	return selectedInstance, nil
}

func (l *LeastConnectionsBalancer) GetAllInstances(serviceName string) []*ServiceInstance {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	// 返回健康的实例
	healthyInstances := make([]*ServiceInstance, 0)
	for _, instance := range l.instances {
		if instance.Alive {
			healthyInstances = append(healthyInstances, instance)
		}
	}
	return healthyInstances
}

// ReleaseConnection 释放连接计数（在请求完成后调用）
func (l *LeastConnectionsBalancer) ReleaseConnection(serviceName, instanceURL string) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	for _, instance := range l.instances {
		if instance.URL == instanceURL {
			instance.Connections--
			if instance.Connections < 0 {
				instance.Connections = 0
			}
			break
		}
	}
}
