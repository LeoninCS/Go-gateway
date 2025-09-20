package core

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/core/health"
	"gateway.example/go-gateway/internal/core/loadbalancer"
	"gateway.example/go-gateway/internal/plugin"
	pl_auth "gateway.example/go-gateway/internal/plugin/auth"
	pl_ratelimit "gateway.example/go-gateway/internal/plugin/ratelimit"
	svc_ratelimit "gateway.example/go-gateway/internal/service/ratelimit"
)

// Gateway API网关核心引擎
// 负责请求路由、负载均衡、健康检查和插件管理
type Gateway struct {
	config        *config.GatewayConfig // 网关配置
	router        *Router               // 路由匹配器
	proxy         *Proxy                // 反向代理
	healthChecker *health.HealthChecker // 健康检查器
	pluginManager *plugin.Manager       // 插件管理器
	rateLimitSvc  svc_ratelimit.Service // 限流服务
}

// NewGateway 创建网关实例并初始化所有组件
func NewGateway(cfg *config.GatewayConfig) (*Gateway, error) {

	// 核心组件初始化
	lbFactory := loadbalancer.NewLoadBalancerFactory()
	log.Println("核心组件: 负载均衡器工厂已创建。")

	// 健康检查器
	healthChecker := health.NewHealthChecker(cfg.HealthCheck.Timeout, cfg.HealthCheck.Interval)
	log.Println("核心组件: 健康检查器已创建。")

	// 注册服务实例到健康检查器和负载均衡器
	for _, serviceCfg := range cfg.Services {
		var instanceURLs []string
		for _, inst := range serviceCfg.Instances {
			instanceURLs = append(instanceURLs, inst.URL)
		}

		healthChecker.RegisterService(serviceCfg.Name, instanceURLs, serviceCfg.HealthCheckPath)

		lb := lbFactory.GetOrCreateLoadBalancer(serviceCfg.Name, serviceCfg.LoadBalancer)
		for _, inst := range serviceCfg.Instances {
			lb.RegisterInstance(serviceCfg.Name, &loadbalancer.ServiceInstance{
				URL:    inst.URL,
				Weight: inst.Weight,
				Alive:  true, // 初始状态默认为健康
			})
		}
		log.Printf("服务发现: 服务 '%s' 的 %d 个实例已注册。", serviceCfg.Name, len(instanceURLs))
	}

	// 启动健康检查
	go healthChecker.Start()

	// 创建反向代理
	proxy := NewProxy(lbFactory, healthChecker)
	log.Println("核心组件: 反向代理已创建并注入依赖。")

	// 插件初始化
	pluginManager := plugin.NewManager()

	// 限流插件
	rateLimitSvc, err := svc_ratelimit.NewService(cfg.RateLimiting)
	if err != nil {
		return nil, fmt.Errorf("初始化限流服务失败: %w", err)
	}
	log.Println("服务层: 限流服务已成功初始化。")

	rateLimitPlugin := pl_ratelimit.NewPlugin(rateLimitSvc)
	pluginManager.Register(rateLimitPlugin)
	log.Println("插件: 'rateLimit' 已成功注册。")

	// 认证插件（如果配置了认证服务）
	if cfg.AuthService.ValidateURL != "" {
		authPlugin, err := pl_auth.NewPlugin(lbFactory, healthChecker, "auth-service")
		if err != nil {
			return nil, fmt.Errorf("初始化认证插件失败: %w", err)
		}
		pluginManager.Register(authPlugin)
		log.Println("插件: 'auth' 已成功注册。")
	}

	// 组装网关实例
	gw := &Gateway{
		config:        cfg,
		router:        NewRouter(cfg.Routes),
		proxy:         proxy,
		healthChecker: healthChecker,
		pluginManager: pluginManager,
		rateLimitSvc:  rateLimitSvc,
	}

	log.Println("网关核心已成功初始化并准备就绪。")
	return gw, nil
}

// ServeHTTP 网关请求处理入口
// 1. 路由匹配 → 2. 插件链执行 → 3. 反向代理转发
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 查找匹配的路由
	route := g.router.FindRoute(r)
	if route == nil {
		log.Printf("[网关核心] 请求 %s %s 未匹配到任何路由", r.Method, r.URL.Path)
		http.Error(w, "服务未找到", http.StatusNotFound)
		return
	}

	// 健康检查路由特殊处理
	if route.ServiceName == "all-services" {
		g.HealthCheckHandler(w, r)
		return
	}

	// 查找对应服务
	service, exists := g.config.Services[route.ServiceName]
	if !exists {
		log.Printf("[网关核心] 请求 %s %s 匹配到路由 '%s'，但服务 '%s' 未在配置中定义", r.Method, r.URL.Path, route.PathPrefix, route.ServiceName)
		http.Error(w, "服务配置错误", http.StatusInternalServerError)
		return
	}
	log.Printf("[网关核心] 请求 %s %s 匹配到路由 -> 服务: %s", r.Method, r.URL.Path, service.Name)

	// 执行插件链
	continueChain, err := g.pluginManager.ExecuteChain(w, r, route.Plugins)
	if err != nil {
		log.Printf("[网关核心] 错误: 插件链执行因内部错误而中断: %v", err)
		return
	}
	if !continueChain {
		log.Printf("[网关核心] 信息: 插件链中断请求，处理结束。")
		return
	}

	// 反向代理转发请求
	g.proxy.ServeHTTP(w, r, route, &service)
}

// HealthCheckHandler 健康检查API端点
// 返回所有服务的健康状态
func (g *Gateway) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// 获取路由配置
	route := g.router.FindRoute(r)
	if route == nil {
		http.Error(w, "路由未找到", http.StatusNotFound)
		return
	}

	// 处理健康检查范围逻辑
	var response interface{}
	if route.HealthCheckScope == "auto" {
		// 根据端口自动选择检测范围
		port := strings.Split(r.Host, ":")[1]
		if port == "8080" {
			response = g.healthChecker.GetAllStatuses()
		} else {
			// 处理单个服务检测
			if route.ServiceName == "all-services" {
				response = g.healthChecker.GetAllStatuses()
			} else if _, exists := g.config.Services[route.ServiceName]; exists {
				response = map[string]interface{}{
					route.ServiceName: g.healthChecker.GetServiceStatus(route.ServiceName),
				}
			}
		}
	} else if route.HealthCheckScope == "all-services" {
		response = g.healthChecker.GetAllStatuses()
	} else {
		// 处理单个服务检测
		if route.ServiceName == "all-services" {
			response = g.healthChecker.GetAllStatuses()
		} else if _, exists := g.config.Services[route.ServiceName]; exists {
			response = map[string]interface{}{
				route.ServiceName: g.healthChecker.GetServiceStatus(route.ServiceName),
			}
		}
	}

	// 返回JSON响应
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("[网关核心] 错误: 写入健康检查响应失败: %v", err)
	}
}

// Shutdown 优雅关闭网关
// 停止健康检查和限流服务
func (g *Gateway) Shutdown() {
	log.Println("网关正在关闭...")
	g.healthChecker.Shutdown()
	if err := g.rateLimitSvc.Close(); err != nil {
		log.Printf("关闭限流服务时出错: %v", err)
	}
	log.Println("网关已成功关闭。")
}
