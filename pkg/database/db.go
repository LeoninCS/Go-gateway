// File: internal/database/database.go
package database

import (
	"log"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// NewConnection 负责创建并返回一个新的数据库连接。
// 它不再是单例，也不再是全局变量。
// 这样使得依赖关系非常明确，并且极大地简化了测试。
func NewConnection(dsn string) (*gorm.DB, error) {
	// 使用 mysql.Open() 连接 MySQL 数据库
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		// 不要在这里用 log.Fatalf，而是返回错误，让调用者决定如何处理。
		// main 函数可以选择 panic，而测试代码可以选择跳过。
		return nil, err
	}

	// 连接池的配置（可选，但生产环境强烈推荐）
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	// 设置最大打开连接数
	sqlDB.SetMaxOpenConns(100)
	// 设置最大空闲连接数
	sqlDB.SetMaxIdleConns(10)

	log.Println("MySQL database connection established.")

	// 注意：AutoMigrate 逻辑已经从这里移除！

	return db, nil
}
