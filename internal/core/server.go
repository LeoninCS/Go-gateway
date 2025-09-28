package core

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gateway.example/go-gateway/pkg/logger"
)

// Server 封装了 http.Server
type Server struct {
	httpServer *http.Server
	logger     logger.Logger
}

// Shutdown 接收context参数的优雅关闭方法
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info(ctx, "服务器正在关闭...")

	// 调用底层http.Server的Shutdown方法
	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error(ctx, "致命错误: 服务器强制关闭", "error", err)
		return err
	}

	s.logger.Info(ctx, "服务器已优雅关闭。")
	return nil
}

func NewServer(port string, handler http.Handler, log logger.Logger) (*Server, error) {
	srv := &http.Server{
		Addr:         port,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return &Server{
		httpServer: srv,
		logger:     log,
	}, nil
}

// Start 启动服务器
func (s *Server) Start() error {
	s.logger.Info(context.Background(), "服务器启动中...", "addr", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// GracefulShutdown 优雅关闭服务器
func (s *Server) GracefulShutdown() {
	quit := make(chan os.Signal, 1)
	// 监听 SIGINT (Ctrl+C) 和 SIGTERM (kill 命令)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// 阻塞直到接收到信号
	<-quit
	s.logger.Info(context.Background(), "收到停止信号，服务器正在优雅地关闭...")

	// 赋予服务器一些时间来完成正在处理的请求
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // 30秒的关闭超时
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		s.logger.Error(ctx, "致命错误: 服务器强制关闭", "error", err)
	}

	// 关闭网关持有的其他资源 (负载均衡器、限流器等)
	if err := s.httpServer.Close(); err != nil { // 调用 Gateway 的 Close 方法
		s.logger.Error(ctx, "致命错误: 关闭网关资源失败", "error", err)
	}
	s.logger.Info(context.Background(), "服务器已优雅关闭。")
}
