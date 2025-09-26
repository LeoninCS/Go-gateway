// 一种日志配置选项
package logger

// Options 日志配置选项
type Options struct {
	Level            string          `yaml:"level"`             // 日志级别
	Format           string          `yaml:"format"`            // 日志格式(json/console)
	OutputPaths      []string        `yaml:"output_paths"`      // 输出路径列表
	ErrorPaths       []string        `yaml:"error_paths"`       // 错误日志输出路径列表
	EnableCaller     bool            `yaml:"enable_caller"`     // 是否启用调用者信息
	EnableStacktrace bool            `yaml:"enable_stacktrace"` // 是否启用堆栈跟踪
	StacktraceLevel  string          `yaml:"stacktrace_level"`  // 堆栈跟踪级别
	Rotation         RotationOptions `yaml:"rotation"`          // 日志滚动配置
}

// RotationOptions 日志轮转配置选项
type RotationOptions struct {
	Enabled bool   `yaml:"enabled"` // 是否启用日志轮转
	Policy  string `yaml:"policy"`  // 轮转策略(time/size/size_and_time)
	// 基于时间的轮转配置
	TimeInterval string `yaml:"time_interval"` // 轮转时间间隔(如"minute"表示每分钟，"hour"表示每小时，"24h"表示每天轮转)
	// 基于大小的轮转配置
	MaxSize    int  `yaml:"max_size"`    // 日志文件最大大小(MB)
	MaxBackups int  `yaml:"max_backups"` // 保留的旧日志文件最大数量
	MaxAge     int  `yaml:"max_age"`     // 日志文件最大保存时间(单位根据TimeInterval决定：分钟或天)
	Compress   bool `yaml:"compress"`    // 是否压缩旧日志文件
}

// Option 函数类型，用于修改Options
type Option func(*Options)

// WithLevel 创建配置日志级别的Option
func WithLevel(level string) Option {
	return func(o *Options) {
		o.Level = level
	}
}

// WithFormat 创建配置日志格式的Option
func WithFormat(format string) Option {
	return func(o *Options) {
		o.Format = format
	}
}

// WithOutputPaths 创建配置输出路径的Option
func WithOutputPaths(paths []string) Option {
	return func(o *Options) {
		o.OutputPaths = paths
	}
}

// WithErrorPaths 创建配置错误输出路径的Option
func WithErrorPaths(paths []string) Option {
	return func(o *Options) {
		o.ErrorPaths = paths
	}
}

// WithCaller 创建配置是否启用调用者信息的Option
func WithCaller(enable bool) Option {
	return func(o *Options) {
		o.EnableCaller = enable
	}
}

// WithStacktrace 创建配置是否启用堆栈跟踪的Option
func WithStacktrace(enable bool) Option {
	return func(o *Options) {
		o.EnableStacktrace = enable
	}
}

// WithStacktraceLevel 创建配置堆栈跟踪级别的Option
func WithStacktraceLevel(level string) Option {
	return func(o *Options) {
		o.StacktraceLevel = level
	}
}

// WithRotation 创建配置日志轮转的Option
func WithRotation(rotation RotationOptions) Option {
	return func(o *Options) {
		o.Rotation = rotation
	}
}
