#!/usr/bin/env python3
import subprocess
import signal
import sys
import os
import time
from datetime import datetime
import threading
import yaml

# 项目根目录
PROJECT_ROOT = "/home/leon/GoCode/go-gateway"
CONFIG_FILE = os.path.join(PROJECT_ROOT, "configs/config.yaml")

processes = []  # 存储进程信息：(进程对象, 服务名, 端口)


def log(message):
    """带时间戳的日志输出"""
    print(f"[{datetime.now().strftime('%Y-%m-%d %H:%M:%S')}] {message}")


def free_port(port):
    """安全释放端口：仅杀死 LISTEN 状态的 TCP 进程"""
    try:
        result = subprocess.check_output(
            f"lsof -t -i tcp:{port} -s TCP:LISTEN",
            shell=True,
            stderr=subprocess.STDOUT
        ).decode().strip()

        if result:
            for pid in result.splitlines():
                if os.path.exists(f"/proc/{pid}"):
                    log(f"[INFO] 释放端口 {port}：杀死进程 PID={pid}")
                    os.kill(int(pid), signal.SIGTERM)
                    time.sleep(0.5)
                    if os.path.exists(f"/proc/{pid}"):
                        os.kill(int(pid), signal.SIGKILL)
                        log(f"[WARN] 进程 PID={pid} 强制杀死")
    except subprocess.CalledProcessError:
        pass
    except Exception as e:
        log(f"[ERROR] 释放端口 {port} 失败：{str(e)}")


def stream_output(proc, service_name):
    """实时打印服务输出"""
    for line in iter(proc.stdout.readline, ''):
        if line:
            log(f"[{service_name}] {line.strip()}")
    proc.stdout.close()


def start_service(service_name, rel_path, port):
    """启动单个服务，并捕获输出日志"""
    abs_path = os.path.abspath(os.path.join(PROJECT_ROOT, rel_path))

    if not os.path.exists(abs_path):
        log(f"[ERROR] 服务目录不存在：{abs_path}")
        return None

    free_port(port)

    log(f"[START] 启动服务 {service_name}（端口 {port}）：{rel_path}")
    proc = subprocess.Popen(
        ["go", "run", rel_path],
        cwd=PROJECT_ROOT,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        # 👇 在这里注入 PORT 环境变量
        env={**os.environ, "PORT": f":{port}"}
    )

    threading.Thread(target=stream_output, args=(proc, service_name), daemon=True).start()

    processes.append((proc, service_name, port, rel_path))
    return proc



def monitor_services():
    """监控服务运行状态"""
    while True:
        time.sleep(1)
        for i, (proc, service_name, port, rel_path) in enumerate(processes):
            if proc.poll() is not None:
                log(f"\n[ERROR] 服务 {service_name}（端口 {port}）异常退出！退出码：{proc.returncode}")
                processes.pop(i)
                if not processes:
                    log("[INFO] 所有服务已退出，脚本终止")
                    sys.exit(1)


def stop_services(sig, frame):
    """优雅停止所有服务"""
    log("\n[STOP] 收到停止信号，正在关闭所有服务...")
    for proc, service_name, port, _ in processes:
        if proc.poll() is None:
            try:
                proc.terminate()
                log(f"[STOP] 发送终止信号到 {service_name}（PID={proc.pid}）")
                for _ in range(5):
                    time.sleep(1)
                    if proc.poll() is not None:
                        break
                if proc.poll() is None:
                    proc.kill()
                    log(f"[STOP] 强制杀死 {service_name}（PID={proc.pid}）")
            except Exception as e:
                log(f"[ERROR] 停止 {service_name} 失败：{str(e)}")
    log("[STOP] 所有服务已关闭")
    sys.exit(0)


def load_services_from_config():
    """从 config.yaml 读取服务和端口，包括 api-gateway"""
    with open(CONFIG_FILE, "r") as f:
        config = yaml.safe_load(f)

    service_defs = []

    # 1️⃣ 读取网关服务
    server_port = config.get("server", {}).get("port")
    if server_port:
        port = int(server_port.lstrip(":"))
        rel_path = "./cmd/api-gateway"
        service_defs.append(("api-gateway", rel_path, port))

    # 2️⃣ 读取其他微服务
    for service in config.get("services", []):
        service_name = service["name"]
        for instance in service.get("instances", []):
            url = instance["url"]  # e.g. http://localhost:8085
            port = int(url.split(":")[-1])
            rel_path = f"./cmd/{service_name}"
            service_defs.append((service_name, rel_path, port))

    return service_defs



if __name__ == "__main__":
    signal.signal(signal.SIGINT, stop_services)
    signal.signal(signal.SIGTERM, stop_services)

    services = load_services_from_config()

    for service_name, rel_path, port in services:
        start_service(service_name, rel_path, port)

    if not processes:
        log("[ERROR] 所有服务启动失败，脚本终止")
        sys.exit(1)

    log("[INFO] 所有服务启动完成（按 Ctrl+C 停止）")
    monitor_services()
