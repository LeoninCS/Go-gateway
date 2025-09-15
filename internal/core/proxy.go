package core

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/core/loadbalancer"
)

// Proxy 负责将请求转发到后端服务。
type Proxy struct {
	lbFactory *loadbalancer.LoadBalancerFactory
}

// NewProxy 创建一个新的 Proxy 实例。
func NewProxy(lbFactory *loadbalancer.LoadBalancerFactory) *Proxy {
	return &Proxy{
		lbFactory: lbFactory,
	}
}

// ServeHTTP 执行反向代理的核心逻辑。
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request, route *config.RouteConfig, service *config.ServiceConfig) {
	// 获取负载均衡器
	lb := p.lbFactory.GetOrCreateLoadBalancer(
		service.Name,
		service.LoadBalancer,
	)

	// **调试日志**
	log.Printf("matchedRoute: %v", route)
	log.Printf("Instances for %s: %v", route.ServiceName, lb.GetAllInstances(route.ServiceName))

	// 获取目标服务实例
	instance, err := lb.GetNextInstance(route.ServiceName)
	if err != nil {
		log.Printf("GetNextInstance error: %v", err)
		http.Error(w, fmt.Sprintf("Service error: %v", err), http.StatusServiceUnavailable)
		return
	}

	// 创建反向代理
	target, err := url.Parse(instance.URL)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	proxy := httputil.NewSingleHostReverseProxy(target)

	// 使用中间件跟踪请求连接数 (如果负载均衡器支持)
	if lbType, ok := lb.(interface{ ReleaseConnection(string, string) }); ok {
		defer func() {
			lbType.ReleaseConnection(route.ServiceName, instance.URL)
		}()
	}
	// 使用代理转发请求
	proxy.ServeHTTP(w, r)
}
