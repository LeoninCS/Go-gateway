// File: internal/models/user.go
package models

import "gorm.io/gorm"

// User 定义了用户模型，对应数据库中的 users 表
type User struct {
	gorm.Model          // 包含了 ID, CreatedAt, UpdatedAt, DeletedAt
	Username     string `gorm:"unique;not null"`
	Phone        string `gorm:"unique;not null"`
	PasswordHash string `gorm:"not null"`
}
