#!/bin/bash

# 当脚本退出时，执行 kill_all 函数
trap "kill_all" EXIT

# 清理函数
kill_all() {
  echo # Newline for prettier output
  echo "Stopping all services..."
  # -9 确保进程被杀死。2>/dev/null 抑制如果没有找到进程的错误
  if [ ! -z "$SERVICE_A_1_PID" ]; then kill -9 $SERVICE_A_1_PID 2>/dev/null; fi
  if [ ! -z "$SERVICE_A_2_PID" ]; then kill -9 $SERVICE_A_2_PID 2>/dev/null; fi
  if [ ! -z "$SERVICE_B_PID" ]; then kill -9 $SERVICE_B_PID 2>/dev/null; fi
  echo "All background services stopped."
}

echo "Starting all services..."

# 启动 Service A 的第一个实例 (端口 8081)
go run ./cmd/service-a/main.go --port=:8081 &
SERVICE_A_1_PID=$!
echo "Service A (1) started with PID $SERVICE_A_1_PID on port 8081"

# 启动 Service A 的第二个实例 (端口 8083)
go run ./cmd/service-a/main.go --port=:8083 &
SERVICE_A_2_PID=$!
echo "Service A (2) started with PID $SERVICE_A_2_PID on port 8083"

# 启动 Service B (端口 8082)
go run ./cmd/service-b/main.go --port=:8082 &
SERVICE_B_PID=$!
echo "Service B started with PID $SERVICE_B_PID on port 8082"

# 稍等片刻，确保后台服务都已启动
echo "Waiting for backend services to initialize..."
sleep 2

# 在前台启动 Gateway
echo "Starting gateway in the foreground..."
go run ./cmd/gateway/main.go
