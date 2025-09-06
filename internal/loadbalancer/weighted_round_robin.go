// internal/loadbalancer/weighted_round_robin.go
package loadbalancer

import (
	"errors"
	"sync"
)

type WeightedRoundRobinBalancer struct {
	serviceName string
	instances   []*ServiceInstance
	mutex       sync.RWMutex
	current     int
}

func NewWeightedRoundRobinBalancer(serviceName string) *WeightedRoundRobinBalancer {
	return &WeightedRoundRobinBalancer{
		serviceName: serviceName,
	}
}

func (w *WeightedRoundRobinBalancer) RegisterInstance(serviceName string, instance *ServiceInstance) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	w.instances = append(w.instances, instance)
}

func (w *WeightedRoundRobinBalancer) GetNextInstance(serviceName string) (*ServiceInstance, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if len(w.instances) == 0 {
		return nil, errors.New("no instances available")
	}

	// 过滤出健康的实例
	healthyInstances := make([]*ServiceInstance, 0)
	totalWeight := 0
	for _, instance := range w.instances {
		if instance.Alive {
			healthyInstances = append(healthyInstances, instance)
			totalWeight += instance.Weight
		}
	}

	if len(healthyInstances) == 0 {
		return nil, errors.New("no healthy instances available")
	}

	// 如果总权重为0，则回退到简单轮询
	if totalWeight == 0 {
		instance := healthyInstances[w.current%len(healthyInstances)]
		w.current++
		return instance, nil
	}

	// 加权轮询算法
	target := w.current % totalWeight
	selectedInstance := healthyInstances[0]
	cumulativeWeight := 0

	for _, instance := range healthyInstances {
		cumulativeWeight += instance.Weight
		if target < cumulativeWeight {
			selectedInstance = instance
			break
		}
	}

	w.current++
	return selectedInstance, nil
}

func (w *WeightedRoundRobinBalancer) GetAllInstances(serviceName string) []*ServiceInstance {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	// 返回健康的实例
	healthyInstances := make([]*ServiceInstance, 0)
	for _, instance := range w.instances {
		if instance.Alive {
			healthyInstances = append(healthyInstances, instance)
		}
	}
	return healthyInstances
}
