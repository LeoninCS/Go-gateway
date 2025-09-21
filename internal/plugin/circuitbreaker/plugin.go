package circuitbreaker

import (
	"fmt"
	"log"
	"net/http"

	"gateway.example/go-gateway/internal/config"
	pl_circuitbreaker "gateway.example/go-gateway/internal/service/circuitbreaker"
)

const PluginName = "circuitbreaker"

type Plugin struct {
	circuitBreakerSvc pl_circuitbreaker.Service
}

func NewPlugin(svc pl_circuitbreaker.Service) *Plugin {
	if svc == nil {
		log.Fatalf("[插件 %s] 致命错误: circuitbreaker.Service 依赖注入失败，为 nil", PluginName)
	}
	return &Plugin{
		circuitBreakerSvc: svc,
	}
}

func (p *Plugin) Name() string {
	return PluginName
}

func (p *Plugin) Execute(w http.ResponseWriter, r *http.Request, pluginCfg config.PluginSpec) (bool, error) {
	// 1. 解析插件配置
	serviceName, err := p.parseConfig(pluginCfg)
	if err != nil {
		http.Error(w, "熔断插件配置错误", http.StatusInternalServerError)
		return false, fmt.Errorf("[插件 %s] %w", p.Name(), err)
	}

	// 2. 检查熔断状态
	allowed, err := p.circuitBreakerSvc.CheckCircuit(serviceName)
	if err != nil {
		http.Error(w, "熔断服务内部错误", http.StatusInternalServerError)
		return false, fmt.Errorf("[插件 %s] 调用熔断服务失败: %w", p.Name(), err)
	}

	if !allowed {
		log.Printf("[插件 %s] 请求被熔断: [服务: %s]", p.Name(), serviceName)
		http.Error(w, "服务暂时不可用", http.StatusServiceUnavailable)
		return false, nil // 中断插件链
	}

	return true, nil // 继续下一个插件
}

func (p *Plugin) parseConfig(cfg config.PluginSpec) (string, error) {
	service, ok := cfg["service"].(string)
	if !ok || service == "" {
		return "", fmt.Errorf("配置 'service' 缺失或类型不正确")
	}
	return service, nil
}
