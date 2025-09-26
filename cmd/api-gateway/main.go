package main

import (
	"log"
	"net/http"

	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/core"
	//"gateway.example/go-gateway/internal/core/ratelimit"
)

func main() {

	// --- 1. 加载配置 ---
	log.Println("加载配置中...")
	cfg, err := config.Load("./configs/config.yaml")
	if err != nil {
		log.Fatalf("致命错误: 加载配置失败: %v", err)
	}
	log.Println("配置加载成功。")

	// --- 2. 日志配置 ---
	log.Println("配置日志...")

	// --- 3. 依赖注入：创建网关实例 ---
	log.Println("初始化网关层...")
	gw, err := core.NewGateway(cfg)
	if err != nil {
		log.Fatalf("创建网关失败: %v", err)
	}
	log.Println("网关层初始化成功。")

	// --- 4. 创建并启动 HTTP 服务器 ---
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
	srv.GracefulShutdown()
}
