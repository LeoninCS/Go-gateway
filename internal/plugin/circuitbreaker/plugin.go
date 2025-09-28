package circuitbreaker

import (
	"context"
	"fmt"
	"net/http"

	"gateway.example/go-gateway/internal/config"
	pl_circuitbreaker "gateway.example/go-gateway/internal/service/circuitbreaker"
	"gateway.example/go-gateway/pkg/logger"
)

const PluginName = "circuitbreaker"

type Plugin struct {
	circuitBreakerSvc pl_circuitbreaker.Service
	log               logger.Logger
}

func NewPlugin(svc pl_circuitbreaker.Service, log logger.Logger) *Plugin {
	if svc == nil {
		log.Fatal(context.Background(), "[插件 %s] 致命错误: circuitbreaker.Service 依赖注入失败，为 nil", PluginName)
	}
	return &Plugin{
		circuitBreakerSvc: svc,
		log:               log,
	}
}

func (p *Plugin) Name() string {
	return PluginName
}

func (p *Plugin) Execute(w http.ResponseWriter, r *http.Request, pluginCfg config.PluginSpec) (bool, error) {
	ctx := r.Context()

	// 1. 解析插件配置
	serviceName, err := p.parseConfig(pluginCfg)
	if err != nil {
		p.log.Error(ctx, "[插件] 熔断插件配置错误", "plugin", p.Name(), "error", err)
		http.Error(w, "熔断插件配置错误", http.StatusInternalServerError)
		return false, fmt.Errorf("[插件 %s] %w", p.Name(), err)
	}

	// 2. 检查熔断状态
	allowed, err := p.circuitBreakerSvc.CheckCircuit(ctx, serviceName)
	if err != nil {
		p.log.Error(ctx, "[插件] 调用熔断服务失败", "plugin", p.Name(), "service", serviceName, "error", err)
		http.Error(w, "熔断服务内部错误", http.StatusInternalServerError)
		return false, fmt.Errorf("[插件 %s] 调用熔断服务失败: %w", p.Name(), err)
	}

	if !allowed {
		p.log.Warn(ctx, "[插件] 请求被熔断", "plugin", p.Name(), "service", serviceName)
		http.Error(w, "服务暂时不可用", http.StatusServiceUnavailable)
		return false, nil // 中断插件链
	}

	p.log.Info(ctx, "[插件] 熔断检查通过", "plugin", p.Name(), "service", serviceName)
	return true, nil // 继续下一个插件
}

func (p *Plugin) parseConfig(cfg config.PluginSpec) (string, error) {
	service, ok := cfg["service"].(string)
	if !ok || service == "" {
		return "", fmt.Errorf("配置 'service' 缺失或类型不正确")
	}
	return service, nil
}
