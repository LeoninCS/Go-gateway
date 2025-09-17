package health

import (
	"log" // 推荐使用 log 模块以获得带时间戳的输出
	"net/http"
	"sync"
	"time"
)

// HealthChecker 负责监控所有上游服务实例的健康状况。
type HealthChecker struct {
	client      *http.Client
	services    sync.Map // 使用 sync.Map 替代 map + RWMutex，更适合"写少读多"的场景
	stopChan    chan struct{}
	checkTicker *time.Ticker
}

// ServiceCheckInfo 存储单个服务的所有健康检查相关信息。
type ServiceCheckInfo struct {
	Instances   []string
	HealthPath  string
	Status      map[string]bool // Instance URL -> isHealthy
	statusMutex sync.RWMutex
}

// NewHealthChecker 创建一个新的 HealthChecker 实例。
// ★ 修正 1: 构造函数应接收全局配置，以便获取超时设置。
func NewHealthChecker(timeout time.Duration, interval time.Duration) *HealthChecker {
	return &HealthChecker{
		client: &http.Client{
			Timeout: timeout,
		},
		stopChan:    make(chan struct{}),
		checkTicker: time.NewTicker(interval),
	}
}

// RegisterService 注册一个服务及其所有实例以进行健康检查。
// ★ 函数名和逻辑保持，但内部实现使用 sync.Map。
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
}

// Start 在一个独立的 goroutine 中启动周期性健康检查。
// ★ 修正 2: 移除 for 循环。启动逻辑应由外部调用者决定是否放入 goroutine。
//
//	我们将在 Gateway 中使用 `go h.Start()` 来调用它。
func (h *HealthChecker) Start() {
	log.Println("[HealthChecker] 开始周期性健康检查...")
	for {
		select {
		case <-h.checkTicker.C:
			h.runAllHealthChecks()
		case <-h.stopChan:
			h.checkTicker.Stop()
			log.Println("[HealthChecker] 已停止。")
			return
		}
	}
}

// Shutdown 优雅地停止健康检查器。
// ★ 修正 3: 函数名从 Stop 改为 Shutdown 以符合 Gateway 中的调用，语义更清晰。
func (h *HealthChecker) Shutdown() {
	close(h.stopChan)
}

// runAllHealthChecks 遍历所有已注册的服务并并发执行检查。
func (h *HealthChecker) runAllHealthChecks() {
	var wg sync.WaitGroup
	h.services.Range(func(key, value interface{}) bool {
		serviceName := key.(string)
		serviceInfo := value.(*ServiceCheckInfo)

		wg.Add(1)
		go func(name string, info *ServiceCheckInfo) {
			defer wg.Done()
			h.checkService(name, info)
		}(serviceName, serviceInfo)

		return true // 继续遍历
	})
	wg.Wait()
}

// checkService 检查单个服务的所有实例。
func (h *HealthChecker) checkService(serviceName string, info *ServiceCheckInfo) {
	for _, instURL := range info.Instances {
		// ★ 修正 4: 使用注册时提供的 healthPath，而不是硬编码。
		checkURL := instURL + info.HealthPath
		resp, err := h.client.Get(checkURL)

		isHealthy := err == nil && resp.StatusCode == http.StatusOK
		if err == nil {
			resp.Body.Close()
		}

		h.updateInstanceStatus(serviceName, info, instURL, isHealthy)
	}
}

func (h *HealthChecker) updateInstanceStatus(serviceName string, info *ServiceCheckInfo, url string, isHealthy bool) {
	info.statusMutex.Lock()
	defer info.statusMutex.Unlock()

	wasHealthy, exists := info.Status[url]
	if !exists || wasHealthy != isHealthy {
		statusStr := "健康"
		if !isHealthy {
			statusStr = "不健康"
		}
		log.Printf("[HealthChecker] 状态变更 -> 服务: %s, 实例: %s, 当前状态: %s", serviceName, url, statusStr)
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
