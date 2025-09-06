package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func mainHandler(w http.ResponseWriter, r *http.Request) {
	port := getPort()
	log.Printf("Service B (%s) received request for: %s", port, r.URL.Path)
	fmt.Fprintf(w, "Hello from Service B at port %s, path: %s\n", port, r.URL.Path)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	port := getPort()
	log.Printf("Service B (%s) received request for: /healthz", port)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		port = ":8083"
	}
	return port
}

func main() {
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
		log.Printf("Starting Service B on %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not start Service B: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Service B is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Println("Service B stopped")
}
