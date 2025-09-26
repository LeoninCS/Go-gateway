package logger

import (
	"context"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger 接口定义了完整的日志记录方法
type Logger interface {
	Debug(ctx context.Context, msg string, fields ...interface{})
	Info(ctx context.Context, msg string, fields ...interface{})
	Warn(ctx context.Context, msg string, fields ...interface{})
	Error(ctx context.Context, msg string, fields ...interface{})
	DPanic(ctx context.Context, msg string, fields ...interface{})
	Panic(ctx context.Context, msg string, fields ...interface{})
	Fatal(ctx context.Context, msg string, fields ...interface{})

	// With 方法用于创建带有预设字段的新logger
	With(fields ...interface{}) Logger
}

// zapLogger 是Logger接口的zap实现
type zapLogger struct {
	z *zap.SugaredLogger
}

// 确保zapLogger实现了Logger接口
var _ Logger = (*zapLogger)(nil)

// New 创建并返回一个Logger实例，支持函数式选项配置
func New(opts ...Option) (Logger, error) {
	options := &Options{
		Level:            "info",    // 默认info级别
		Format:           "console", // 默认控制台格式
		OutputPaths:      []string{"stdout"},
		ErrorPaths:       []string{"stderr"},
		EnableCaller:     true,
		EnableStacktrace: true,
		StacktraceLevel:  "panic",
		Rotation: RotationOptions{
			Enabled:      false, // 默认不启用
			Policy:       "time",
			TimeInterval: "24h",
			MaxSize:      100, // MB
			MaxBackups:   7,
			MaxAge:       30, // 天或分钟，根据TimeInterval决定
			Compress:     true,
		},
	}
	for _, o := range opts {
		o(options)
	}

	// 配置编码器
	encoderConfig := zapcore.EncoderConfig{
		MessageKey:    "msg",
		LevelKey:      "level",
		TimeKey:       "ts",
		CallerKey:     "caller",
		StacktraceKey: "stacktrace",
		EncodeLevel:   zapcore.LowercaseLevelEncoder,
		EncodeTime:    zapcore.ISO8601TimeEncoder, // ISO8601时间格式
		EncodeCaller:  zapcore.ShortCallerEncoder,
	}

	var encoder zapcore.Encoder
	if options.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// 配置级别
	level := zap.NewAtomicLevelAt(levelFromString(options.Level))

	// 构建zap选项
	var zapOptions []zap.Option
	if options.EnableCaller {
		zapOptions = append(zapOptions, zap.AddCallerSkip(1)) // 修正调用者位置
	}

	if options.EnableStacktrace {
		stacktraceLevel := zapcore.PanicLevel
		if l := levelFromString(options.StacktraceLevel); l != zapcore.InvalidLevel {
			stacktraceLevel = l
		}
		zapOptions = append(zapOptions, zap.AddStacktrace(stacktraceLevel))
	}

	// 处理日志输出，支持轮转
	cores := make([]zapcore.Core, 0)

	// 处理普通输出路径
	if len(options.OutputPaths) > 0 {
		var ws zapcore.WriteSyncer
		if options.Rotation.Enabled {
			// 使用带轮转功能的WriteSyncer
			writers := make([]zapcore.WriteSyncer, 0)
			for _, path := range options.OutputPaths {
				if path == "stdout" || path == "stderr" {
					// 标准输出不进行轮转
					var writer *os.File
					if path == "stdout" {
						writer = os.Stdout
					} else {
						writer = os.Stderr
					}
					writers = append(writers, zapcore.AddSync(writer))
				} else {
					// 确保目录存在
					dir := filepath.Dir(path)
					if err := os.MkdirAll(dir, 0755); err != nil {
						return nil, err
					}
					// 创建带轮转功能的writer
					logger := &lumberjack.Logger{
						Filename:   path,
						MaxBackups: options.Rotation.MaxBackups,
						Compress:   options.Rotation.Compress,
					}

					// 根据策略配置轮转参数
					switch options.Rotation.Policy {
					case "size":
						// 只基于大小轮转
						logger.MaxSize = options.Rotation.MaxSize // MB
					case "time":
						// 只基于时间轮转
						// 转换TimeInterval和MaxAge到lumberjack支持的格式
						configureTimeRotation(logger, options.Rotation.TimeInterval, options.Rotation.MaxAge)
					case "size_and_time":
						// 同时基于大小和时间轮转
						logger.MaxSize = options.Rotation.MaxSize // MB
						configureTimeRotation(logger, options.Rotation.TimeInterval, options.Rotation.MaxAge)
					default:
						// 默认使用时间轮转
						configureTimeRotation(logger, options.Rotation.TimeInterval, options.Rotation.MaxAge)
					}

					writers = append(writers, zapcore.AddSync(logger))
				}
			}
			ws = zapcore.NewMultiWriteSyncer(writers...)
		} else {
			// 不使用轮转功能
			openWs, _, err := zap.Open(options.OutputPaths...)
			if err != nil {
				return nil, err
			}
			ws = openWs
		}
		cores = append(cores, zapcore.NewCore(encoder, ws, level))
	}

	// 处理错误输出路径
	if len(options.ErrorPaths) > 0 {
		var ws zapcore.WriteSyncer
		if options.Rotation.Enabled {
			// 使用带轮转功能的WriteSyncer
			writers := make([]zapcore.WriteSyncer, 0)
			for _, path := range options.ErrorPaths {
				if path == "stdout" || path == "stderr" {
					// 标准输出不进行轮转
					var writer *os.File
					if path == "stdout" {
						writer = os.Stdout
					} else {
						writer = os.Stderr
					}
					writers = append(writers, zapcore.AddSync(writer))
				} else {
					// 确保目录存在
					dir := filepath.Dir(path)
					if err := os.MkdirAll(dir, 0755); err != nil {
						return nil, err
					}
					// 创建带轮转功能的writer
					logger := &lumberjack.Logger{
						Filename:   path,
						MaxBackups: options.Rotation.MaxBackups,
						Compress:   options.Rotation.Compress,
					}

					// 根据策略配置轮转参数
					switch options.Rotation.Policy {
					case "size":
						// 只基于大小轮转
						logger.MaxSize = options.Rotation.MaxSize // MB
					case "time":
						// 只基于时间轮转
						configureTimeRotation(logger, options.Rotation.TimeInterval, options.Rotation.MaxAge)
					case "size_and_time":
						// 同时基于大小和时间轮转
						logger.MaxSize = options.Rotation.MaxSize // MB
						configureTimeRotation(logger, options.Rotation.TimeInterval, options.Rotation.MaxAge)
					default:
						// 默认使用时间轮转
						configureTimeRotation(logger, options.Rotation.TimeInterval, options.Rotation.MaxAge)
					}

					writers = append(writers, zapcore.AddSync(logger))
				}
			}
			ws = zapcore.NewMultiWriteSyncer(writers...)
		} else {
			// 不使用轮转功能
			openWs, _, err := zap.Open(options.ErrorPaths...)
			if err != nil {
				return nil, err
			}
			ws = openWs
		}
		// 错误日志使用相同的编码器，但级别设为Error
		errLevel := zap.NewAtomicLevelAt(zapcore.ErrorLevel)
		cores = append(cores, zapcore.NewCore(encoder, ws, errLevel))
	}

	// 构建核心
	core := zapcore.NewTee(cores...)

	// 构建logger
	logger := zap.New(core, zapOptions...)

	return &zapLogger{z: logger.Sugar()}, nil
}

// 配置基于时间的轮转参数
func configureTimeRotation(logger *lumberjack.Logger, timeInterval string, maxAge int) {
	// 根据TimeInterval确定MaxAge的单位
	// lumberjack的MaxAge总是以天为单位，所以需要转换
	// 对于分钟级别的轮转，我们需要特殊处理

	// 解析TimeInterval
	switch timeInterval {
	case "minute":
		// 对于分钟级轮转，lumberjack本身不直接支持
		// 这里我们可以设置MaxAge为0表示不基于时间删除，但会在轮转时创建新文件
		// 实际的分钟级保留需要在应用层额外处理
		logger.MaxAge = 0
	case "hour":
		// 将小时转换为天
		logger.MaxAge = maxAge / 24
	case "day", "24h":
		// 天为单位，直接使用
		logger.MaxAge = maxAge
	default:
		// 默认以天为单位
		logger.MaxAge = maxAge
	}
}

// With 创建带有预设字段的新logger
func (l *zapLogger) With(fields ...interface{}) Logger {
	return &zapLogger{z: l.z.With(fields...)}
}

// Debug 记录debug级别日志
func (l *zapLogger) Debug(ctx context.Context, msg string, fields ...interface{}) {
	contextFields := FromContext(ctx)
	allFields := append(contextFields, fields...)
	l.z.Debugw(msg, allFields...)
}

// Info 记录info级别日志
func (l *zapLogger) Info(ctx context.Context, msg string, fields ...interface{}) {
	contextFields := FromContext(ctx)
	allFields := append(contextFields, fields...)
	l.z.Infow(msg, allFields...)
}

// Warn 记录warn级别日志
func (l *zapLogger) Warn(ctx context.Context, msg string, fields ...interface{}) {
	contextFields := FromContext(ctx)
	allFields := append(contextFields, fields...)
	l.z.Warnw(msg, allFields...)
}

// Error 记录error级别日志
func (l *zapLogger) Error(ctx context.Context, msg string, fields ...interface{}) {
	contextFields := FromContext(ctx)
	allFields := append(contextFields, fields...)
	l.z.Errorw(msg, allFields...)
}

// DPanic 记录dpanic级别日志（开发环境触发panic）
func (l *zapLogger) DPanic(ctx context.Context, msg string, fields ...interface{}) {
	contextFields := FromContext(ctx)
	allFields := append(contextFields, fields...)
	l.z.DPanicw(msg, allFields...)
}

// Panic 记录panic级别日志并触发panic
func (l *zapLogger) Panic(ctx context.Context, msg string, fields ...interface{}) {
	contextFields := FromContext(ctx)
	allFields := append(contextFields, fields...)
	l.z.Panicw(msg, allFields...)
}

// Fatal 记录fatal级别日志并退出程序
func (l *zapLogger) Fatal(ctx context.Context, msg string, fields ...interface{}) {
	contextFields := FromContext(ctx)
	allFields := append(contextFields, fields...)
	l.z.Fatalw(msg, allFields...)
}

// levelFromString 将字符串级别转换为zapcore.Level
func levelFromString(level string) zapcore.Level {
	var l zapcore.Level
	if err := l.UnmarshalText([]byte(level)); err != nil {
		return zapcore.InfoLevel // 默认info级别
	}
	return l
}
