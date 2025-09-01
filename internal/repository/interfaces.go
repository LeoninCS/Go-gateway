package repository

import "gateway.example/go-gateway/internal/models"

// UserRepository 定义了对 users 表的操作接口。
type UserRepository interface {
	Create(user *models.User) error
	FindByUsername(username string) (*models.User, error)
	// 可以根据需要添加更多方法，如:
	// FindByID(id uint) (*models.User, error)
	Update(user *models.User) error
	Delete(id uint) error
}

// 在这里定义其他模型的 Repository 接口，例如:
// RouteRepository, ServiceRepository 等
// type RouteRepository interface { ... }
