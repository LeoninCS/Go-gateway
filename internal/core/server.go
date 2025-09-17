package core

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Server 封装了 http.Server
type Server struct {
	httpServer *http.Server
}

func (s *Server) Shutdown(ctx context.Context) any {
	panic("unimplemented")
}

func NewServer(port string, handler http.Handler) (*Server, error) {
	srv := &http.Server{
		Addr:         port,
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	return &Server{httpServer: srv}, nil
}

// Start 启动服务器
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// GracefulShutdown 优雅关闭服务器
func (s *Server) GracefulShutdown() {
	quit := make(chan os.Signal, 1)
	// 监听 SIGINT (Ctrl+C) 和 SIGTERM (kill 命令)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// 阻塞直到接收到信号
	<-quit
	log.Println("收到停止信号，服务器正在优雅地关闭...")

	// 赋予服务器一些时间来完成正在处理的请求
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // 30秒的关闭超时
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("致命错误: 服务器强制关闭: %v", err)
	}

	// 关闭网关持有的其他资源 (负载均衡器、限流器等)
	if err := s.httpServer.Close(); err != nil { // 调用 Gateway 的 Close 方法
		log.Fatalf("致命错误: 关闭网关资源失败: %v", err)
	}
	log.Println("服务器已优雅关闭。")
}
