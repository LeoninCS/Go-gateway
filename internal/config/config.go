// internal/config/config.go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// Config 网关配置
type Config struct {
	Server   ServerConfig    `yaml:"server"`
	Services []ServiceConfig `yaml:"services"`
	Routes   []RouteConfig   `yaml:"routes"`
	JWT      JWTConfig       `yaml:"jwt"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port string `yaml:"port"`
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	Name            string           `yaml:"name"`
	Instances       []InstanceConfig `yaml:"instances"`
	HealthCheckPath string           `yaml:"health_check_path"`
	LoadBalancer    string           `yaml:"load_balancer"` // 负载均衡策略
}

// InstanceConfig 实例配置
type InstanceConfig struct {
	URL    string `yaml:"url"`
	Weight int    `yaml:"weight"`
}

// RouteConfig 路由配置
type RouteConfig struct {
	PathPrefix   string `yaml:"path_prefix"`
	ServiceName  string `yaml:"service_name"`
	AuthRequired bool   `yaml:"auth_required"`
}

// JWTConfig JWT配置
type JWTConfig struct {
	SecretKey       string `yaml:"secret_key"`
	DurationMinutes int    `yaml:"duration_minutes"`
}

// Load 加载配置文件
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &config, nil
}
