// internal/loadbalancer/round_robin.go
package loadbalancer

import (
	"errors"
	"sync"
)

type RoundRobinBalancer struct {
	serviceName string
	instances   []*ServiceInstance
	mutex       sync.RWMutex
	index       int
}

func NewRoundRobinBalancer(serviceName string) *RoundRobinBalancer {
	return &RoundRobinBalancer{
		serviceName: serviceName,
	}
}

func (r *RoundRobinBalancer) RegisterInstance(serviceName string, instance *ServiceInstance) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.instances = append(r.instances, instance)
}

func (r *RoundRobinBalancer) GetNextInstance(serviceName string) (*ServiceInstance, error) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if len(r.instances) == 0 {
		return nil, errors.New("no instances available")
	}

	// 过滤出健康的实例
	healthyInstances := make([]*ServiceInstance, 0)
	for _, instance := range r.instances {
		if instance.Alive {
			healthyInstances = append(healthyInstances, instance)
		}
	}

	if len(healthyInstances) == 0 {
		return nil, errors.New("no healthy instances available")
	}

	// 轮询选择下一个实例
	instance := healthyInstances[r.index%len(healthyInstances)]
	r.index++

	return instance, nil
}

func (r *RoundRobinBalancer) GetAllInstances(serviceName string) []*ServiceInstance {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// 返回健康的实例
	healthyInstances := make([]*ServiceInstance, 0)
	for _, instance := range r.instances {
		if instance.Alive {
			healthyInstances = append(healthyInstances, instance)
		}
	}
	return healthyInstances
}
