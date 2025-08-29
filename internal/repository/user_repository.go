package repository

import (
	"gateway.example/go-gateway/internal/models"
	"gorm.io/gorm"
)

// GormUserRepository 是 UserRepository 接口的 GORM 实现。
type GormUserRepository struct {
	db *gorm.DB
}

// NewGormUserRepository 构造函数，依赖注入 *gorm.DB
func NewGormUserRepository(db *gorm.DB) UserRepository {
	return &GormUserRepository{db: db}
}

func (r *GormUserRepository) Create(user *models.User) error {
	result := r.db.Create(user)
	if result.Error != nil {
		// 检查并转换 GORM 错误为我们定义的通用错误
		if isDuplicateError(result.Error) {
			return ErrDuplicate
		}
		return result.Error
	}
	return nil
}

func (r *GormUserRepository) FindByUsername(username string) (*models.User, error) {
	var user models.User
	result := r.db.Where("username = ?", username).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return nil, ErrNotFound
		}
		return nil, result.Error
	}
	return &user, nil
}

// isDuplicateError 是一个辅助函数，用于检查 GORM 错误是否是唯一键冲突
func isDuplicateError(err error) bool {
	// 这里可以根据实际使用的数据库类型进行更精确的判断
	// 对于 MySQL，错误信息通常包含 "Duplicate entry" 或错误码 1062
	// 这是一个简单的实现，实际项目中可能需要更复杂的逻辑
	return err != nil &&
		(contains(err.Error(), "Duplicate entry") ||
			contains(err.Error(), "1062"))
}

func contains(s, substr string) bool {
	// 简单的字符串包含检查
	// 在实际项目中，你可能需要使用更健壮的方法
	return len(s) >= len(substr) &&
		s[:len(substr)] == substr ||
		contains(s[1:], substr)
}
