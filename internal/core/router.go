package core

import (
	"net/http"
	"strings"

	"gateway.example/go-gateway/internal/config"
)

// Router 负责解析请求并找到匹配的路由。
type Router struct {
	routes []config.RouteConfig
}

// NewRouter 创建一个新的 Router 实例。
func NewRouter(routes []config.RouteConfig) *Router {
	return &Router{
		routes: routes,
	}
}

// FindRoute 遍历已配置的路由，根据请求的URL路径前缀找到第一个匹配的路由。
// 返回匹配的路由配置指针，如果未找到则返回 nil。
func (ro *Router) FindRoute(r *http.Request) *config.RouteConfig {
	for i, route := range ro.routes {
		if strings.HasPrefix(r.URL.Path, route.PathPrefix) {
			// 返回指向原始配置切片中元素的指针，以确保任何修改都能反映到原始配置上
			return &ro.routes[i]
		}
	}
	return nil
}
