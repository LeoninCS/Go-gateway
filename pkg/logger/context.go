// 从context中提取预设字段，返回zap格式的字段切片
package logger

import "context"

// contextKey 自定义类型，防止与其他包的context key冲突
type contextKey string

const (
	// TraceIDKey 用于在context中存储trace_id的键
	TraceIDKey = contextKey("trace_id")
	// UserIDKey 用于在context中存储user_id的键
	UserIDKey = contextKey("user_id")
	// RequestIDKey 用于在context中存储request_id的键
	RequestIDKey = contextKey("request_id")
	// SessionIDKey 用于在context中存储session_id的键
	SessionIDKey = contextKey("session_id")
	// CustomFieldsKey 用于在context中存储自定义字段的键
	CustomFieldsKey = contextKey("custom_fields")
)

// WithTraceID 向context中添加trace_id
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// WithUserID 向context中添加user_id
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}

// WithRequestID 向context中添加request_id
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithSessionID 向context中添加session_id
func WithSessionID(ctx context.Context, sessionID string) context.Context {
	return context.WithValue(ctx, SessionIDKey, sessionID)
}

// WithCustomFields 向context中添加自定义字段
func WithCustomFields(ctx context.Context, fields map[string]interface{}) context.Context {
	return context.WithValue(ctx, CustomFieldsKey, fields)
}

// FromContext 从context中提取预设字段，返回zap格式的字段切片
func FromContext(ctx context.Context) []interface{} {
	var fields []interface{}

	// 提取trace_id
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		fields = append(fields, "trace_id", traceID)
	}

	// 提取user_id
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		fields = append(fields, "user_id", userID)
	}

	// 提取request_id
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		fields = append(fields, "request_id", requestID)
	}

	// 提取session_id
	if sessionID, ok := ctx.Value(SessionIDKey).(string); ok {
		fields = append(fields, "session_id", sessionID)
	}

	// 提取自定义字段
	if customFields, ok := ctx.Value(CustomFieldsKey).(map[string]interface{}); ok {
		for k, v := range customFields {
			fields = append(fields, k, v)
		}
	}

	return fields
}
