// File: internal/service/auth_service.go
package service

import (
	"errors"
	"time"

	"gateway.example/go-gateway/internal/auth" // JWT 生成工具
	"gateway.example/go-gateway/internal/dao"  // 数据访问层
	"gateway.example/go-gateway/internal/models"

	"golang.org/x/crypto/bcrypt" // 用于密码加密和比对
)

// AuthService 封装了所有与认证相关的业务逻辑
type AuthService struct {
	userDAO     dao.UserDAO   // 依赖 UserDAO 来操作数据库
	jwtSecret   string        // JWT 签名密钥
	jwtDuration time.Duration // JWT 有效期
}

// NewAuthService 是 AuthService 的构造函数
// 它接收所有必要的依赖，并通过这种方式实现“依赖注入”
func NewAuthService(userDAO dao.UserDAO, jwtSecret string, jwtDurationMinutes int) *AuthService {
	return &AuthService{
		userDAO:     userDAO,
		jwtSecret:   jwtSecret,
		jwtDuration: time.Duration(jwtDurationMinutes) * time.Minute,
	}
}

// Register 是处理用户注册的业务逻辑
func (s *AuthService) Register(username, password string) (*models.User, error) {
	// 1. 业务规则：检查用户是否已存在
	_, err := s.userDAO.FindByUsername(username)
	if err == nil {
		// 如果 err 是 nil，说明找到了用户，表示用户已存在
		return nil, errors.New("user already exists")
	}

	// 2. 业务逻辑：对密码进行哈希处理 (绝不能明文存储密码！)
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err // 如果哈希失败，是服务器内部错误
	}

	// 3. 准备要存入数据库的用户模型
	user := &models.User{
		Username:     username,
		PasswordHash: string(hashedPassword),
	}

	// 4. 调用 DAO 将用户数据持久化到数据库
	if err := s.userDAO.Create(user); err != nil {
		return nil, err // 如果创建失败，返回数据库错误
	}

	// 5. 返回创建成功的用户信息 (不包含密码)
	return user, nil
}

// Login 是处理用户登录的业务逻辑
func (s *AuthService) Login(username, password string) (string, error) {
	// 1. 调用 DAO 根据用户名查找用户
	user, err := s.userDAO.FindByUsername(username)
	if err != nil {
		// 不管是用户不存在还是其他数据库错误，都返回统一的错误信息，防止恶意探测
		return "", errors.New("invalid username or password")
	}

	// 2. 核心业务：比对用户输入的密码和数据库中存储的哈希
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		// 如果比对失败 (密码不匹配)，返回同样的统一错误信息
		return "", errors.New("invalid username or password")
	}

	// 3. 密码验证通过，生成 JWT Token
	token, err := auth.GenerateToken(int64(user.ID), user.Username, []byte(s.jwtSecret), s.jwtDuration)
	if err != nil {
		// Token 生成失败是服务器内部问题
		return "", err
	}

	// 4. 返回生成的 Token
	return token, nil
}
