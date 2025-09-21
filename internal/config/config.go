// package config 定义了网关YAML配置的结构体和加载逻辑。
package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

// Config 是整个网关配置的根结构

type GatewayConfig struct {
	Server         ServerConfig             `yaml:"server"`
	HealthCheck    HealthCheckConfig        `yaml:"health_check"`
	Services       map[string]ServiceConfig `yaml:"services"`
	Routes         []*RouteConfig           `yaml:"routes"`
	RateLimiting   RateLimitingConfig       `yaml:"rate_limiting"`
	JWT            JWTConfig                `yaml:"jwt"`
	AuthService    AuthServiceConfig        `yaml:"auth_service"`
	CircuitBreaker CircuitBreakerConfig     `yaml:"circuit_breaker"`
}

// ServiceConfig 定义了一个可被路由的上游服务

type ServiceConfig struct {
	Name            string           `yaml:"name"`
	Instances       []InstanceConfig `yaml:"instances"`
	HealthCheckPath string           `yaml:"health_check_path"`
	LoadBalancer    string           `yaml:"load_balancer"`
}

// RouteConfig 定义了一条路由规则

type RouteConfig struct {
	PathPrefix       string       `yaml:"path_prefix,omitempty"`
	Path             string       `yaml:"path,omitempty"`
	ServiceName      string       `yaml:"service_name"`
	Plugins          []PluginSpec `yaml:"plugins,omitempty"`
	Methods          []string     `yaml:"methods,omitempty"`
	RequiresAuth     bool         `yaml:"requires_auth,omitempty"`
	HealthCheckScope string       `yaml:"health_check_scope,omitempty"`
}

// ServerConfig 定义服务器配置

type ServerConfig struct {
	Port string `yaml:"port"`
}

// HealthCheckConfig 定义健康检查配置

type HealthCheckConfig struct {
	Interval time.Duration `yaml:"interval"`
	Timeout  time.Duration `yaml:"timeout"`
}

// InstanceConfig 定义服务实例配置

type InstanceConfig struct {
	URL    string `yaml:"url"`
	Weight int    `yaml:"weight"`
}

// PluginSpec 定义插件配置

type PluginSpec map[string]interface{}

// RateLimitingConfig 定义限流配置

type RateLimitingConfig struct {
	Rules []RateLimiterRule `yaml:"rules"`
}

// RateLimiterRule 定义限流规则

type RateLimiterRule struct {
	Name        string              `yaml:"name"`
	Type        string              `yaml:"type"`
	TokenBucket TokenBucketSettings `yaml:"tokenBucket,omitempty"`
}

// TokenBucketSettings 定义令牌桶设置

type TokenBucketSettings struct {
	Capacity   int `yaml:"capacity"`
	RefillRate int `yaml:"refillRate"`
}

// JWTConfig 定义JWT配置

type JWTConfig struct {
	SecretKey       string `yaml:"secret_key"`
	DurationMinutes int    `yaml:"duration_minutes"`
}

// AuthServiceConfig 定义认证服务配置

type AuthServiceConfig struct {
	ValidateURL string `yaml:"validate_url"`
}

// CircuitBreakerConfig 定义断路器配置

type CircuitBreakerConfig struct {
	FailureThreshold int           `yaml:"failure_threshold"`
	SuccessThreshold int           `yaml:"success_threshold"`
	ResetTimeout     time.Duration `yaml:"reset_timeout"`
}

// Load 从指定路径加载配置文件

func Load(path string) (*GatewayConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件 '%s' 失败: %w", path, err)
	}

	var config GatewayConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析配置文件 '%s' 失败: %w", path, err)
	}

	return &config, nil
}
