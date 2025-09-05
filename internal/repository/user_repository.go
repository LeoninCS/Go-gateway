// file: internal/repository/user_repository.go
package repository

import (
	"errors"

	"gateway.example/go-gateway/internal/models" // 注意：请将 "gateway-example" 替换为你的 go.mod 中的模块名
)

// UserRepository 定义了用户数据的操作接口
type UserRepository interface {
	FindByUsername(username string) (*models.User, error)
}

// NewInMemoryUserRepository 创建一个基于内存的用户仓库实例，用于测试
func NewInMemoryUserRepository() UserRepository {
	// 创建一些假数据
	users := map[string]*models.User{
		"admin": {ID: "1", Username: "xcq", Password: "password123"},
		"user":  {ID: "2", Username: "user", Password: "password456"},
		"xcq":   {ID: "2", Username: "xcq", Password: "xxx"},
	}
	return &inMemoryUserRepository{users: users}
}

type inMemoryUserRepository struct {
	users map[string]*models.User
}

func (r *inMemoryUserRepository) FindByUsername(username string) (*models.User, error) {
	if user, ok := r.users[username]; ok {
		return user, nil
	}
	return nil, errors.New("user not found")
}
