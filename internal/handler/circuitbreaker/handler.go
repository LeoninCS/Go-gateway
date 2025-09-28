package circuitbreaker

import (
	"encoding/json"
	"net/http"

	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/service/circuitbreaker"
	"gateway.example/go-gateway/pkg/logger"
)

type CircuitBreakerHandler struct {
	gateway *config.GatewayConfig
	svc     circuitbreaker.Service
	log     logger.Logger
}

func NewCircuitBreakerHandler(gateway *config.GatewayConfig, svc circuitbreaker.Service, log logger.Logger) *CircuitBreakerHandler {
	return &CircuitBreakerHandler{
		gateway: gateway,
		svc:     svc,
		log:     log,
	}
}

func (h *CircuitBreakerHandler) Status(w http.ResponseWriter, r *http.Request) {
	statuses := h.svc.GetAllState(r.Context())

	response := map[string]interface{}{
		"status":   "ok",
		"circuits": statuses,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error(r.Context(), "[Handler] 编码响应时出错", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func (h *CircuitBreakerHandler) Reset(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("service")
	if serviceName == "" {
		h.log.Error(r.Context(), "[Handler] 重置服务时未提供服务名称")
		http.Error(w, "缺少服务名称参数", http.StatusBadRequest)
		return
	}
	err := h.svc.Reset(r.Context(), serviceName)
	if err != nil {
		h.log.Error(r.Context(), "[Handler] 重置服务 %s 时出错", "service", serviceName, "error", err)
		http.Error(w, "重置熔断器失败", http.StatusInternalServerError)
		return
	}
	response := map[string]string{
		"status":  "ok",
		"message": "熔断器重置成功",
		"service": serviceName,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.log.Error(r.Context(), "[Handler] 编码响应时出错", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
