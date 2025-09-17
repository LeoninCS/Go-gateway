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

processes = []  # 存储进程信息：(进程对象, 服务名, 端口, 相对路径)


def log(message):
    """带时间戳的日志输出"""
    print(f"[{datetime.now().strftime('%Y-%m-%d %H:%M:%S')}] {message}")


def free_port(port):
    """安全释放端口：仅杀死 LISTEN 状态的 TCP 进程"""
    try:
        # 使用 fuser 命令，更简洁可靠
        subprocess.run(f"fuser -k -n tcp {port}", shell=True, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        log(f"[INFO] 确保端口 {port} 已被释放。")
    except (subprocess.CalledProcessError, FileNotFoundError):
        # fuser 命令不存在或没有进程在监听该端口，都可忽略
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
        env={**os.environ, "PORT": f"{port}"} # PORT 环境变量通常不带冒号
    )

    threading.Thread(target=stream_output, args=(proc, service_name), daemon=True).start()

    processes.append((proc, service_name, port, rel_path))
    return proc



def monitor_services():
    """监控服务运行状态"""
    while True:
        time.sleep(1)
        # 从列表副本进行迭代以安全地删除元素
        for proc_info in processes[:]:
            proc, service_name, port, rel_path = proc_info
            if proc.poll() is not None:
                log(f"\n[ERROR] 服务 {service_name}（端口 {port}）异常退出！退出码：{proc.returncode}")
                processes.remove(proc_info)
                # 可以选择在这里添加服务重启逻辑
                if not processes:
                    log("[INFO] 所有服务已退出，脚本终止")
                    sys.exit(1)


def stop_services(sig, frame):
    """优雅停止所有服务"""
    log("\n[STOP] 收到停止信号，正在关闭所有服务...")
    # 反向停止，先停网关
    for proc, service_name, port, _ in reversed(processes):
        if proc.poll() is None:
            try:
                log(f"[STOP] 发送终止信号到 {service_name}（PID={proc.pid}）")
                # Go 程序通常能很好地处理 SIGINT (Ctrl+C)
                proc.send_signal(signal.SIGINT)
            except Exception as e:
                log(f"[ERROR] 发送停止信号到 {service_name} 失败：{str(e)}")

    # 等待所有进程终止
    shutdown_timeout = 10  # 秒
    start_time = time.time()
    while any(p[0].poll() is None for p in processes) and time.time() - start_time < shutdown_timeout:
        time.sleep(0.5)
    
    # 强制杀死仍在运行的进程
    for proc, service_name, port, _ in processes:
        if proc.poll() is None:
            log(f"[STOP] 进程 {service_name}（PID={proc.pid}）未能优雅退出，强制杀死。")
            proc.kill()

    log("[STOP] 所有服务已关闭")
    sys.exit(0)


def load_services_from_config():
    """从 config.yaml 读取服务和端口，包括 api-gateway"""
    with open(CONFIG_FILE, "r") as f:
        config = yaml.safe_load(f)

    service_defs = []

    # 1️⃣ 读取网关服务
    server_port_str = config.get("server", {}).get("port")
    if server_port_str:
        port = int(server_port_str.lstrip(":"))
        rel_path = "./cmd/api-gateway"
        service_defs.append(("api-gateway", rel_path, port))

    # 2️⃣ 读取其他微服务
    # config.yaml 中的 services 是一个字典，不是列表
    services_map = config.get("services", {})
    for service_name, service_config in services_map.items():
        # service_name 直接从字典的键获得 (e.g., "auth-service")
        # service_config 是包含 instances 等信息的字典
        for instance in service_config.get("instances", []):
            url = instance["url"]  # e.g. http://localhost:8085
            port = int(url.split(":")[-1])
            rel_path = f"./cmd/{service_name}"
            service_defs.append((f"{service_name}-{port}", rel_path, port))
            
    return service_defs



if __name__ == "__main__":
    signal.signal(signal.SIGINT, stop_services)
    signal.signal(signal.SIGTERM, stop_services)
    
    try:
        services = load_services_from_config()

        for service_name, rel_path, port in services:
            start_service(service_name, rel_path, port)

        if not processes:
            log("[ERROR] 所有服务启动失败，脚本终止")
            sys.exit(1)

        log("[INFO] 所有服务启动完成（按 Ctrl+C 停止）")
        monitor_services()

    except FileNotFoundError:
        log(f"[FATAL] 配置文件未找到: {CONFIG_FILE}")
        sys.exit(1)
    except Exception as e:
        log(f"[FATAL] 发生未处理的错误: {e}")
        stop_services(None, None) # 尝试清理
        sys.exit(1)