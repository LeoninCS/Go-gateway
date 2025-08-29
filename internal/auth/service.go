package auth

import (
	"time"

	"gateway.example/go-gateway/internal/models"
	"gateway.example/go-gateway/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo    repository.UserRepository // 使用新的接口类型
	jwtSecret   string
	jwtDuration time.Duration
}

func NewAuthService(userRepo repository.UserRepository, jwtSecret string, jwtDurationMinutes int) *AuthService {
	return &AuthService{
		userRepo:    userRepo,
		jwtSecret:   jwtSecret,
		jwtDuration: time.Duration(jwtDurationMinutes) * time.Minute,
	}
}

func (s *AuthService) Register(username, password, phone string) (*models.User, error) {
	// 检查用户是否已存在
	_, err := s.userRepo.FindByUsername(username)
	if err == nil {
		return nil, ErrUserExists
	}

	// 如果是"未找到"错误，可以继续注册流程
	if err != nil && err != repository.ErrNotFound {
		return nil, err // 其他错误（如数据库连接问题）
	}

	// 哈希密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	// 创建用户对象
	user := &models.User{
		Username:     username,
		Phone:        phone,
		PasswordHash: string(hashedPassword),
	}

	// 保存到数据库
	if err := s.userRepo.Create(user); err != nil {
		if err == repository.ErrDuplicate {
			return nil, ErrUserExists
		}
		return nil, err
	}

	return user, nil
}

func (s *AuthService) Login(username, password string) (string, error) {
	// 查找用户
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		if err == repository.ErrNotFound {
			return "", ErrInvalidCredentials
		}
		return "", err
	}

	// 验证密码
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return "", ErrInvalidCredentials
	}

	// 生成 JWT Token
	token, err := GenerateToken(int64(user.ID), user.Username, []byte(s.jwtSecret), s.jwtDuration)
	if err != nil {
		return "", err
	}

	return token, nil
}
