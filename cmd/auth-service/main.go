package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gateway.example/go-gateway/internal/config"
	authHandler "gateway.example/go-gateway/internal/handler/auth"
	"gateway.example/go-gateway/internal/repository"
	authSvc "gateway.example/go-gateway/internal/service/auth"
)

func main() {
	// 1. 加载配置文件
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}

	// 2. 初始化用户仓库 - 使用内存存储用户数据
	userRepo := repository.NewInMemoryUserRepository()

	// 3. 创建认证服务 - 负责用户认证的核心业务逻辑
	authService, err := authSvc.NewAuthService(userRepo, cfg.JWT.SecretKey, cfg.JWT.DurationMinutes)
	if err != nil {
		log.Fatalf("could not create auth service: %v", err)
	}

	// 4. 创建认证处理器 - HTTP请求处理入口
	authHandler := authHandler.NewAuthHandler(authService)

	// 5. 创建HTTP请求多路复用器 - 路由分发器
	mux := http.NewServeMux()

	// 6. 注册登录接口 - 仅支持POST方法
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		authHandler.LoginHandler(w, r)
	})

	// 7. 注册健康检查接口 - 用于服务健康状态监控
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// 8. 注册令牌验证接口 - 供API网关调用
	mux.HandleFunc("/validate", func(w http.ResponseWriter, r *http.Request) {
		authHandler.ValidateHandler(w, r)
	})

	// 9. 获取服务端口号 - 支持环境变量配置
	port := os.Getenv("PORT")
	if port == "" {
		port = "8085"
	}

	// 10. 端口格式标准化 - 确保端口格式正确
	if !strings.Contains(port, ":") {
		port = ":" + port
	}
	log.Printf("Auth service starting on port %s", port)

	// 11. 创建HTTP服务器实例 - 支持优雅关闭
	server := &http.Server{
		Addr:         port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	// 12. 在goroutine中启动服务器 - 避免阻塞主线程
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("failed to start auth service: %v", err)
		}
	}()

	// 13. 优雅关闭处理 - 监听系统中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Auth service is shutting down...")

	// 14. 执行优雅关闭 - 给未完成的请求30秒处理时间
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Auth service shutdown error: %v", err)
	}
	log.Println("Auth service stopped")
}
