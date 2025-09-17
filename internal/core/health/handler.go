// internal/handlers/health_handler.go
package health

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"

	"gateway.example/go-gateway/internal/config"
)

type HealthHandler struct {
	gateway       *config.GatewayConfig
	healthChecker *HealthChecker // 添加health checker引用
}

func NewHealthHandler(gatewayCfg *config.GatewayConfig, hc *HealthChecker) *HealthHandler {
	return &HealthHandler{
		gateway:       gatewayCfg,
		healthChecker: hc, // 传入health checker
	}
}

func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	// 获取请求的端口
	port := strings.Split(r.Host, ":")[1]
	portNum, _ := strconv.Atoi(port)

	// 根据端口决定检测范围
	var response interface{}
	if portNum == 8080 {
		// 获取所有服务的健康状态
		response = map[string]interface{}{
			"status":      "ok",
			"check_scope": "all-services",
			"services":    h.healthChecker.GetAllStatuses(),
		}
	} else {
		// 获取单个服务的健康状态
		// 这里需要从路由中获取服务名，但需要重构来传递health checker
		response = map[string]interface{}{
			"status":      "ok",
			"check_scope": "single-service",
			"port":        port,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("写入健康检查响应失败: %v", err)
	}
}
