package main

import (
	"fmt"
	"log"

	"gateway.example/go-gateway/internal/config"
	"gateway.example/go-gateway/internal/dao"
	"gateway.example/go-gateway/internal/database"
	"gateway.example/go-gateway/internal/gateway"  // gateway 包
	"gateway.example/go-gateway/internal/handlers" // ★ 导入新的 handlers 包
	"gateway.example/go-gateway/internal/models"
	"gateway.example/go-gateway/internal/server"
	"gateway.example/go-gateway/internal/service"
)

func main() {
	// ... 1-4 步数据库相关初始化保持不变 ...
	log.Println("Loading configuration...")
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("Fatal error: failed to load config: %v", err)
	}
	log.Println("Configuration loaded successfully.")

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.DBName,
	)
	log.Printf("Preparing database connection for user '%s' on host '%s:%d'", cfg.Database.User, cfg.Database.Host, cfg.Database.Port)

	db, err := database.NewConnection(dsn)
	if err != nil {
		log.Fatalf("Fatal error: failed to connect to database: %v", err)
	}

	log.Println("Running database migrations...")
	err = db.AutoMigrate(&models.User{})
	if err != nil {
		log.Fatalf("Fatal error: failed to auto-migrate database: %v", err)
	}
	log.Println("Database migration completed.")

	// --- 5. 依赖注入：按顺序创建各个层 ---
	// DAO -> Service -> Handler -> Gateway
	log.Println("Initializing application layers (DAO, Service, Handler, Gateway)...")

	// a. 创建数据访问层 (DAO)
	userDAO := dao.NewUserDAO(db)

	// b. 创建业务逻辑层 (Service)
	authService := service.NewAuthService(userDAO, cfg.JWT.SecretKey, cfg.JWT.DurationMinutes)

	// c. ★ 创建 HTTP 处理层 (Handler)
	authHandler := handlers.NewAuthHandler(authService)

	// d. ★ 创建网关/路由层 (Gateway)，并将 Handler 注入进去
	gw, err := gateway.NewGateway(cfg, authHandler)
	if err != nil {
		log.Fatalf("Fatal error: failed to create gateway: %v", err)
	}
	log.Println("Application layers initialized successfully.")

	// --- 6. 创建并启动 HTTP 服务器 ---
	srv, err := server.NewServer(cfg.Gateway.Port, gw)
	if err != nil {
		log.Fatalf("Fatal error: failed to create server: %v", err)
	}
	log.Printf("Starting HTTP server on port %s...", cfg.Gateway.Port)

	if err := srv.Start(); err != nil {
		log.Fatalf("Fatal error: server failed to start: %v", err)
	}
}
