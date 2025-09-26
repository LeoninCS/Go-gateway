package logger

import (
	"context"
	"errors"
)

// ErrorWithStack 记录带有错误堆栈的错误日志
func ErrorWithStack(ctx context.Context, logger Logger, err error, msg string, fields ...interface{}) {
	if err != nil {
		allFields := append(fields, "error", err)
		logger.Error(ctx, msg, allFields...)
	}
}

// RecordMetrics 记录性能指标
func RecordMetrics(ctx context.Context, logger Logger, operation string, durationMs int64, success bool, fields ...interface{}) {
	allFields := append(fields,
		"operation", operation,
		"duration_ms", durationMs,
		"success", success,
	)

	if success {
		logger.Info(ctx, "operation completed", allFields...)
	} else {
		logger.Warn(ctx, "operation failed", allFields...)
	}
}

// LogIfError 当有错误时才记录日志
func LogIfError(ctx context.Context, logger Logger, err error, msg string, fields ...interface{}) {
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		allFields := append(fields, "error", err)
		logger.Error(ctx, msg, allFields...)
	}
}
