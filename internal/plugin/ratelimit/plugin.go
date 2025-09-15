// internal/plugin/ratelimit/plugin.go
package ratelimit

import (
	"fmt"
	"log"
	"strings"

	"gateway.example/go-gateway/internal/config"
	core_ratelimit "gateway.example/go-gateway/internal/core/ratelimit"
	"gateway.example/go-gateway/internal/handler/middleware"
	"gateway.example/go-gateway/internal/plugin"
	svcr "gateway.example/go-gateway/internal/service/ratelimit"
)

const Name = "rateLimit"

// Plugin 实现了网关的插件接口。
type Plugin struct {
	svc svcr.Service
}

// NewPlugin 创建一个 ratelimit 插件实例。
func NewPlugin(svc svcr.Service) (*Plugin, error) {
	if svc == nil {
		return nil, fmt.Errorf("ratelimit service 不能为空")
	}
	return &Plugin{svc: svc}, nil
}

// Name 返回插件的名称。
func (p *Plugin) Name() string {
	return Name
}

// in: internal/plugin/ratelimit/plugin.go

// CreateMiddleware 根据配置创建限流中间件
func (p *Plugin) CreateMiddleware(spec config.PluginSpec) (plugin.Middleware, error) {
	// 1. 解析 'rule' 字段
	ruleNameVal, ok := spec["rule"]
	if !ok {
		return nil, fmt.Errorf("限流插件配置错误：缺少 'rule' 字段")
	}
	ruleName, ok := ruleNameVal.(string)
	if !ok {
		return nil, fmt.Errorf("限流插件配置错误：'rule' 字段必须是 string 类型")
	}
	// 2. 解析 'strategy' 字段 (现在更简单)
	strategyStr := "ip" // 默认策略
	if strategyVal, exists := spec["strategy"]; exists {
		var ok bool
		strategyStr, ok = strategyVal.(string)
		if !ok {
			return nil, fmt.Errorf("限流插件配置错误：'strategy' 字段必须是 string 类型")
		}
	}
	// 3. (核心变化) 直接调用工厂函数获取 IdentifierFunc
	identifierFunc, err := core_ratelimit.GetIdentifierFunc(strategyStr)
	if err != nil {
		return nil, fmt.Errorf("初始化限流插件失败: %w", err)
	}
	log.Printf("[DEBUG] 创建限流中间件: rule='%s', strategy='%s'", ruleName, strategyStr)
	// 4. 使用正确的参数调用 NewRateLimiter
	mw := middleware.NewRateLimiter(p.svc, identifierFunc, ruleName)
	return plugin.Middleware(mw), nil
}

// in: internal/plugin/ratelimit/plugin.go

// getIdentifierFuncByStrategy 是一个工厂函数，根据策略字符串返回对应的标识符提取函数。
func getIdentifierFuncByStrategy(strategy string) (core_ratelimit.IdentifierFunc, error) {
	switch strings.ToLower(strategy) {
	case "ip":
		return core_ratelimit.FromIP, nil
	// case "header":
	//   // 如果要支持header策略，这里需要进一步解析配置，比如 `spec["headerName"]`
	//   // return core_ratelimit.FromHeader("X-User-ID"), nil
	case "user":
		return nil, fmt.Errorf("策略 'user' 尚未实现")
	default:
		return nil, fmt.Errorf("不支持的限流策略: '%s'", strategy)
	}
}
