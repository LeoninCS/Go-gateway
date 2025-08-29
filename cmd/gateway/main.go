package main

import (
	"fmt"
	"log"

	"gateway.example/go-gateway/internal/auth"
	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/database"
	"gateway.example/go-gateway/internal/gateway"
	"gateway.example/go-gateway/internal/health"
	"gateway.example/go-gateway/internal/models"
	"gateway.example/go-gateway/internal/repository"
	"gateway.example/go-gateway/internal/server"
)

func main() {
	// --- 1. 加载配置 ---
	log.Println("Loading configuration...")
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("Fatal error: failed to load config: %v", err)
	}
	log.Println("Configuration loaded successfully.")

	// --- 2. 初始化数据库连接 ---
	log.Println("Initializing database connection...")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.DBName,
	)

	db, err := database.NewConnection(dsn)
	if err != nil {
		log.Fatalf("Fatal error: failed to connect to database: %v", err)
	}

	// --- 3. 自动迁移数据库 ---
	log.Println("Running database migrations...")
	err = db.AutoMigrate(&models.User{})
	if err != nil {
		log.Fatalf("Fatal error: failed to auto-migrate database: %v", err)
	}
	log.Println("Database migration completed.")

	// --- 4. 依赖注入：按顺序创建各个层 ---
	log.Println("Initializing application layers...")

	// a. 创建 Repository 层
	userRepo := repository.NewGormUserRepository(db)

	// b. 创建 Service 层
	authService := auth.NewAuthService(userRepo, cfg.JWT.SecretKey, cfg.JWT.DurationMinutes)

	// c. 创建 Handler 层
	authHandler := auth.NewAuthHandler(authService)
	healthHandler := health.NewHealthHandler()
	// d. 创建网关层
	gw, err := gateway.NewGateway(cfg, authHandler, healthHandler)
	if err != nil {
		log.Fatalf("Fatal error: failed to create gateway: %v", err)
	}
	log.Println("Application layers initialized successfully.")

	// --- 5. 创建并启动 HTTP 服务器 ---
	srv, err := server.NewServer(cfg.Gateway.Port, gw)
	if err != nil {
		log.Fatalf("Fatal error: failed to create server: %v", err)
	}
	log.Printf("Starting HTTP server on port %s...", cfg.Gateway.Port)

	if err := srv.Start(); err != nil {
		log.Fatalf("Fatal error: server failed to start: %v", err)
	}
}
