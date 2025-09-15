package main

import (
	"log"
	"net/http"
	"os"

	"gateway.example/go-gateway/internal/config"
	authHandler "gateway.example/go-gateway/internal/handler/auth"
	"gateway.example/go-gateway/internal/repository"
	authSvc "gateway.example/go-gateway/internal/service/auth"
)

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		log.Fatalf("could not load config: %v", err)
	}

	userRepo := repository.NewInMemoryUserRepository()
	authService := authSvc.NewAuthService(userRepo, cfg.JWT.SecretKey, cfg.JWT.DurationMinutes)
	authHandler := authHandler.NewAuthHandler(authService)

	mux := http.NewServeMux()
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
			return
		}
		authHandler.LoginHandler(w, r)
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// 从环境变量 PORT 获取端口，默认为 8085
	port := os.Getenv("PORT")
	if port == "" {
		port = ":8085"
	}
	log.Printf("Auth service starting on port %s", port)

	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatalf("failed to start auth service: %v", err)
	}
}
