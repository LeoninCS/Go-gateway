package main

import (
	"context"
	"net/http"

	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/core"
	"gateway.example/go-gateway/pkg/logger"
)

var log logger.Logger

func main() {
	// --- 1. 初始化日志 ---
	log, err := logger.NewWithConfigFile("./configs/logs/api-gateway-log.yaml")
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	// --- 2. 加载配置 ---
	log.Info(ctx, "加载配置中...")
	cfg, err := config.Load("./configs/config.yaml")
	if err != nil {
		log.Fatal(ctx, "致命错误: 加载配置失败", "error", err)
	}
	log.Info(ctx, "配置加载成功。")

	// --- 3. 依赖注入：创建网关实例 ---
	log.Info(ctx, "初始化网关层...")
	gw, err := core.NewGateway(cfg, log)
	if err != nil {
		log.Fatal(ctx, "创建网关失败", "error", err)
	}
	log.Info(ctx, "网关层初始化成功。")

	// --- 4. 创建并启动 HTTP 服务器 ---
	srv, err := core.NewServer(cfg.Server.Port, gw, log)
	if err != nil {
		log.Fatal(ctx, "致命错误: 创建服务器失败", "error", err)
	}
	log.Info(ctx, "HTTP 服务器正在端口上启动", "port", cfg.Server.Port)

	// 在一个 Goroutine 中启动服务器，以便主 Goroutine 可以监听信号
	go func() {
		if err := srv.Start(); err != nil && err != http.ErrServerClosed { // 检查 err != http.ErrServerClosed
			log.Fatal(ctx, "服务器启动失败", "error", err)
		}
	}()

	// --- 5. 平滑关机处理 ---
	// 创建一个通道来接收停止信号
	srv.GracefulShutdown()
}
