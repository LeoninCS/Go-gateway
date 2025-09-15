// internal/core/gateway.go
package core

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/core/health"
	"gateway.example/go-gateway/internal/core/loadbalancer"
	"gateway.example/go-gateway/internal/plugin"
	pl_ratelimit "gateway.example/go-gateway/internal/plugin/ratelimit"
	"gateway.example/go-gateway/internal/repository"
	"gateway.example/go-gateway/internal/service/auth"
	svcr "gateway.example/go-gateway/internal/service/ratelimit"
)

// Gateway 网关结构体
type Gateway struct {
	config        *config.Config
	router        *Router
	proxy         *Proxy
	healthChecker *health.HealthChecker
	authService   *auth.AuthService
	pluginManager *plugin.Manager // 修改点: 引入插件管理器
}

// NewGateway 创建并初始化网关实例
func NewGateway(cfg *config.Config, rateLimitSvc svcr.Service) *Gateway {
	// 初始化健康检查器 (逻辑不变)
	healthChecker := health.NewHealthChecker(30 * time.Second)
	for _, service := range cfg.Services {
		instances := make([]string, 0)
		for _, instance := range service.Instances {
			instances = append(instances, instance.URL)
		}
		healthChecker.RegisterService(service.Name, instances, service.HealthCheckPath)
	}
	go healthChecker.Start()

	// 初始化负载均衡器工厂 (逻辑不变)
	lbFactory := loadbalancer.NewLoadBalancerFactory()
	for _, service := range cfg.Services {
		lb := lbFactory.GetOrCreateLoadBalancer(service.Name, service.LoadBalancer)
		for _, inst := range service.Instances {
			lb.RegisterInstance(service.Name, &loadbalancer.ServiceInstance{
				URL:    inst.URL,
				Weight: inst.Weight,
				Alive:  true,
			})
		}
	}

	// 初始化认证服务 (逻辑不变)
	userRepo := repository.NewInMemoryUserRepository()
	authService := auth.NewAuthService(userRepo, cfg.JWT.SecretKey, cfg.JWT.DurationMinutes)

	// --- 新增: 初始化插件系统 ---
	pluginManager := plugin.NewManager()

	// 初始化并注册限流插件
	rateLimitPlugin, err := pl_ratelimit.NewPlugin(rateLimitSvc)
	if err != nil {
		log.Fatalf("无法创建限流插件: %v", err)
	}
	pluginManager.Register(rateLimitPlugin)
	log.Println("限流插件已成功注册。")

	// 未来可以在这里注册其他插件...

	// 创建并返回 Gateway 实例
	return &Gateway{
		config:        cfg,
		router:        NewRouter(cfg.Routes),
		proxy:         NewProxy(lbFactory),
		healthChecker: healthChecker,
		authService:   authService,
		pluginManager: pluginManager, // 使用插件管理器
	}
}

// isJWTValid 检查JWT token是否有效 (逻辑不变)
func (g *Gateway) isJWTValid(tokenString string) bool {
	return g.authService.ValidateToken(tokenString)
}

// ServeHTTP 实现 http.Handler 接口，协调整个请求处理流程
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. 处理特殊端点 (逻辑不变)
	if r.URL.Path == "/healthz" {
		g.HealthCheckHandler(w, r)
		return
	}

	// 2. 路由匹配 (逻辑不变)
	matchedRoute := g.router.FindRoute(r)
	if matchedRoute == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// --- 核心修改点: 动态构建并应用中间件链 ---
	var finalHandler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 最终的处理逻辑：获取服务配置并执行代理
		serviceConfig := g.getServiceConfig(matchedRoute.ServiceName)
		if serviceConfig == nil {
			http.Error(w, fmt.Sprintf("Service configuration for '%s' not found", matchedRoute.ServiceName), http.StatusServiceUnavailable)
			return
		}
		g.proxy.ServeHTTP(w, r, matchedRoute, serviceConfig) // 委托给 Proxy 处理
	})

	// 3. 应用认证中间件 (如果需要)
	if matchedRoute.AuthRequired {
		finalHandler = g.authMiddleware(finalHandler)
	}

	// 4. 应用插件中间件 (例如限流)
	// 这个循环会从后往前包裹 handler，确保插件按配置顺序执行
	for i := len(matchedRoute.Plugins) - 1; i >= 0; i-- {
		pluginSpec := matchedRoute.Plugins[i]

		// 从插件配置中获取插件名称
		pluginName, ok := pluginSpec["name"].(string)
		if !ok {
			log.Printf("[ERROR] 路由 '%s' 的插件配置缺少 'name' 字段", matchedRoute.PathPrefix)
			continue // 跳过这个无效的插件配置
		}

		// 从插件管理器创建中间件
		middleware, err := g.pluginManager.CreateMiddleware(pluginName, pluginSpec)
		if err != nil {
			log.Printf("[ERROR] 无法为路由 '%s' 创建插件 '%s' 的中间件: %v", matchedRoute.PathPrefix, pluginName, err)
			// 根据策略，可以选择返回 500 错误或跳过该插件
			http.Error(w, "Internal Server Error during middleware creation", http.StatusInternalServerError)
			return
		}

		// 将当前 handler 用新的中间件包裹起来
		finalHandler = middleware(finalHandler)
	}

	// 5. 执行最终形成的、包含所有中间件的处理链
	finalHandler.ServeHTTP(w, r)
}

// authMiddleware 是一个简单的认证中间件 (从旧的ServeHTTP中提取)
func (g *Gateway) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if !g.isJWTValid(tokenString) {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		// 认证通过，调用下一个 handler
		next.ServeHTTP(w, r)
	})
}

// getServiceConfig 获取服务配置 (逻辑不变)
func (g *Gateway) getServiceConfig(serviceName string) *config.ServiceConfig {
	for i, service := range g.config.Services {
		if service.Name == serviceName {
			return &g.config.Services[i]
		}
	}
	return nil
}

// HealthCheckHandler 处理健康检查请求 (逻辑不变)
func (g *Gateway) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	response := make(map[string]map[string]bool)
	for serviceName, urls := range g.healthChecker.ServiceURLs {
		response[serviceName] = make(map[string]bool)
		for _, url := range urls {
			response[serviceName][url] = g.healthChecker.IsInstanceHealthy(serviceName, url)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Close 关闭网关资源 (逻辑不变, 但注意 rateLimiter 已不存在)
func (g *Gateway) Close() error {
	log.Println("Closing Gateway resources...")
	// 这里可以添加未来其他需要关闭的资源
	// g.pluginManager.CloseAll() etc.
	log.Println("网关资源已成功关闭。")
	return nil
}
