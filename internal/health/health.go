package health

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

type HealthChecker struct {
	Client      *http.Client               //
	Checks      map[string]map[string]bool // 服务名称 -> 实例URL -> 健康状态
	CheckMutex  sync.RWMutex               //
	StopChan    chan struct{}              // 停止信号通道
	Interval    time.Duration              // 检查间隔
	ServiceURLs map[string][]string        // 服务对应的实例URL列表
}

func NewHealthChecker(interval time.Duration) *HealthChecker {
	return &HealthChecker{
		Client: &http.Client{
			Timeout: 5 * time.Second,
		},
		Checks:      make(map[string]map[string]bool),
		StopChan:    make(chan struct{}),
		Interval:    interval,
		ServiceURLs: make(map[string][]string),
	}
}

// RegisterService 注册服务及其实例
func (h *HealthChecker) RegisterService(serviceName string, instances []string, healthPath string) {
	h.CheckMutex.Lock()
	defer h.CheckMutex.Unlock()

	h.ServiceURLs[serviceName] = instances

	// 初始化状态检查
	if _, exists := h.Checks[serviceName]; !exists {
		h.Checks[serviceName] = make(map[string]bool)
	}

	for _, url := range instances {
		if _, exists := h.Checks[serviceName][url]; !exists {
			h.Checks[serviceName][url] = true // 默认健康，同步修改引用
		}
	}
}

// Start 开始健康检查
func (h *HealthChecker) Start() {
	ticker := time.NewTicker(h.Interval)

	for {
		select {
		case <-ticker.C:
			h.runHealthChecks()
		case <-h.StopChan:
			ticker.Stop()
			return
		}
	}
}

// Stop 停止健康检查（原方法名补充完整语义）
func (h *HealthChecker) Stop() {
	close(h.StopChan)
}

// runHealthChecks 执行健康检查
func (h *HealthChecker) runHealthChecks() {
	// 遍历服务对应的实例列表，同步修改引用
	for serviceName, instances := range h.ServiceURLs {
		for _, url := range instances {
			healthy := h.checkInstanceHealth(url)
			h.updateInstanceStatus(serviceName, url, healthy)
		}
	}
}

// checkInstanceHealth 检查单个实例的健康状态
func (h *HealthChecker) checkInstanceHealth(url string) bool {
	// 这里简化检查，实际应该更健壮
	resp, err := h.Client.Get(url + "/healthz")
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// updateInstanceStatus 更新实例健康状态
func (h *HealthChecker) updateInstanceStatus(serviceName, url string, healthy bool) {
	h.CheckMutex.Lock()
	defer h.CheckMutex.Unlock()

	// 检查服务是否已注册，同步修改引用
	if _, exists := h.Checks[serviceName]; !exists {
		h.Checks[serviceName] = make(map[string]bool)
	}

	wasHealthy := h.Checks[serviceName][url]
	h.Checks[serviceName][url] = healthy

	// 记录健康状态变化
	if wasHealthy != healthy {
		status := "healthy"
		if !healthy {
			status = "unhealthy"
		}
		fmt.Printf("[%s] Instance %s is now %s\n", time.Now().Format("2006-01-02 15:04:05"), url, status)
	}
}

// IsInstanceHealthy 检查实例是否健康
func (h *HealthChecker) IsInstanceHealthy(serviceName, url string) bool {
	h.CheckMutex.RLock()
	defer h.CheckMutex.RUnlock()

	// 检查服务和实例是否存在，同步修改引用
	if status, exists := h.Checks[serviceName][url]; exists {
		return status
	}
	return false
}
