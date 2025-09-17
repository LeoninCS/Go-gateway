// file: internal/plugin/manager.go
package plugin

import (
	"fmt"
	"log"
	"net/http"

	"gateway.example/go-gateway/internal/config"
)

// Interface 定义了插件必须实现的接口
type Interface interface {
	// === 修改点 1: 将 GetName() 改为 Name() ===
	Name() string
	Execute(w http.ResponseWriter, r *http.Request, params config.PluginSpec) (continueChain bool, err error)
}

// Manager 负责管理和执行插件
type Manager struct {
	plugins map[string]Interface
}

// ... (NewManager, GetPlugin 保持不变) ...
func NewManager() *Manager {
	return &Manager{
		plugins: make(map[string]Interface),
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
	// === 修改点 2: 调用 p.Name() 而不是 p.GetName() ===
	name := p.Name()
	log.Printf("[插件管理器] 正在注册插件 '%s'", name)
	if _, exists := m.plugins[name]; exists {
		log.Printf("[插件管理器] 警告: 插件 '%s' 已存在，将被覆盖", name)
	}
	m.plugins[name] = p
}

// ... ExecuteChain 方法保持不变，它已经写对了 ...
func (m *Manager) ExecuteChain(w http.ResponseWriter, r *http.Request, pluginSpecs []config.PluginSpec) (bool, error) {
	// (此部分代码是正确的，无需修改)
	for _, spec := range pluginSpecs {
		pluginName, ok := spec["name"].(string)
		if !ok || pluginName == "" {
			log.Printf("[插件管理器] 错误: 插件配置缺少 'name' 字段或类型不正确: %v", spec)
			http.Error(w, "内部服务器错误: 插件配置错误", http.StatusInternalServerError)
			return false, fmt.Errorf("无效的插件配置: %v", spec)
		}
		plugin := m.GetPlugin(pluginName)
		if plugin == nil {
			log.Printf("[插件管理器] 错误: 未找到名为 '%s' 的已注册插件", pluginName)
			http.Error(w, "内部服务器错误: 插件未找到", http.StatusInternalServerError)
			return false, fmt.Errorf("插件 '%s' 未注册", pluginName)
		}
		log.Printf("[插件管理器] 执行插件: %s", pluginName)
		continueChain, err := plugin.Execute(w, r, spec)
		if err != nil {
			log.Printf("[插件管理器] 错误: 插件 '%s' 执行时返回内部错误: %v", pluginName, err)
			return false, err
		}
		if !continueChain {
			log.Printf("[插件管理器] 信息: 插件 '%s' 中断了请求链。", pluginName)
			return false, nil
		}
	}
	return true, nil
}
