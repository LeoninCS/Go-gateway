// package core 提供了网关的核心路由和代理功能。
package core

import (
	"log"
	"net/http"
	"strings"

	"gateway.example/go-gateway/internal/config"
)

// Router 负责解析HTTP请求并找到匹配的路由配置。
type Router struct {
	// routes 存储所有路由配置的指针切片
	routes []*config.RouteConfig
}

// NewRouter 创建并初始化一个新的路由器实例
func NewRouter(routes []*config.RouteConfig) *Router {
	log.Printf("核心组件: 路由器已初始化，共加载 %d 条路由规则。", len(routes))
	return &Router{
		routes: routes,
	}
}

// FindRoute 根据请求URL路径查找匹配的路由配置
func (ro *Router) FindRoute(r *http.Request) *config.RouteConfig {
	// 遍历所有路由配置，使用路径前缀进行匹配
	for _, route := range ro.routes {
		// 安全检查：确保路由配置不为空
		if route != nil && strings.HasPrefix(r.URL.Path, route.PathPrefix) {
			return route // 返回匹配的路由配置指针
		}
	}
	return nil // 没有找到匹配的路由
}
