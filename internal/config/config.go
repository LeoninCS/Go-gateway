// file: internal/config/config.go
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config 是整个应用的主配置结构体
type Config struct {
	Server   ServerConfig    `yaml:"server"`
	Services []ServiceConfig `yaml:"services"`
	Routes   []RouteConfig   `yaml:"routes"` // 新增 Routes
	JWT      JWTConfig       `yaml:"jwt"`
}

// ServerConfig 对应 yaml 中的 "server" 部分
type ServerConfig struct {
	Port string `yaml:"port"`
}

// ServiceConfig 对应 yaml 中 "services" 列表里的每个服务
// 结构已简化，只包含核心信息
type ServiceConfig struct {
	Name            string `yaml:"name"`
	URL             string `yaml:"url"`             // 对应单个 URL
	HealthCheckPath string `yaml:"healthCheckPath"` // 明确健康检查路径
}

// RouteConfig 对应 yaml 中 "routes" 列表里的每个路由规则
type RouteConfig struct {
	PathPrefix   string `yaml:"path_prefix"`
	ServiceName  string `yaml:"service_name"`
	AuthRequired bool   `yaml:"auth_required"`
}

type JWTConfig struct {
	SecretKey       string `yaml:"secret_key"`
	DurationMinutes int    `yaml:"duration_minutes"`
}

// Load 从指定路径加载配置文件
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}
