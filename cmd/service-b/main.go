package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"gateway.example/go-gateway/pkg/logger"
)

var log logger.Logger

func mainHandler(w http.ResponseWriter, r *http.Request) {
	port := getPort()
	ctx := context.Background()
	log.Info(ctx, "Service B received request", "port", port, "path", r.URL.Path)
	fmt.Fprintf(w, "Hello from Service B at port %s, path: %s\n", port, r.URL.Path)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	port := getPort()
	ctx := context.Background()
	log.Info(ctx, "Service B received health check request", "port", port)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}
	// 确保端口格式正确
	if !strings.Contains(port, ":") {
		port = ":" + port
	}
	return port
}

func main() {
	// 初始化自定义日志器
	var err error
	log, err = logger.NewWithConfigFile("./configs/logs/service-b-log.yaml")
	if err != nil {
		panic(err)
	}
	ctx := context.Background()

	port := getPort()

	mux := http.NewServeMux()
	mux.HandleFunc("/", mainHandler)
	mux.HandleFunc("/healthz", healthHandler)

	server := &http.Server{
		Addr:         port,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		log.Info(ctx, "Starting Service B", "port", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(ctx, "Could not start Service B", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info(ctx, "Service B is shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Info(ctx, "Server shutdown error", "error", err)
	}
	log.Info(ctx, "Service B stopped")
}
