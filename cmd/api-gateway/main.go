package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/core"
	"gateway.example/go-gateway/internal/core/ratelimit"
	Svc "gateway.example/go-gateway/internal/service/ratelimit"
)

func main() {
	// --- 1. 加载配置 ---
	log.Println("加载配置中...")
	cfg, err := config.Load("./configs/config.yaml")
	if err != nil {
		log.Fatalf("致命错误: 加载配置失败: %v", err)
	}
	log.Println("配置加载成功。")

	// --- 2. 初始化限流器 ---
	log.Println("初始化限流器...")
	// 对于单机内存限流器，只需一个全局实例
	rateLimiter := ratelimit.NewMemoryTokenBucketLimiter()
	log.Printf("限流器已初始化: %s\n", rateLimiter.Name())

	// --- 3. 依赖注入：创建网关实例 ---
	log.Println("初始化网关层...")

	rateLimitSvc, err := Svc.NewService(cfg)
	if err != nil {
		log.Fatalf("致命错误: 创建服务失败: %v", err)
	}

	gw := core.NewGateway(cfg, rateLimitSvc) // 将 rateLimiter 传给 NewGateway
	log.Println("网关层初始化成功。")

	// --- 4. 创建并启动 HTTP 服务器 ---
	// 使用现有的 server.NewServer，将 gw (它实现了 http.Handler) 传入
	srv, err := core.NewServer(cfg.Server.Port, gw)
	if err != nil {
		log.Fatalf("致命错误: 创建服务器失败: %v", err)
	}

	log.Printf("HTTP 服务器正在端口 %s 上启动...", cfg.Server.Port)

	// 在一个 Goroutine 中启动服务器，以便主 Goroutine 可以监听信号
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed { // 检查 err != http.ErrServerClosed
			log.Fatalf("服务器启动失败: %v", err)
		}
	}()

	// --- 5. 平滑关机处理 ---
	// 创建一个通道来接收停止信号
	quit := make(chan os.Signal, 1)
	// 监听 SIGINT (Ctrl+C) 和 SIGTERM (kill 命令)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	// 阻塞直到接收到信号
	<-quit
	log.Println("收到停止信号，服务器正在优雅地关闭...")

	// 赋予服务器一些时间来完成正在处理的请求
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // 30秒的关闭超时
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("致命错误: 服务器强制关闭: %v", err)
	}

	// 关闭网关持有的其他资源 (负载均衡器、限流器等)
	if err := gw.Close(); err != nil { // 调用 Gateway 的 Close 方法
		log.Fatalf("致命错误: 关闭网关资源失败: %v", err)
	}
	log.Println("服务器已优雅关闭。")
}
