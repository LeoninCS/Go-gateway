package gateway

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"gateway.example/go-gateway/internal/auth"
	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/handlers" // ★ 导入新的 handlers 包
	"gateway.example/go-gateway/internal/loadbalancer"
)

// Gateway 是我们的核心网关处理器
type Gateway struct {
	Config      *config.Config
	Mux         *http.ServeMux
	authHandler *handlers.AuthHandler // ★ 持有具体的 Handler 实例
}

// NewGateway 创建一个新的 Gateway 实例
func NewGateway(cfg *config.Config, authHandler *handlers.AuthHandler) (*Gateway, error) { // ★ 接收 AuthHandler
	gw := &Gateway{
		Config:      cfg,
		Mux:         http.NewServeMux(),
		authHandler: authHandler, // ★ 存储 Handler 实例
	}
	gw.registerRoutes()
	return gw, nil
}

// ServeHTTP 使 Gateway 实现了 http.Handler 接口
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.Mux.ServeHTTP(w, r)
}

// registerRoutes 初始化网关的所有路由规则
func (g *Gateway) registerRoutes() {
	// ===================================================================
	// 1. 注册公共路由 (Public Routes)
	// ===================================================================
	// ★ 调用注入的 handler 中的方法来注册
	g.Mux.HandleFunc("/register", g.authHandler.Register)
	g.Mux.HandleFunc("/login", g.authHandler.Login)
	g.Mux.HandleFunc("/healthz", handlers.Healthz) // ★ 调用包级别的函数

	// ===================================================================
	// 2. 注册受保护的路由 (Protected Routes)
	// ===================================================================
	jwtMiddleware := auth.Middleware(&g.Config.JWT)
	for i := range g.Config.Services {
		// ... 这部分反向代理的逻辑保持不变 ...
		serviceCfg := &g.Config.Services[i]
		var backends []*loadbalancer.Backend
		for _, endpoint := range serviceCfg.Endpoints {
			target, err := url.Parse(endpoint.URL)
			if err != nil {
				log.Printf("Error parsing target URL '%s' for service '%s': %v. Skipping.", endpoint.URL, serviceCfg.Name, err)
				continue
			}
			proxy := httputil.NewSingleHostReverseProxy(target)
			backends = append(backends, &loadbalancer.Backend{
				URL:          target,
				ReverseProxy: proxy,
				Alive:        true,
			})
		}
		if len(backends) == 0 {
			log.Printf("No valid backends for service '%s'. Skipping.", serviceCfg.Name)
			continue
		}
		lb := loadbalancer.NewLoadBalancer(backends)
		go runHealthChecks(serviceCfg, backends)
		protectedHandler := jwtMiddleware(http.StripPrefix(serviceCfg.Path, lb))
		g.Mux.Handle(serviceCfg.Path, protectedHandler)
		log.Printf("Registered PROTECTED service '%s' at path '%s' with %d backends. Strategy: '%s'",
			serviceCfg.Name, serviceCfg.Path, len(backends), serviceCfg.LoadBalancingStrategy)

	}
}

// runHealthChecks 在后台为指定服务的所有后端运行健康检查
// 这个函数现在是动态的，它会从配置中读取检查间隔和路径
func runHealthChecks(serviceCfg *config.ServiceConfig, backends []*loadbalancer.Backend) {
	// 如果服务没有配置端点，则不执行任何操作
	if len(serviceCfg.Endpoints) == 0 {
		return
	}

	// 使用第一个端点的健康检查间隔作为该服务所有检查的周期
	// 这是一个常见的简化策略
	intervalStr := serviceCfg.Endpoints[0].HealthCheck.Interval
	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		log.Printf("[Health Check] Invalid interval '%s' for service '%s'. Defaulting to 30s. Error: %v",
			intervalStr, serviceCfg.Name, err)
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		// 遍历所有后端，并使用其对应的健康检查配置
		for i, backend := range backends {
			// 确保索引不会越界
			if i >= len(serviceCfg.Endpoints) {
				continue
			}
			endpointCfg := serviceCfg.Endpoints[i]

			// 构建健康检查的完整 URL
			healthCheckURL := *backend.URL // 复制 URL 对象以避免修改原始对象
			healthCheckURL.Path = endpointCfg.HealthCheck.Path

			client := http.Client{Timeout: 3 * time.Second}
			resp, err := client.Get(healthCheckURL.String())

			isAlive := err == nil && resp != nil && resp.StatusCode < 500
			if resp != nil {
				// 必须关闭 Body，否则会导致资源泄露
				_ = resp.Body.Close()
			}

			// 仅当状态发生变化时才更新和记录日志
			if isAlive != backend.IsAlive() {
				backend.SetAlive(isAlive)
				if isAlive {
					log.Printf("[Health Check] Backend UP: %s for service '%s'", backend.URL, serviceCfg.Name)
				} else {
					log.Printf("[Health Check] Backend DOWN: %s for service '%s' (URL: %s, Error: %v)",
						backend.URL, serviceCfg.Name, healthCheckURL.String(), err)
				}
			}
		}
	}
}
