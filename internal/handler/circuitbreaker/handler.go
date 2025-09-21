package circuitbreaker

import (
	"encoding/json"
	"log"
	"net/http"

	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/service/circuitbreaker"
)

type CircuitBreakerHandler struct {
	gateway *config.GatewayConfig
	svc     circuitbreaker.Service
}

func NewCircuitBreakerHandler(gateway *config.GatewayConfig, svc circuitbreaker.Service) *CircuitBreakerHandler {
	return &CircuitBreakerHandler{
		gateway: gateway,
		svc:     svc,
	}
}

func (h *CircuitBreakerHandler) Status(w http.ResponseWriter, r *http.Request) {
	statuses := h.svc.GetAllState()

	response := map[string]interface{}{
		"status":   "ok",
		"circuits": statuses,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("[ERROR] CircuitBreakerHandler: 编码响应时出错: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func (h *CircuitBreakerHandler) Reset(w http.ResponseWriter, r *http.Request) {
	serviceName := r.URL.Query().Get("service")
	if serviceName == "" {
		log.Printf("[ERROR] CircuitBreakerHandler: 重置服务时未提供服务名称")
		http.Error(w, "缺少服务名称参数", http.StatusBadRequest)
		return
	}
	err := h.svc.Reset(serviceName)
	if err != nil {
		log.Printf("[ERROR] CircuitBreakerHandler: 重置服务 %s 时出错: %v", serviceName, err)
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
		log.Printf("[ERROR] CircuitBreakerHandler: 编码响应时出错: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
