// File: internal/config/config.go
package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

// h1--Config 对应整个 YAML 文件的顶层结构
type Config struct {
	// 字段名 Gateway 匹配 YAML 中的 'gateway:'
	Gateway  GatewayConfig   `yaml:"gateway"`
	Services []ServiceConfig `yaml:"services"`
	JWT      JWTConfig       `yaml:"jwt"`
	Database DatabaseConfig  `yaml:"database"`
}

// h2--GatewayConfig 对应 YAML 中的 'gateway' 部分
type GatewayConfig struct {
	Port string `yaml:"port"`
}

// h2--ServiceConfig 对应 'services' 数组中的每一个服务对象
type ServiceConfig struct {
	Name                  string `yaml:"name"`
	Path                  string `yaml:"path"`
	LoadBalancingStrategy string `yaml:"load_balancing_strategy"`
	// Endpoints 字段匹配 YAML 中的 'endpoints:'
	Endpoints []EndpointConfig `yaml:"endpoints"`
}

// h3--EndpointConfig 对应 'endpoints' 数组中的每个后端实例对象
type EndpointConfig struct {
	URL         string            `yaml:"url"`
	HealthCheck HealthCheckConfig `yaml:"health_check"`
}

// h3--HealthCheckConfig 对应 'health_check' 对象
type HealthCheckConfig struct {
	Path     string `yaml:"path"`
	Interval string `yaml:"interval"`
}

// h2--JWTConfig
type JWTConfig struct {
	SecretKey       string `yaml:"secret_key"`
	DurationMinutes int    `yaml:"duration_minutes"`
}

// h2--DatabaseConfig 对应 YAML 中的 'database' 部分
type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

// Load 函数从指定路径加载并解析 YAML 配置文件
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
