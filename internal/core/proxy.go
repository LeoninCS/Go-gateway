// package core 提供了网关的核心路由和代理功能。
package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"

	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/core/health"
	"gateway.example/go-gateway/internal/core/loadbalancer"
	"gateway.example/go-gateway/internal/service/circuitbreaker"
	"gateway.example/go-gateway/pkg/logger"
)

// Proxy 负责将请求转发到后端服务。
type Proxy struct {
	lbFactory         *loadbalancer.LoadBalancerFactory
	healthChecker     *health.HealthChecker
	circuitBreakerSvc circuitbreaker.Service // 添加熔断器服务依赖
	logger            logger.Logger          // 添加日志器
}

type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

// NewProxy 创建一个新的 Proxy 实例。
func NewProxy(lbFactory *loadbalancer.LoadBalancerFactory, hc *health.HealthChecker, cbSvc circuitbreaker.Service, log logger.Logger) *Proxy {
	return &Proxy{
		lbFactory:         lbFactory,
		healthChecker:     hc,
		circuitBreakerSvc: cbSvc,
		logger:            log,
	}
}

// ServeHTTP 执行反向代理的核心逻辑。
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request, route *config.RouteConfig, service *config.ServiceConfig) {
	ctx := r.Context()

	// 推荐实践: 在使用指针前进行 nil 检查，增强代码健壮性。
	if service == nil {
		p.logger.Error(ctx, "[Proxy] 内部错误: 服务配置为 nil", "route", route.PathPrefix)
		http.Error(w, "网关内部配置错误", http.StatusInternalServerError)
		return
	}

	// 1. 获取该服务对应的负载均衡器
	lb := p.lbFactory.GetOrCreateLoadBalancer(
		service.Name,
		service.LoadBalancer,
	)

	// 2. 获取一个健康的实例
	instance, err := p.getHealthyInstance(ctx, lb, service.Name)
	if err != nil {
		p.logger.Error(ctx, "[Proxy] 错误: 服务无可用实例", "service", service.Name, "error", err)
		http.Error(w, fmt.Sprintf("服务 '%s' 当前不可用", service.Name), http.StatusServiceUnavailable)
		return
	}
	p.logger.Info(ctx, "[Proxy] 信息: 为服务选择健康实例", "service", service.Name, "instance", instance.URL)

	// 3. 创建反向代理
	targetURL, err := url.Parse(instance.URL)
	if err != nil {
		p.logger.Error(ctx, "[Proxy] 内部错误: 解析实例URL失败", "instance_url", instance.URL, "error", err)
		http.Error(w, "网关内部错误", http.StatusInternalServerError)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(targetURL)

	// 4. 设置 director 来重写请求
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req) // 执行默认的 host, scheme 等重写

		// 新增: 路径重写逻辑 - 移除路由前缀
		originalPath := req.URL.Path
		if len(route.PathPrefix) > 0 && len(originalPath) >= len(route.PathPrefix) {
			// 移除路径前缀，保留剩余部分
			newPath := originalPath[len(route.PathPrefix):]
			if newPath == "" {
				newPath = "/"
			}
			req.URL.Path = newPath
			p.logger.Info(req.Context(), "[Proxy] 路径重写", "original_path", originalPath, "new_path", newPath)
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
		p.logger.Info(ctx, "[Proxy] 服务请求完成", "service", service.Name, "status_code", statusCode, "success", success)
		p.circuitBreakerSvc.RecordResult(ctx, service.Name, success)
	}
}

// getHealthyInstance 封装了"获取下一个健康实例"的逻辑
func (p *Proxy) getHealthyInstance(ctx context.Context, lb loadbalancer.LoadBalancer, serviceName string) (*loadbalancer.ServiceInstance, error) {
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

		p.logger.Warn(ctx, "[Proxy] 警告: 跳过不健康的实例", "instance", instance.URL, "service", serviceName)
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
