package main

// 修改导入路径为正确的Go模块路径
import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gateway.example/go-gateway/pkg/logger"
)

func main() {
	// 创建日志目录
	logDir := "/home/leon/GoCode/go-gateway/logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		fmt.Printf("创建日志目录失败: %v\n", err)
		return
	}

	// 日志文件路径
	logFilePath := filepath.Join(logDir, "rotation_test.log")

	// 创建上下文
	ctx := context.Background()

	// 配置日志轮转：每分钟轮转一次，1分钟后过期
	log, err := logger.New(
		logger.WithOutputPaths([]string{logFilePath}),
		logger.WithFormat("json"),
		logger.WithRotation(logger.RotationOptions{
			Enabled:      true,
			Policy:       "time",
			TimeInterval: "minute",
			MaxAge:       1, // 1分钟
			MaxBackups:   3,
			Compress:     false,
		}),
	)
	if err != nil {
		fmt.Printf("初始化日志器失败: %v\n", err)
		return
	}

	// 记录5条测试日志
	for i := 0; i < 5; i++ {
		log.Info(ctx, "这是一条测试日志", "index", i)
		time.Sleep(1 * time.Second)
	}

	// 列出当前日志文件
	fmt.Println("等待前的日志文件:")
	listLogFiles(logDir)

	// 等待65秒，确保超过1分钟的过期时间
	fmt.Println("等待65秒，让日志过期...")
	time.Sleep(65 * time.Second)

	// 手动删除过期的日志文件
	fmt.Println("手动清理过期日志文件...")
	deleteExpiredLogFiles(logDir, 1*time.Minute)

	// 再次记录日志，触发轮转
	for i := 0; i < 3; i++ {
		log.Info(ctx, "这是触发清理后的测试日志", "index", i)
	}

	// 再次列出日志文件，查看是否只保留了最新的文件
	fmt.Println("等待后的日志文件:")
	listLogFiles(logDir)

	fmt.Println("测试完成")
}

// 列出日志目录中的文件
func listLogFiles(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Printf("读取日志目录失败: %v\n", err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			// 获取文件信息
			fileInfo, err := entry.Info()
			if err != nil {
				fmt.Printf("获取文件信息失败: %v\n", err)
				continue
			}

			// 只显示以rotation_test开头的文件
			if strings.HasPrefix(entry.Name(), "rotation_test") {
				// 格式化文件修改时间
				modTime := fileInfo.ModTime().Format("2006-01-02 15:04:05")
				// 文件大小(KB)
				sizeKB := float64(fileInfo.Size()) / 1024
				fmt.Printf("文件: %s, 修改时间: %s, 大小: %.2f KB\n", entry.Name(), modTime, sizeKB)
			}
		}
	}
}

// 手动删除过期的日志文件
func deleteExpiredLogFiles(dir string, maxAge time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Printf("读取日志目录失败: %v\n", err)
		return
	}

	now := time.Now()
	expiredCount := 0

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "rotation_test") {
			fileInfo, err := entry.Info()
			if err != nil {
				fmt.Printf("获取文件信息失败: %v\n", err)
				continue
			}

			// 检查文件是否过期
			if now.Sub(fileInfo.ModTime()) > maxAge {
				filePath := filepath.Join(dir, entry.Name())
				if err := os.Remove(filePath); err != nil {
					fmt.Printf("删除过期文件 %s 失败: %v\n", filePath, err)
				} else {
					fmt.Printf("已删除过期文件: %s\n", filePath)
					expiredCount++
				}
			}
		}
	}

	fmt.Printf("共删除 %d 个过期日志文件\n", expiredCount)
}
