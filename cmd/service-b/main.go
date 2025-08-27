// 文件：cmd/service-b/main.go
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
)

// mainHandler 处理服务的主要请求
func mainHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Service B received request for: %s", r.URL.Path)
	fmt.Fprintf(w, "Hello from Service B at path: %s\n", r.URL.Path)
}

// healthHandler 处理健康检查请求
func healthHandler(w http.ResponseWriter, r *http.Request) {
	// 我们会记录这个请求以便在终端看到它的运行
	log.Println("Service B received request for: /health")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

func main() {
	port := flag.String("port", ":8082", "Port for the service to listen on")
	flag.Parse()

	// 为根路径注册主处理器
	http.HandleFunc("/", mainHandler)
	// 为 /health 路径注册专用处理器
	http.HandleFunc("/health", healthHandler)

	log.Printf("Starting Service B on %s", *port)
	if err := http.ListenAndServe(*port, nil); err != nil {
		log.Fatalf("Could not start Service B: %v", err)
	}
}
