package plugin

import (
	"net/http"

	"gateway.example/go-gateway/internal/config"
)

// Middleware 是一个标准的 HTTP 中间件类型别名，为了代码清晰。
// 它接收一个 http.Handler 并返回一个新的 http.Handler。
type Middleware func(http.Handler) http.Handler

// Plugin 定义了所有网关插件必须实现的接口。
type Plugin interface {
	// Name 返回插件的唯一名称。
	// 这个名称必须与配置文件中 `plugins` 列表下的 `name` 字段完全匹配。
	// 例如："rateLimit", "jwtAuth", "cors"
	Name() string

	// CreateMiddleware 根据路由上为该插件提供的特定配置，
	// 创建一个中间件实例。
	// pluginConfig 是从 YAML 解析出的具体配置块，例如：
	// { "name": "rateLimit", "rule": "default-limit", "strategy": "ip" }
	CreateMiddleware(pluginConfig config.PluginSpec) (Middleware, error)
}
