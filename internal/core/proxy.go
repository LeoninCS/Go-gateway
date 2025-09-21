// package core 提供了网关的核心路由和代理功能。
package core

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/core/health"
	"gateway.example/go-gateway/internal/core/loadbalancer"
	"gateway.example/go-gateway/internal/service/circuitbreaker"
)

// Proxy 负责将请求转发到后端服务。
type Proxy struct {
	lbFactory         *loadbalancer.LoadBalancerFactory
	healthChecker     *health.HealthChecker
	circuitBreakerSvc circuitbreaker.Service // 添加熔断器服务依赖
}

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

// NewProxy 创建一个新的 Proxy 实例。
func NewProxy(lbFactory *loadbalancer.LoadBalancerFactory, hc *health.HealthChecker, cbSvc circuitbreaker.Service) *Proxy {
	return &Proxy{
		lbFactory:         lbFactory,
		healthChecker:     hc,
		circuitBreakerSvc: cbSvc,
	}
}

// ServeHTTP 执行反向代理的核心逻辑。
// (此函数签名已符合指针最佳实践，无需修改)
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request, route *config.RouteConfig, service *config.ServiceConfig) {
	// ★ 推荐实践: 在使用指针前进行 nil 检查，增强代码健壮性。
	if service == nil {
		log.Printf("[Proxy] 内部错误: 服务配置为 nil (路由: %s)", route.PathPrefix)
		http.Error(w, "网关内部配置错误", http.StatusInternalServerError)
		return
	}

	// 1. 获取该服务对应的负载均衡器
	lb := p.lbFactory.GetOrCreateLoadBalancer(
		service.Name,
		service.LoadBalancer,
	)

	// 2. 获取一个健康的实例
	instance, err := p.getHealthyInstance(lb, service.Name)
	if err != nil {
		log.Printf("[Proxy] 错误: 服务 '%s' 无可用实例: %v", service.Name, err)
		http.Error(w, fmt.Sprintf("服务 '%s' 当前不可用", service.Name), http.StatusServiceUnavailable)
		return
	}
	log.Printf("[Proxy] 信息: 为服务 '%s' 选择的健康实例: %s", service.Name, instance.URL)

	// 3. 创建反向代理
	targetURL, err := url.Parse(instance.URL)
	if err != nil {
		log.Printf("[Proxy] 内部错误: 解析实例 URL '%s' 失败: %v", instance.URL, err)
		http.Error(w, "网关内部错误", http.StatusInternalServerError)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// 4. 设置 director 来重写请求
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req) // 执行默认的 host, scheme 等重写

		// ★ 新增: 路径重写逻辑 - 移除路由前缀
		originalPath := req.URL.Path
		if len(route.PathPrefix) > 0 && len(originalPath) >= len(route.PathPrefix) {
			// 移除路径前缀，保留剩余部分
			newPath := originalPath[len(route.PathPrefix):]
			if newPath == "" {
				newPath = "/"
			}
			req.URL.Path = newPath
			log.Printf("[Proxy] 路径重写: %s -> %s", originalPath, newPath)
		}

		req.Header.Set("X-Gateway-Proxy", "true")
		// 可以在此处添加更多基于路由或服务配置的头操作
	}

	// 5. 使用 responseWriterWrapper 捕获响应状态码
	wrapper := &responseWriterWrapper{
		ResponseWriter: w,
		statusCode:     0,
	}

	// 6. 执行代理
	proxy.ServeHTTP(wrapper, r)

	// 7. 根据响应状态码更新熔断器状态
	// 判断请求是否成功（2xx 状态码视为成功，其他视为失败）
	statusCode := wrapper.GetStatusCode()
	success := statusCode >= 200 && statusCode < 300

	if p.circuitBreakerSvc != nil {
		log.Printf("[Proxy] 服务 '%s' 请求完成，状态码: %d, 成功: %v", service.Name, statusCode, success)
		p.circuitBreakerSvc.RecordResult(service.Name, success)
	}
}

// getHealthyInstance 封装了“获取下一个健康实例”的逻辑 (代码无误，无需修改)
func (p *Proxy) getHealthyInstance(lb loadbalancer.LoadBalancer, serviceName string) (*loadbalancer.ServiceInstance, error) {
	allInstances := lb.GetAllInstances(serviceName)
	if len(allInstances) == 0 {
		return nil, errors.New("服务未注册任何实例")
	}

	// 尝试次数等于实例总数，避免在所有实例都不健康时无限循环
	maxAttempts := len(allInstances)
	for i := 0; i < maxAttempts; i++ {
		instance, err := lb.GetNextInstance(serviceName)
		if err != nil {
			return nil, err // 负载均衡器内部错误
		}

		if p.healthChecker.IsInstanceHealthy(serviceName, instance.URL) {
			return instance, nil // 找到健康的实例，立即返回
		}

		log.Printf("[Proxy] 警告: 跳过不健康的实例 '%s' (服务: %s)", instance.URL, serviceName)
	}

	return nil, fmt.Errorf("在所有实例中未找到健康的实例")
}

func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}
func (w *responseWriterWrapper) GetStatusCode() int {
	if w.statusCode == 0 {
		// 如果没有显式设置状态码，默认认为是 200 OK
		return http.StatusOK
	}
	return w.statusCode
}
