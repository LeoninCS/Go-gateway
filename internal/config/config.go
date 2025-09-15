// internal/config/config.go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v2"
)

// Config 是整个网关的配置根结构
type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Services     []ServiceConfig    `yaml:"services"`
	Routes       []RouteConfig      `yaml:"routes"`
	JWT          JWTConfig          `yaml:"jwt"`
	RateLimiting RateLimitingConfig `yaml:"rate_limiting"` // 修改点：引用新的限流配置结构
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Port string `yaml:"port"`
}

// ServiceConfig 后端服务配置
type ServiceConfig struct {
	Name            string           `yaml:"name"`
	Instances       []InstanceConfig `yaml:"instances"`
	HealthCheckPath string           `yaml:"health_check_path"`
	LoadBalancer    string           `yaml:"load_balancer"`
}

// InstanceConfig 服务实例配置
type InstanceConfig struct {
	URL    string `yaml:"url"`
	Weight int    `yaml:"weight"`
}

// RouteConfig 路由配置
type RouteConfig struct {
	PathPrefix   string       `yaml:"path_prefix"`
	ServiceName  string       `yaml:"service_name"`
	AuthRequired bool         `yaml:"auth_required"`
	Plugins      []PluginSpec `yaml:"plugins"` // 修改点：使用通用的插件配置
}

// PluginSpec 是插件的通用配置结构
// UnmarshalYAML 会被自动调用，将 YAML map 转换为此类型
type PluginSpec map[string]interface{}

// JWTConfig JWT配置
type JWTConfig struct {
	SecretKey       string `yaml:"secret_key"`
	DurationMinutes int    `yaml:"duration_minutes"`
}

// --- 以下是为新的限流架构重新设计的配置结构 ---

// RateLimitingConfig 是限流功能的总配置
type RateLimitingConfig struct {
	Rules []RateLimiterRule `yaml:"rules"` // 定义一个规则列表
}

// RateLimiterRule 定义了单条可被引用的限流规则
type RateLimiterRule struct {
	Name        string              `yaml:"name"`        // 规则的唯一名称, 例如 "default-ip-limit"
	Type        string              `yaml:"type"`        // 限流器类型, 例如 "memory_token_bucket"
	TokenBucket TokenBucketSettings `yaml:"tokenBucket"` // 令牌桶算法的特定配置
}

// TokenBucketSettings 定义了令牌桶的参数
type TokenBucketSettings struct {
	Capacity   int `yaml:"capacity"`   // 桶容量
	RefillRate int `yaml:"refillRate"` // 每秒填充速率
}

// --- 加载函数保持不变 ---

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
