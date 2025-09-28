// file: internal/plugin/ratelimit/plugin.go
package ratelimit

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"gateway.example/go-gateway/internal/config"
	svc_ratelimit "gateway.example/go-gateway/internal/service/ratelimit"
	"gateway.example/go-gateway/pkg/logger"
)

const (
	PluginName          = "ratelimit"
	HeaderXForwardedFor = "X-Forwarded-For"
	HeaderXRealIP       = "X-Real-IP"
)

// Plugin 实现了 plugin.Interface 接口
type Plugin struct {
	rateLimitSvc svc_ratelimit.Service
	log          logger.Logger
}

// NewPlugin 创建一个新的限流插件实例
func NewPlugin(svc svc_ratelimit.Service, log logger.Logger) *Plugin {
	if svc == nil {
		log.Fatal(context.Background(), "[插件 %s] 致命错误: ratelimit.Service 依赖注入失败，为 nil", PluginName)
	}
	return &Plugin{
		rateLimitSvc: svc,
		log:          log,
	}
}

// Name 返回插件的名称
func (p *Plugin) Name() string {
	return PluginName
}

// Execute 执行插件的核心逻辑
func (p *Plugin) Execute(w http.ResponseWriter, r *http.Request, pluginCfg config.PluginSpec) (bool, error) {
	ctx := r.Context()

	// 1. 解析插件配置
	ruleName, strategy, err := p.parseConfig(pluginCfg)
	if err != nil {
		http.Error(w, "限流插件配置错误", http.StatusInternalServerError)
		return false, fmt.Errorf("[插件 %s] %w", p.Name(), err)
	}

	// 2. 根据策略提取标识符
	identifier := p.getIdentifier(r, strategy)
	if identifier == "" {
		p.log.Warn(ctx, "[插件 %s] 警告: 未能根据策略 '%s' 找到有效的请求标识符",
			p.Name(), strategy,
			"plugin", p.Name(),
			"strategy", strategy)
		// 如果无法识别，可以选择放行或拒绝，这里选择放行并记录日志
		return true, nil
	}

	// 3. 使用新的 Service 接口进行限流检查
	allowed, err := p.rateLimitSvc.CheckLimit(ctx, ruleName, identifier)
	if err != nil {
		http.Error(w, "限流服务内部错误", http.StatusInternalServerError)
		return false, fmt.Errorf("[插件 %s] 调用限流服务失败: %w", p.Name(), err)
	}

	if !allowed {
		p.log.Info(ctx, "[插件 %s] 请求被拒绝: [规则: %s, 标识: %s]",
			p.Name(), ruleName, identifier,
			"plugin", p.Name(),
			"rule", ruleName,
			"identifier", identifier,
			"action", "rejected")
		http.Error(w, "请求过于频繁", http.StatusTooManyRequests)
		return false, nil // 中断插件链
	}

	// 请求被允许，不打印日志，避免日志泛滥
	return true, nil // 继续下一个插件
}

// parseConfig 从配置中解析出规则名称和策略
func (p *Plugin) parseConfig(cfg config.PluginSpec) (string, string, error) {
	rule, ok := cfg["rule"].(string)
	if !ok || rule == "" {
		return "", "", fmt.Errorf("配置 'rule' 缺失或类型不正确")
	}

	strategy, ok := cfg["strategy"].(string)
	if !ok || strategy == "" {
		return "", "", fmt.Errorf("配置 'strategy' 缺失或类型不正确")
	}

	return rule, strategy, nil
}

// getIdentifier 根据策略从请求中获取唯一标识符
func (p *Plugin) getIdentifier(r *http.Request, strategy string) string {
	switch strategy {
	case "ip":
		// 遵循标准实践，优先 X-Forwarded-For
		xff := r.Header.Get(HeaderXForwardedFor)
		if xff != "" {
			// XFF 可能包含多个 IP: "client, proxy1, proxy2"
			// 第一个通常是真实客户端 IP
			ips := strings.Split(xff, ",")
			clientIP := strings.TrimSpace(ips[0])
			return clientIP
		}

		// 其次是 X-Real-IP
		ip := r.Header.Get(HeaderXRealIP)
		if ip != "" {
			return ip
		}

		// 最后回退到 RemoteAddr，它可能是直接连接的客户端或上一级代理的 IP
		// net.SplitHostPort 用于去除可能存在的端口号
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			// 如果没有端口号，直接返回
			return r.RemoteAddr
		}
		return host
	case "path":
		return r.URL.Path
	default:
		return ""
	}
}
