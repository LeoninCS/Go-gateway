package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/health"
	"gateway.example/go-gateway/internal/loadbalancer"
	"gateway.example/go-gateway/internal/repository"
	"gateway.example/go-gateway/internal/server"
	authSvc "gateway.example/go-gateway/internal/service/auth"
	"github.com/golang-jwt/jwt/v5"
)

// Gateway 网关结构体，实现 http.Handler 接口
type Gateway struct {
	config        *config.Config
	lbFactory     *loadbalancer.LoadBalancerFactory
	healthChecker *health.HealthChecker
	authService   *authSvc.AuthService
}

// Server 封装 http.Server
type Server struct {
	httpServer *http.Server
}

// NewGateway 创建并初始化网关实例
func NewGateway(cfg *config.Config) *Gateway {
	// 初始化健康检查器
	healthChecker := health.NewHealthChecker(30 * time.Second)
	// 注册服务到健康检查器
	for _, service := range cfg.Services {
		instances := make([]string, 0)
		for _, instance := range service.Instances {
			instances = append(instances, instance.URL)
		}
		healthChecker.RegisterService(service.Name, instances, service.HealthCheckPath)
	}
	// 启动健康检查
	go healthChecker.Start()

	// 初始化负载均衡器工厂
	lbFactory := loadbalancer.NewLoadBalancerFactory()
	// 注册服务实例到负载均衡器
	for _, service := range cfg.Services {
		// 获取或创建负载均衡器
		lb := lbFactory.GetOrCreateLoadBalancer(service.Name, service.LoadBalancer)
		for _, inst := range service.Instances {
			// 注册每个实例
			lb.RegisterInstance(service.Name, &loadbalancer.ServiceInstance{
				URL:    inst.URL,
				Weight: inst.Weight,
				Alive:  true, // 默认标记为存活
			})
		}
	}

	// 初始化认证服务
	userRepo := repository.NewInMemoryUserRepository()
	authService := authSvc.NewAuthService(userRepo, cfg.JWT.SecretKey, cfg.JWT.DurationMinutes)
	return &Gateway{
		config:        cfg,
		lbFactory:     lbFactory,
		healthChecker: healthChecker,
		authService:   authService,
	}
}

// isJWTValid 检查JWT token是否有效
func (g *Gateway) isJWTValid(tokenString string) bool {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(g.config.JWT.SecretKey), nil
	})
	if err != nil {
		return false
	}
	return token.Valid
}

// ServeHTTP 实现 http.Handler 接口，处理所有请求
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 处理健康检查端点
	if r.URL.Path == "/healthz" {
		g.HealthCheckHandler(w, r)
		return
	}
	// 查找匹配的路由
	var matchedRoute *config.RouteConfig
	for _, route := range g.config.Routes {
		if strings.HasPrefix(r.URL.Path, route.PathPrefix) {
			matchedRoute = &route
			break
		}
	}
	if matchedRoute == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// 检查是否需要认证
	if matchedRoute.AuthRequired {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}
		// 提取Bearer token
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if !g.isJWTValid(tokenString) {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
	}
	// 获取负载均衡器
	lb := g.lbFactory.GetOrCreateLoadBalancer(
		matchedRoute.ServiceName,
		g.getServiceConfig(matchedRoute.ServiceName).LoadBalancer,
	)

	// **调试日志**
	log.Printf("matchedRoute: %v", matchedRoute)
	log.Printf("Instances for %s: %v", matchedRoute.ServiceName, lb.GetAllInstances(matchedRoute.ServiceName))

	// 获取目标服务实例
	instance, err := lb.GetNextInstance(matchedRoute.ServiceName)
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
	// 修改请求，去除路径前缀
	//r.URL.Path = strings.TrimPrefix(r.URL.Path, matchedRoute.PathPrefix)
	// 使用中间件跟踪请求连接数
	if lbType, ok := lb.(interface{ ReleaseConnection(string, string) }); ok {
		defer func() {
			lbType.ReleaseConnection(matchedRoute.ServiceName, instance.URL)
		}()
	}
	// 使用代理转发请求
	proxy.ServeHTTP(w, r)
}

// getServiceConfig 获取服务配置
func (g *Gateway) getServiceConfig(serviceName string) *config.ServiceConfig {
	for _, service := range g.config.Services {
		if service.Name == serviceName {
			return &service
		}
	}
	return nil
}

// HealthCheckHandler 处理健康检查请求
func (g *Gateway) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// 返回所有服务的健康状态
	response := make(map[string]map[string]bool)
	for serviceName, instances := range g.healthChecker.ServiceURLs {
		response[serviceName] = make(map[string]bool)
		for _, url := range instances {
			response[serviceName][url] = g.healthChecker.IsInstanceHealthy(serviceName, url)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}
	gateway := NewGateway(cfg)

	// 使用 NewServer 函数创建服务器实例
	server, err := server.NewServer(cfg.Server.Port, gateway)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}
	log.Printf("Gateway starting on port %s", cfg.Server.Port)

	// 启动服务器
	go func() {
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("failed to start gateway: %v", err)
		}
	}()

	// 等待优雅关闭信号
	server.GracefulShutdown()
}
