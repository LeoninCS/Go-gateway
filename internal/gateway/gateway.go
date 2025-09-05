// file: internal/gateway/gateway.go
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

type Gateway struct {
	Config        *config.Config
	Mux           *http.ServeMux
	HealthHandler *health.HealthHandler
}

func NewGateway(cfg *config.Config, healthHandler *health.HealthHandler) (*Gateway, error) {
	gw := &Gateway{
		Config:        cfg,
		Mux:           http.NewServeMux(),
		HealthHandler: healthHandler,
	}
	gw.registerRoutes()
	return gw, nil
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.Mux.ServeHTTP(w, r)
}

func (g *Gateway) findServiceByName(name string) *config.ServiceConfig {
	for i := range g.Config.Services {
		if g.Config.Services[i].Name == name {
			return &g.Config.Services[i]
		}
	}
	return nil
}

func (g *Gateway) registerRoutes() {
	g.Mux.HandleFunc("GET /healthz", g.HealthHandler.Healthz)

	jwtMiddleware := auth.Middleware(&g.Config.JWT)

	for i := range g.Config.Routes {
		routeCfg := &g.Config.Routes[i]

		serviceCfg := g.findServiceByName(routeCfg.ServiceName)
		if serviceCfg == nil {
			log.Printf("Service '%s' for route '%s' not found. Skipping route.", routeCfg.ServiceName, routeCfg.PathPrefix)
			continue
		}

		// **--- 这是核心修改点 ---**
		// 2. 为找到的服务创建后端列表和负载均衡器
		// 由于现在每个服务只有一个 URL，我们不再需要遍历 Endpoints
		var backends []*loadbalancer.Backend
		targetURL, err := url.Parse(serviceCfg.URL)
		if err != nil {
			log.Printf("Error parsing URL '%s' for service '%s': %v. Skipping route.", serviceCfg.URL, serviceCfg.Name, err)
			continue
		}

		proxy := httputil.NewSingleHostReverseProxy(targetURL)
		backends = append(backends, &loadbalancer.Backend{
			URL:          targetURL,
			ReverseProxy: proxy,
			Alive:        true,
		})
		// **--- 修改结束 ---**

		lb := loadbalancer.NewLoadBalancer(backends)
		go g.runHealthChecks(serviceCfg, backends)

		var finalHandler http.Handler = http.StripPrefix(routeCfg.PathPrefix, lb)

		if routeCfg.AuthRequired {
			finalHandler = jwtMiddleware(finalHandler)
			log.Printf("Applying JWT middleware to route: %s", routeCfg.PathPrefix)
		}

		g.Mux.Handle(routeCfg.PathPrefix+"/", finalHandler)

		log.Printf("Registered route: Path '%s/*' -> Service '%s'. Auth Required: %v",
			routeCfg.PathPrefix, routeCfg.ServiceName, routeCfg.AuthRequired)
	}
}

func (g *Gateway) runHealthChecks(serviceCfg *config.ServiceConfig, backends []*loadbalancer.Backend) {
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	healthCheckURL := serviceCfg.URL + serviceCfg.HealthCheckPath

	for range ticker.C {
		// 因为现在只有一个后端，所以我们直接检查它
		backend := backends[0]

		resp, err := http.Get(healthCheckURL)
		if err != nil || resp.StatusCode != http.StatusOK {
			if backend.IsAlive() {
				log.Printf("Backend for service '%s' at '%s' is DOWN", serviceCfg.Name, serviceCfg.URL)
				backend.SetAlive(false)
			}
		} else {
			if !backend.IsAlive() {
				log.Printf("Backend for service '%s' at '%s' is UP", serviceCfg.Name, serviceCfg.URL)
				backend.SetAlive(true)
			}
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
}
