package health

import (
	"context"
	"net/http"
	"sync"
	"time"

	"gateway.example/go-gateway/pkg/logger"
)

// HealthChecker 负责监控所有上游服务实例的健康状况。
type HealthChecker struct {
	client      *http.Client
	services    sync.Map // 使用 sync.Map 替代 map + RWMutex，更适合"写少读多"的场景
	stopChan    chan struct{}
	checkTicker *time.Ticker
	log         logger.Logger
}

// ServiceCheckInfo 存储单个服务的所有健康检查相关信息。
type ServiceCheckInfo struct {
	Instances   []string
	HealthPath  string
	Status      map[string]bool // Instance URL -> isHealthy
	statusMutex sync.RWMutex
}

// NewHealthChecker 创建一个新的 HealthChecker 实例。
func NewHealthChecker(timeout time.Duration, interval time.Duration, log logger.Logger) *HealthChecker {
	return &HealthChecker{
		client: &http.Client{
			Timeout: timeout,
		},
		stopChan:    make(chan struct{}),
		checkTicker: time.NewTicker(interval),
		log:         log,
	}
}

// RegisterService 注册一个服务及其所有实例以进行健康检查。
func (h *HealthChecker) RegisterService(serviceName string, instances []string, healthPath string) {
	statusMap := make(map[string]bool)
	for _, instURL := range instances {
		statusMap[instURL] = true // 初始状态默认为健康
	}

	serviceInfo := &ServiceCheckInfo{
		Instances:  instances,
		HealthPath: healthPath,
		Status:     statusMap,
	}
	h.services.Store(serviceName, serviceInfo)

	h.log.Info(context.Background(), "[HealthChecker] 服务已注册", "service", serviceName, "instance_count", len(instances), "health_path", healthPath)
}

// Start 在一个独立的 goroutine 中启动周期性健康检查。
func (h *HealthChecker) Start() {
	h.log.Info(context.Background(), "[HealthChecker] 开始周期性健康检查...")
	for {
		select {
		case <-h.checkTicker.C:
			h.runAllHealthChecks()
		case <-h.stopChan:
			h.checkTicker.Stop()
			h.log.Info(context.Background(), "[HealthChecker] 已停止。")
			return
		}
	}
}

// Shutdown 优雅地停止健康检查器。
func (h *HealthChecker) Shutdown() {
	close(h.stopChan)
}

// runAllHealthChecks 遍历所有已注册的服务并并发执行检查。
func (h *HealthChecker) runAllHealthChecks() {
	ctx := context.Background()
	var wg sync.WaitGroup
	h.services.Range(func(key, value interface{}) bool {
		serviceName := key.(string)
		serviceInfo := value.(*ServiceCheckInfo)

		wg.Add(1)
		go func(name string, info *ServiceCheckInfo) {
			defer wg.Done()
			h.checkService(ctx, name, info)
		}(serviceName, serviceInfo)

		return true // 继续遍历
	})
	wg.Wait()
}

// checkService 检查单个服务的所有实例。
func (h *HealthChecker) checkService(ctx context.Context, serviceName string, info *ServiceCheckInfo) {
	for _, instURL := range info.Instances {
		checkURL := instURL + info.HealthPath
		resp, err := h.client.Get(checkURL)

		isHealthy := err == nil && resp.StatusCode == http.StatusOK
		if err == nil {
			resp.Body.Close()
		}

		h.updateInstanceStatus(ctx, serviceName, info, instURL, isHealthy)
	}
}

func (h *HealthChecker) updateInstanceStatus(ctx context.Context, serviceName string, info *ServiceCheckInfo, url string, isHealthy bool) {
	info.statusMutex.Lock()
	defer info.statusMutex.Unlock()

	wasHealthy, exists := info.Status[url]
	if !exists || wasHealthy != isHealthy {
		statusStr := "健康"
		if !isHealthy {
			statusStr = "不健康"
		}
		h.log.Info(ctx, "[HealthChecker] 状态变更 -> 服务: %s, 实例: %s, 当前状态: %s", serviceName, url, statusStr)
		info.Status[url] = isHealthy
	}
}

// IsInstanceHealthy 检查特定实例的当前健康状态。
func (h *HealthChecker) IsInstanceHealthy(serviceName, url string) bool {
	val, ok := h.services.Load(serviceName)
	if !ok {
		return false // 服务未注册
	}
	info := val.(*ServiceCheckInfo)

	info.statusMutex.RLock()
	defer info.statusMutex.RUnlock()

	isHealthy, exists := info.Status[url]
	return exists && isHealthy
}

// GetAllStatuses 返回所有服务的健康状态，用于 /healthz 端点。
func (h *HealthChecker) GetAllStatuses() map[string]map[string]bool {
	statuses := make(map[string]map[string]bool)
	h.services.Range(func(key, value interface{}) bool {
		serviceName := key.(string)
		info := value.(*ServiceCheckInfo)

		info.statusMutex.RLock()
		defer info.statusMutex.RUnlock()

		instanceStatuses := make(map[string]bool)
		for url, isHealthy := range info.Status {
			instanceStatuses[url] = isHealthy
		}
		statuses[serviceName] = instanceStatuses
		return true
	})
	return statuses
}

// GetServiceStatus 返回单个服务的健康状态
func (h *HealthChecker) GetServiceStatus(serviceName string) map[string]bool {
	val, ok := h.services.Load(serviceName)
	if !ok {
		return nil // 服务未注册
	}
	info := val.(*ServiceCheckInfo)

	info.statusMutex.RLock()
	defer info.statusMutex.RUnlock()

	statuses := make(map[string]bool)
	for url, isHealthy := range info.Status {
		statuses[url] = isHealthy
	}
	return statuses
}
