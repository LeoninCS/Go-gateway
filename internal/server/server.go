// File: internal/server/server.go (修改后版本)
package server

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

// NewServer 创建一个新的 Server 实例
// 注意！函数签名已经改变，增加了第二个参数 handler
func NewServer(port string, handler http.Handler) (*Server, error) {
	srv := &http.Server{
		Addr:    port,    // 传入的端口字符串
		Handler: handler, // <-- 关键！将传入的 handler (也就是我们的网关) 设置为服务器的处理器
		// 可以添加更多服务器配置，如超时
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return &Server{httpServer: srv}, nil
}

// Start 启动服务器并优雅地处理关闭信号
func (s *Server) Start() error {
	// 使用一个 channel 来接收错误
	serverErrors := make(chan error, 1)

	// 在一个 goroutine 中启动服务器，这样它不会阻塞主线程
	go func() {
		log.Printf("Server is listening on %s", s.httpServer.Addr)
		serverErrors <- s.httpServer.ListenAndServe()
	}()

	// 优雅关机逻辑
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	// 阻塞，直到接收到错误或关机信号
	select {
	case err := <-serverErrors:
		return err
	case sig := <-shutdown:
		log.Printf("Shutdown signal received: %s. Starting graceful shutdown...", sig)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Graceful shutdown failed: %v", err)
			return err
		}
		log.Println("Server gracefully stopped")
	}

	return nil
}
