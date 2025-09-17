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

// Gateway 是整个API网关的核心引擎。
type Gateway struct {
	config        *config.GatewayConfig
	router        *Router
	proxy         *Proxy
	healthChecker *health.HealthChecker
	pluginManager *plugin.Manager
	rateLimitSvc  svc_ratelimit.Service
}

// NewGateway 创建并初始化网关的所有组件。
func NewGateway(cfg *config.GatewayConfig) (*Gateway, error) {
	// ... (section 1. a. b. unchanged) ...
	rateLimitSvc, err := svc_ratelimit.NewService(cfg.RateLimiting)
	if err != nil {
		return nil, fmt.Errorf("初始化限流服务失败: %w", err)
	}
	log.Println("服务层: 限流服务已成功初始化。")

	// --- 核心组件初始化 ---
	lbFactory := loadbalancer.NewLoadBalancerFactory()
	log.Println("核心组件: 负载均衡器工厂已创建。")

	// ★ 修正 1: 对齐 HealthChecker 的构造函数，传入 timeout 和 interval。
	healthChecker := health.NewHealthChecker(cfg.HealthCheck.Timeout, cfg.HealthCheck.Interval)
	log.Println("核心组件: 健康检查器已创建。")

	// ★ 修正 2: 注册服务实例到健康检查器和负载均衡器。
	//    - 由于 cfg.Services 的值现在是 `*ServiceConfig`，循环变量 `serviceCfg` 本身就是指针。
	//    - 移除了无用的 `serviceMap`，直接使用 `cfg.Services`。
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

	// ★ 修正 3: 必须在后台 goroutine 中启动健康检查，否则会阻塞网关启动。
	go healthChecker.Start()

	proxy := NewProxy(lbFactory, healthChecker)
	log.Println("核心组件: 反向代理已创建并注入依赖。")

	// --- 插件初始化 ---
	pluginManager := plugin.NewManager()

	rateLimitPlugin := pl_ratelimit.NewPlugin(rateLimitSvc)
	pluginManager.Register(rateLimitPlugin)
	log.Println("插件: 'rateLimit' 已成功注册。")

	if cfg.AuthService.ValidateURL != "" {
		authPlugin, err := pl_auth.NewPlugin(lbFactory, healthChecker, "auth-service")
		if err != nil {
			return nil, fmt.Errorf("初始化认证插件失败: %w", err)
		}
		pluginManager.Register(authPlugin)
		log.Println("插件: 'auth' 已成功注册。")
	}

	// --- 组装 Gateway ---
	gw := &Gateway{
		config:        cfg,
		router:        NewRouter(cfg.Routes), // cfg.Routes 已经是 []*config.RouteConfig
		proxy:         proxy,
		healthChecker: healthChecker,
		pluginManager: pluginManager,
		rateLimitSvc:  rateLimitSvc,
	}

	log.Println("网关核心已成功初始化并准备就绪。")
	return gw, nil
}

// ServeHTTP 是网关处理所有入口请求的处理器。
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 1. 查找匹配的路由
	route := g.router.FindRoute(r)
	if route == nil {
		log.Printf("[网关核心] 请求 %s %s 未匹配到任何路由", r.Method, r.URL.Path)
		http.Error(w, "服务未找到", http.StatusNotFound)
		return
	}

	// ★ 修正: 对于healthz路由的特殊处理，跳过服务查找
	if route.ServiceName == "all-services" {
		// 直接调用HealthCheckHandler处理健康检查请求
		g.HealthCheckHandler(w, r)
		return
	}

	// ★ 修正: 正确地从 map[string]ServiceConfig 中查找服务
	service, exists := g.config.Services[route.ServiceName]
	if !exists {
		log.Printf("[网关核心] 请求 %s %s 匹配到路由 '%s'，但服务 '%s' 未在配置中定义", r.Method, r.URL.Path, route.PathPrefix, route.ServiceName)
		http.Error(w, "服务配置错误", http.StatusInternalServerError)
		return
	}
	log.Printf("[网关核心] 请求 %s %s 匹配到路由 -> 服务: %s", r.Method, r.URL.Path, service.Name)

	// 2. 执行插件链
	continueChain, err := g.pluginManager.ExecuteChain(w, r, route.Plugins)
	if err != nil {
		// 插件链内部已经处理了错误响应，这里只记录日志
		log.Printf("[网关核心] 错误: 插件链执行因内部错误而中断: %v", err)
		return
	}
	if !continueChain {
		log.Printf("[网关核心] 信息: 插件链中断请求，处理结束。")
		return
	}

	// 3. 将请求交给反向代理
	g.proxy.ServeHTTP(w, r, route, &service) // 注意这里需要传递指针
}

// HealthCheckHandler 提供一个API端点，返回所有服务的健康状态。
func (g *Gateway) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// 获取路由配置中的健康检查范围设置
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
			// 处理单个服务检测，包括all-services特殊值
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
		// 处理单个服务检测，包括all-services特殊值
		if route.ServiceName == "all-services" {
			response = g.healthChecker.GetAllStatuses()
		} else if _, exists := g.config.Services[route.ServiceName]; exists {
			response = map[string]interface{}{
				route.ServiceName: g.healthChecker.GetServiceStatus(route.ServiceName),
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("[网关核心] 错误: 写入健康检查响应失败: %v", err)
	}
}

// Shutdown 优雅地关闭网关的所有组件。
func (g *Gateway) Shutdown() {
	log.Println("网关正在关闭...")
	g.healthChecker.Shutdown() // 对齐函数名
	if err := g.rateLimitSvc.Close(); err != nil {
		log.Printf("关闭限流服务时出错: %v", err)
	}
	log.Println("网关已成功关闭。")
}
