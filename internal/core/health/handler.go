// internal/handlers/health_handler.go
package health

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/pkg/logger"
)

type HealthHandler struct {
	gateway       *config.GatewayConfig
	healthChecker *HealthChecker // 添加health checker引用
	logger        logger.Logger  // 添加日志器
}

func NewHealthHandler(gatewayCfg *config.GatewayConfig, hc *HealthChecker, log logger.Logger) *HealthHandler {
	return &HealthHandler{
		gateway:       gatewayCfg,
		healthChecker: hc,  // 传入health checker
		logger:        log, // 传入日志器
	}
}

func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	// 记录健康检查请求
	h.logger.Info(ctx, "健康检查请求", "method", r.Method, "url", r.URL.Path)

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
		h.logger.Info(ctx, "健康检查: 返回所有服务状态", "port", port)
	} else {
		// 获取单个服务的健康状态
		// 这里需要从路由中获取服务名，但需要重构来传递health checker
		response = map[string]interface{}{
			"status":      "ok",
			"check_scope": "single-service",
			"port":        port,
		}
		h.logger.Info(ctx, "健康检查: 返回单个服务状态", "port", port)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Error(ctx, "写入健康检查响应失败", "error", err)
	}
}
