// internal/handlers/health_handler.go
package health

import (
	"net/http"
)

type HealthHandler struct {
	// 可以在这里添加依赖，比如数据库连接等
}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

func (h *HealthHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok", "service": "gateway"}`))
}
