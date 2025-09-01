package gateway

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"gateway.example/go-gateway/internal/auth"
	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/health"
	"gateway.example/go-gateway/internal/loadbalancer"
)

// Gateway 是我们的核心网关处理器
type Gateway struct {
	Config        *config.Config        // 网关配置
	Mux           *http.ServeMux        // HTTP 请求多路复用器
	AuthHandler   *auth.AuthHandler     // 认证处理器
	HealthHandler *health.HealthHandler // 健康检查处理器
}

// NewGateway 创建一个新的 Gateway 实例
func NewGateway(cfg *config.Config, authHandler *auth.AuthHandler, healthHandler *health.HealthHandler) (*Gateway, error) {
	gw := &Gateway{
		Config:        cfg,
		Mux:           http.NewServeMux(),
		AuthHandler:   authHandler,
		HealthHandler: healthHandler, // 初始化 HealthHandler
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
	g.Mux.HandleFunc("POST /register", g.AuthHandler.Register)
	g.Mux.HandleFunc("POST /unregister", g.AuthHandler.Unregister)
	g.Mux.HandleFunc("POST /login", g.AuthHandler.Login)
	g.Mux.HandleFunc("POST /logout", g.AuthHandler.Logout)
	g.Mux.HandleFunc("POST /send_verification_code", g.AuthHandler.SendVerificationCode)
	g.Mux.HandleFunc("POST /reset_password", g.AuthHandler.ResetPassword)
	g.Mux.HandleFunc("POST /change_password", g.AuthHandler.ChangePassword)
	g.Mux.HandleFunc("GET /healthz", g.HealthHandler.Healthz)

	// ===================================================================
	// 2. 注册受保护的路由 (Protected Routes)
	// ===================================================================
	jwtMiddleware := auth.Middleware(&g.Config.JWT)

	for i := range g.Config.Services {
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

		// 启动健康检查
		go g.runHealthChecks(serviceCfg, backends)

		// 创建受保护的handler
		protectedHandler := jwtMiddleware(http.StripPrefix(serviceCfg.Path, lb))

		// 注册路由，注意这里使用了方法+路径的模式匹配
		g.Mux.Handle(serviceCfg.Path, protectedHandler)

		log.Printf("Registered PROTECTED service '%s' at path '%s' with %d backends. Strategy: '%s'",
			serviceCfg.Name, serviceCfg.Path, len(backends), serviceCfg.LoadBalancingStrategy)
	}
}

// runHealthChecks 在后台为指定服务的所有后端运行健康检查
func (g *Gateway) runHealthChecks(serviceCfg *config.ServiceConfig, backends []*loadbalancer.Backend) {
	if len(serviceCfg.Endpoints) == 0 {
		return
	}

	// 使用配置中的健康检查间隔，或者默认为30秒
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
		for i, backend := range backends {
			if i >= len(serviceCfg.Endpoints) {
				continue
			}

			endpointCfg := serviceCfg.Endpoints[i]
			healthCheckURL := *backend.URL
			healthCheckURL.Path = endpointCfg.HealthCheck.Path

			client := http.Client{Timeout: 3 * time.Second}
			resp, err := client.Get(healthCheckURL.String())

			isAlive := err == nil && resp != nil && resp.StatusCode < 500
			if resp != nil {
				resp.Body.Close()
			}

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
