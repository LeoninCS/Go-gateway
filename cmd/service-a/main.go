// 文件：cmd/service-a/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

// mainHandler 处理服务的主要请求
func mainHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Service A received request for: %s", r.URL.Path)
	fmt.Fprintf(w, "Hello from Service A at path: %s\n", r.URL.Path)
}

// healthHandler 处理健康检查请求
func healthHandler(w http.ResponseWriter, r *http.Request) {
	// 不记录健康检查以保持主日志干净，
	// 除非你在调试。
	// log.Println("Service A health check received")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

func main() {
	port := flag.String("port", ":8081", "Port for the service to listen on")
	flag.Parse()

	// 为根路径注册主处理器
	http.HandleFunc("/", mainHandler)

	// --- 这是关键的补充 ---
	// 为 /health 路径注册专用处理器
	http.HandleFunc("/health", healthHandler)

	log.Printf("Starting Service A on %s", *port)
	if err := http.ListenAndServe(*port, nil); err != nil {
		log.Fatalf("Could not start Service A: %v", err)
	}
}
