// file: internal/plugin/manager.go
package plugin

import (
	"context"
	"fmt"
	"net/http"

	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/pkg/logger"
)

// Interface 定义了插件必须实现的接口
type Interface interface {
	Name() string
	Execute(w http.ResponseWriter, r *http.Request, params config.PluginSpec) (continueChain bool, err error)
}

// Manager 负责管理和执行插件
type Manager struct {
	plugins map[string]Interface
	log     logger.Logger
}

func NewManager() *Manager {
	log, err := logger.DefaultNew()
	ctx := context.Background()
	if err != nil {
		log.Fatal(ctx, "[插件管理器] 致命错误: 无法初始化日志记录器", "error", err)
	}
	return &Manager{
		plugins: make(map[string]Interface),
		log:     log,
	}
}

func (m *Manager) GetLimiter(ruleName, key string) interface{} {
	return nil
}

func (m *Manager) GetPlugin(name string) Interface {
	return m.plugins[name]
}

// Register 注册一个插件
func (m *Manager) Register(p Interface) {
	ctx := context.Background()
	name := p.Name()

	m.log.Info(ctx, "[插件管理器] 正在注册插件 '%s'", name,
		"plugin_name", name,
		"action", "register")

	if _, exists := m.plugins[name]; exists {
		m.log.Warn(ctx, "[插件管理器] 警告: 插件 '%s' 已存在，将被覆盖", name,
			"plugin_name", name,
			"action", "overwrite")
	}
	m.plugins[name] = p
}

// ExecuteChain 执行插件链
func (m *Manager) ExecuteChain(w http.ResponseWriter, r *http.Request, pluginSpecs []config.PluginSpec) (bool, error) {
	ctx := r.Context()

	for _, spec := range pluginSpecs {
		pluginName, ok := spec["name"].(string)
		if !ok || pluginName == "" {
			m.log.Error(ctx, "[插件管理器] 错误: 插件配置缺少 'name' 字段或类型不正确: %v", spec,
				"spec", spec,
				"action", "config_error")
			http.Error(w, "内部服务器错误: 插件配置错误", http.StatusInternalServerError)
			return false, fmt.Errorf("无效的插件配置: %v", spec)
		}

		plugin := m.GetPlugin(pluginName)
		if plugin == nil {
			m.log.Error(ctx, "[插件管理器] 错误: 未找到名为 '%s' 的已注册插件", pluginName,
				"plugin_name", pluginName,
				"action", "plugin_not_found")
			http.Error(w, "内部服务器错误: 插件未找到", http.StatusInternalServerError)
			return false, fmt.Errorf("插件 '%s' 未注册", pluginName)
		}

		m.log.Info(ctx, "[插件管理器] 执行插件: %s", pluginName,
			"plugin_name", pluginName,
			"action", "execute")

		continueChain, err := plugin.Execute(w, r, spec)
		if err != nil {
			m.log.Error(ctx, "[插件管理器] 错误: 插件 '%s' 执行时返回内部错误: %v", pluginName, err,
				"plugin_name", pluginName,
				"error", err.Error(),
				"action", "execute_error")
			return false, err
		}

		if !continueChain {
			m.log.Info(ctx, "[插件管理器] 信息: 插件 '%s' 中断了请求链。", pluginName,
				"plugin_name", pluginName,
				"action", "chain_interrupted")
			return false, nil
		}
	}

	return true, nil
}
