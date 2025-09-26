package logger

import (
	"net/http"
	"time"

	"github.com/google/uuid"
)

// 自定义响应写入器包装器，用于记录状态码
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

// 重写WriteHeader方法以记录状态码
func (w *responseWriterWrapper) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

// Status 返回记录的状态码
func (w *responseWriterWrapper) Status() int {
	if w.statusCode == 0 {
		// 如果没有显式设置状态码，默认为200 OK
		return http.StatusOK
	}
	return w.statusCode
}

// Middleware 创建一个HTTP中间件，自动记录请求日志
func Middleware(logger Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 生成请求ID和跟踪ID
			requestID := uuid.New().String()
			traceID := uuid.New().String()

			// 向context中添加请求相关信息
			ctx := r.Context()
			ctx = WithRequestID(ctx, requestID)
			ctx = WithTraceID(ctx, traceID)
			ctx = WithCustomFields(ctx, map[string]interface{}{
				"method": r.Method,
				"path":   r.URL.Path,
				"remote": r.RemoteAddr,
			})

			// 更新请求的context
			r = r.WithContext(ctx)

			// 包装响应写入器以记录状态码
			wrapper := &responseWriterWrapper{
				ResponseWriter: w,
				statusCode:     0,
			}

			// 记录请求开始
			startTime := time.Now()
			logger.Info(ctx, "request started")

			// 执行后续处理，传入包装后的响应写入器
			next.ServeHTTP(wrapper, r)

			// 记录请求完成，使用包装器获取状态码
			duration := time.Since(startTime)
			logger.Info(ctx, "request completed",
				"duration_ms", duration.Milliseconds(),
				"status_code", wrapper.Status(),
			)
		})
	}
}
