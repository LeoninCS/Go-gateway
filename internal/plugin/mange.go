package plugin

import (
	"fmt"
	"log"

	"gateway.example/go-gateway/internal/config"
)

// Manager 是一个插件注册中心和工厂。
// 它在网关启动时被初始化，并持有所有可用的插件实例。
type Manager struct {
	plugins map[string]Plugin
}

// NewManager 创建一个新的插件管理器实例。
func NewManager() *Manager {
	return &Manager{
		plugins: make(map[string]Plugin),
	}
}

// Register 将一个插件实例注册到管理器中。
// 如果同名插件已存在，它会引发 panic，因为这是启动时的配置错误。
func (m *Manager) Register(p Plugin) {
	name := p.Name()
	if _, exists := m.plugins[name]; exists {
		log.Fatalf("[FATAL] 插件注册失败：名为 '%s' 的插件已存在。", name)
	}
	m.plugins[name] = p
	log.Printf("[INFO] 插件已注册: %s", name)
}

// CreateMiddleware 根据插件名称和特定配置创建一个中间件实例。
// 这是网关在处理每个请求时调用的核心方法。
func (m *Manager) CreateMiddleware(name string, spec config.PluginSpec) (Middleware, error) {
	// 1. 根据名称查找已注册的插件
	plugin, exists := m.plugins[name]
	if !exists {
		return nil, fmt.Errorf("未找到名为 '%s' 的插件", name)
	}

	// 2. 委托给具体的插件去创建中间件
	// 插件自己负责解析 spec (map[string]interface{}) 并验证其内容
	return plugin.CreateMiddleware(spec)
}
