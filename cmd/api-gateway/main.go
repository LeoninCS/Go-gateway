// file: cmd/gateway/main.go
package main

import (
	"log"

	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/gateway"
	"gateway.example/go-gateway/internal/health"
	"gateway.example/go-gateway/internal/server"
)

func main() {
	// --- 1. 加载配置 ---
	log.Println("Loading configuration...")
	// 确保路径正确，这里假设 main.go 在 cmd/gateway/ 下
	cfg, err := config.Load("./configs/config.yaml")
	if err != nil {
		log.Fatalf("Fatal error: failed to load config: %v", err)
	}
	log.Println("Configuration loaded successfully.")

	// --- 2. 依赖注入：创建处理器和网关 ---
	log.Println("Initializing application layers...")

	// a. 创建 Handler
	healthHandler := health.NewHealthHandler()

	// b. 创建网关层
	gw, err := gateway.NewGateway(cfg, healthHandler)
	if err != nil {
		log.Fatalf("Fatal error: failed to create gateway: %v", err)
	}
	log.Println("Application layers initialized successfully.")

	// --- 3. 创建并启动 HTTP 服务器 ---
	// **修改点**: 将 cfg.Gateway.Port 修改为 cfg.Server.Port
	srv, err := server.NewServer(cfg.Server.Port, gw)
	if err != nil {
		log.Fatalf("Fatal error: failed to create server: %v", err)
	}

	// **修改点**: 将 cfg.Gateway.Port 修改为 cfg.Server.Port
	log.Printf("Starting HTTP server on port %s...", cfg.Server.Port)

	if err := srv.Start(); err != nil {
		log.Fatalf("Fatal error: server failed to start: %v", err)
	}
}
