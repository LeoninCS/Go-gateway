// package core 提供了网关的核心路由和代理功能。
package core

import (
	"log"
	"net/http"
	"strings"

	"gateway.example/go-gateway/internal/config"
)

// Router 负责解析请求并找到匹配的路由。
type Router struct {
	// ★ 修正 1: 字段类型从值切片 []config.RouteConfig 更改为指针切片 []*config.RouteConfig。
	// 这与 config 包的更改保持一致，避免了在路由匹配过程中对大型配置对象的拷贝。
	routes []*config.RouteConfig
}

// NewRouter 创建一个新的 Router 实例。
// ★ 修正 2: 参数类型同步更改为 []*config.RouteConfig，以接收来自配置加载器的正确类型。
func NewRouter(routes []*config.RouteConfig) *Router {
	log.Printf("核心组件: 路由器已初始化，共加载 %d 条路由规则。", len(routes))
	return &Router{
		routes: routes,
	}
}

// FindRoute 遍历已配置的路由，根据请求的URL路径前缀找到第一个匹配的路由。
// 返回匹配的路由配置指针，如果未找到则返回 nil。
func (ro *Router) FindRoute(r *http.Request) *config.RouteConfig {
	// ★ 修正 3: 循环和返回逻辑简化。
	// 由于 ro.routes 现在是指针切片，`range` 循环直接返回我们需要的 *config.RouteConfig 指针。
	// 不再需要使用索引 `i` 来取地址 `&ro.routes[i]`，代码更简洁、意图更清晰。
	for _, route := range ro.routes {
		// 确保 route 不为 nil，增加代码健壮性
		if route != nil && strings.HasPrefix(r.URL.Path, route.PathPrefix) {
			return route // 直接返回指针
		}
	}
	return nil
}
