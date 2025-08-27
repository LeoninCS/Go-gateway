package dao

import (
	"errors" // 导入标准库 errors
	"strings"

	"gateway.example/go-gateway/internal/models" // ★ 导入 models 包，而不是 auth
	"gorm.io/gorm"
)

// ★ 定义 DAO 层相关的错误
var (
	ErrUserExists   = errors.New("user already exists")
	ErrUserNotFound = errors.New("user not found")
)

// UserDAO 是一个接口，定义了所有与用户数据相关的操作
type UserDAO interface {
	Create(user *models.User) error                       // ★ 使用 models.User
	FindByUsername(username string) (*models.User, error) // ★ 使用 models.User
}

// userDAO 实现了 UserDAO 接口
type userDAO struct {
	db *gorm.DB
}

// NewUserDAO 是 userDAO 的构造函数
func NewUserDAO(db *gorm.DB) UserDAO {
	return &userDAO{
		db: db,
	}
}

// Create 将一个新用户存入数据库
func (d *userDAO) Create(user *models.User) error { // ★ 使用 models.User
	result := d.db.Create(user)
	if result.Error != nil {
		if strings.Contains(result.Error.Error(), "Duplicate entry") {
			return ErrUserExists // ★ 直接使用本包定义的错误
		}
		return result.Error
	}
	return nil
}

// FindByUsername 通过用户名在数据库中查找用户
func (d *userDAO) FindByUsername(username string) (*models.User, error) { // ★ 使用 models.User
	var user models.User // ★ 使用 models.User
	result := d.db.First(&user, "username = ?", username)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) { // 使用 errors.Is 更健壮
			return nil, ErrUserNotFound // ★ 直接使用本包定义的错误
		}
		return nil, result.Error
	}
	return &user, nil
}
