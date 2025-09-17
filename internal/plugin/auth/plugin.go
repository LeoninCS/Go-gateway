// file: internal/plugin/auth/plugin.go
package auth

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"gateway.example/go-gateway/internal/config" // ★ 引入 config 包
	"gateway.example/go-gateway/internal/core/health"
	"gateway.example/go-gateway/internal/core/loadbalancer"
)

const (
	PluginName = "auth" // 定义插件名称常量，与YAML配置保持一致
)

// Plugin 实现了认证插件的逻辑。
type Plugin struct {
	client        *http.Client
	lbFactory     *loadbalancer.LoadBalancerFactory
	healthChecker *health.HealthChecker
	serviceName   string
}

// NewPlugin 创建一个新的认证插件实例
func NewPlugin(lbFactory *loadbalancer.LoadBalancerFactory, hc *health.HealthChecker, serviceName string) (*Plugin, error) {
	return &Plugin{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		lbFactory:     lbFactory,
		healthChecker: hc,
		serviceName:   serviceName,
	}, nil
}

// Execute 方法中修改验证请求的URL获取方式
func (p *Plugin) Execute(w http.ResponseWriter, r *http.Request, pluginCfg config.PluginSpec) (bool, error) {
	// (未使用 pluginCfg 参数，但签名必须匹配)
	_ = pluginCfg

	log.Printf("[插件: %s] 开始执行...", p.Name())

	// 1. --- 从 Header 中获取 Authorization ---
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		log.Printf("[插件: %s] 未授权: 缺少 Authorization 请求头", p.Name())
		http.Error(w, "Unauthorized: Authorization header required", http.StatusUnauthorized)
		return false, nil // 中断执行链
	}

	// 2. --- 校验 "Bearer " 前缀并提取 token ---
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		log.Printf("[插件: %s] 未授权: Authorization 请求头格式无效", p.Name())
		http.Error(w, `Unauthorized: Invalid Authorization header format (expected "Bearer <token>")`, http.StatusUnauthorized)
		return false, nil
	}

	// 3. --- 使用负载均衡器获取健康的auth-service实例 ---
	lb := p.lbFactory.GetOrCreateLoadBalancer(p.serviceName, "round_robin")
	instance, err := p.getHealthyInstance(lb)
	if err != nil {
		log.Printf("[插件: %s] 服务不可用: 无法获取健康实例: %v", p.Name(), err)
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return false, err
	}

	// 4. --- 创建并发送 HTTP 请求到认证服务 ---
	// 实例URL已经包含协议前缀，直接拼接路径即可
	validateURL := instance.URL + "/validate"
	req, err := http.NewRequestWithContext(r.Context(), "POST", validateURL, nil)
	if err != nil {
		log.Printf("[插件: %s] 内部错误: 创建 HTTP 请求失败: %v", p.Name(), err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return false, fmt.Errorf("创建认证 HTTP 请求失败: %w", err)
	}
	req.Header.Set("Authorization", authHeader)

	resp, err := p.client.Do(req)
	if err != nil {
		log.Printf("[插件: %s] 服务不可用: 调用认证服务失败: %v", p.Name(), err)
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		return false, err
	}
	defer resp.Body.Close()

	// 5. --- 根据 auth-service 的响应决定是否放行 ---
	if resp.StatusCode == http.StatusOK {
		log.Printf("[插件: %s] 授权成功: Token 有效", p.Name())
		return true, nil // 成功，继续执行
	}

	log.Printf("[插件: %s] 未授权: Token 无效 (认证服务返回状态码 %d)", p.Name(), resp.StatusCode)
	http.Error(w, "Unauthorized", http.StatusUnauthorized)
	return false, nil
}

func (p *Plugin) Name() string {
	return PluginName
}

// getHealthyInstance 从负载均衡器获取健康实例
func (p *Plugin) getHealthyInstance(lb loadbalancer.LoadBalancer) (*loadbalancer.ServiceInstance, error) {
	maxRetries := 3
	for i := 0; i < maxRetries; i++ {
		instance, err := lb.GetNextInstance(p.serviceName)
		if err != nil {
			return nil, fmt.Errorf("获取服务实例失败: %w", err)
		}
		if instance == nil {
			return nil, fmt.Errorf("没有可用的服务实例")
		}

		// 检查实例健康状态
		if p.healthChecker.IsInstanceHealthy(p.serviceName, instance.URL) {
			return instance, nil
		}

		time.Sleep(100 * time.Millisecond) // 短暂等待后重试
	}
	return nil, fmt.Errorf("无法找到健康实例，重试 %d 次后失败", maxRetries)
}
